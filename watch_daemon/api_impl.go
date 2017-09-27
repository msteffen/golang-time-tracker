package watchd

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	_ "github.com/mattn/go-sqlite3"
	log "github.com/sirupsen/logrus"

	"github.com/msteffen/golang-time-tracker/client"
	"github.com/msteffen/golang-time-tracker/pkg/escape"
	"github.com/msteffen/golang-time-tracker/pkg/watcher"
)

// s_* contains the number of seconds in the given time duration, for compactly
// specifying unix times (seconds since unix epoch) and durations (seconds)
const (
	s_Second = 1
	s_Minute = 60 * s_Second
	s_Hour   = 60 * s_Minute
	s_Day    = 24 * s_Hour
)

// maxTime is the only time t such that for all go time.Time values t',
// t.After(t') == true
var /* const */ maxTime = time.Unix(1<<63-62135596801, 999999999)

// maxWatches is the maximum number of watches that may exist concurrently
const maxWatches = 4
const tickSyncFrequency = 3 * time.Second
const watchSyncFrequency = 3 * time.Second

type watch struct {
	//// Not Owned
	// [pointer to owner] the server that owns this watch
	server *server

	//// Owned
	// dir is the directory being watched
	dir string

	// label recorded in the database with every watch event identified with this
	// watch
	label string

	// hasPending is > 0 if watch has recieved a write that hasn't been recorded
	// in the database
	hasPending int32

	// ctx is the context attached to this watch (cancel, below, cancels this ctx)
	ctx context.Context

	// cancel causes the corresponding call to Watch() to exit (and cancels 'ctx'
	// above)
	cancel func()

	// when the watch started
	start time.Time
}

type dbWatchInfo struct {
	// dir is the directory being watched
	dir string

	// label recorded in the database with every file event underneath 'dir'
	label string

	// lastWrite indicates the most recent write recieved for this watch
	lastWrite time.Time
}

// recordWritesInDB runs every 5 seconds, and if any writes have been recorded
// since the last run, it writes a corresponding record into the DB
func (w *watch) recordWritesInDB() {
	// tick at most once every 'tickSyncFrequency'seconds (that 'notified' is set)
	// so that a flood of events (e.g. moving a large dir) doesn't create a flood
	// of persisted data.
	// TODO(msteffen) make this adjustable
	ticker := time.Tick(tickSyncFrequency)
	for {
		time.Sleep(tickSyncFrequency)

		// check if w.ctx has been cancelled (e.g. because another watch was added,
		// exceeding 'maxWatches')
		select {
		case <-w.ctx.Done():
			return // ctx has been cancelled
		case <-ticker:
		}

		// Check if a write has been received since the last iteration
		if atomic.CompareAndSwapInt32(&w.hasPending, 1, 0) {
			// don't start logging changes until after the watch has been established, to
			// avoid logging the flood of events corresponding to pre-existing files
			if time.Now().Sub(w.start).Seconds() > 10 {
				log.Infof("watch on [%s] (labeled [%s]) registered a change", w.dir, w.label)
			}

			// can't 'defer dbMu.Unlock()' b/c we're in a loop
			// w.server.dbMu.Unlock() is at the bottom of 'if ...(w.hasPending)' below
			w.server.dbMu.Lock()
			curTime := w.server.clock.Now()

			// update DB with new write
			// - watches are part of the API package basically because of this write
			// here; this way, the api package is the only place that writes to the
			// DB. These two changes are bundled into a transaction because even
			// though the watch holds a lock on the DB, the transaction avoids issues
			// where one write succeeds but the other doesn't
			success := true
			txn, err := w.server.db.Begin()
			if err != nil {
				log.Errorf("error creating txn to record write at %v for %q (will attempt to roll back): %v",
					curTime, w.dir, err)
			}
			for _, cmd := range []string{
				// On startup, time-tracker will scan all old watches and recreate them,
				// which creates a flurry of write events. The 'watch'es for all of
				// these then try to record a write in the DB simultaneously, and all
				// after the first get an error from SQLite that they've violated the
				// UNIQUE constraint on the 'ticks' table.
				//
				// Rather than trying to separate the write events from all of the
				// watches to avoid violating the UNIQUE constaint, just use INSERT OR
				// IGNORE here and ignore all writes after the first.
				fmt.Sprintf(`INSERT OR IGNORE INTO ticks (time, labels) VALUES (%d, %q);`,
					curTime.Unix(), escape.Escape(w.label)),
				fmt.Sprintf(`UPDATE watches SET last_write = %d WHERE dir = %q;`,
					curTime.Unix(), escape.Escape(w.dir)),
			} {
				if _, err := txn.Exec(cmd); err != nil {
					log.Errorf("error recording write at %v for %q (will attempt to roll back): %v",
						curTime, w.dir, err)
					success = false
					break
				}
			}
			if !success {
				if err := txn.Rollback(); err != nil {
					log.Errorf("error rolling back txn: %v", err)
				}
			} else if err := txn.Commit(); err != nil {
				log.Errorf("error committing: %v", err)
			}
			w.server.dbMu.Unlock()
		}
	}
}

// handleEvent receives WatchEvents from a watcher, and signals to
// 'recordWritesInDB' that a tick needs to be recorded
func (w *watch) handleEvent(e watcher.WatchEvent) error {
	// don't start logging changes until after the watch has been established, to
	// avoid logging the flood of events corresponding to pre-existing files
	if time.Now().Sub(w.start).Seconds() > 10 {
		log.Infof("observed %s", e)
	}
	atomic.StoreInt32(&w.hasPending, 1)
	return nil
}

// server implements the client.TimeTrackerAPI interface
type server struct {
	//// Not owned
	clock Clock

	//// Owned
	// maxEventGap is a configuration option that determines how far apart ticks
	// may be before they're considered part of different intervals
	maxEventGap int64

	// db is (a client of) a SQLite database that contains all ticks and watches
	// persisted on disk
	db *sql.DB

	// dbMu guards 'db'. The sqlite driver does not allow for concurrent writes.
	// See https://github.com/mattn/go-sqlite3#faq
	// This allows for safe concurrent use of 'db'
	dbMu sync.RWMutex

	// watches maps paths currently watched by 'server' to a cancel() fn that
	// kills the inotify watch in the kernel
	watches map[string]*watch

	// watchMu guards 'watches'. Note that if you're planning to lock both dbMu
	// and watchMu, you must lock dbMu first (currently only syncWatches locks
	// both, and locks them in that order)
	watchMu sync.Mutex
}

// NewServer returns an implementation of client.TimeTrackerAPI
func NewServer(clock Clock, dbPath string) (client.TimeTrackerAPI, error) {
	// Create DB connection
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	err = db.Ping()
	for err != nil {
		log.Print("Waiting for DB to start")
		time.Sleep(time.Second)
		err = db.Ping()
	}
	// Take advantage of sqlite INTEGER PRIMARY KEY table for fast range scan of
	// ticks and watches: https://sqlite.org/lang_createtable.html#rowid
	// Also, create tables in txn b/c we need both of them
	if _, err = db.Exec(`
		BEGIN TRANSACTION;
	  CREATE TABLE IF NOT EXISTS ticks (time INTEGER PRIMARY KEY ASC, labels TEXT);
	  CREATE TABLE IF NOT EXISTS watches (last_write INTEGER, dir TEXT, label TEXT);
		COMMIT;
	`); err != nil {
		return nil, fmt.Errorf("could not create SQL tables: %v", err)
	}

	// Create new server struct
	s := &server{
		watches:     make(map[string]*watch),
		db:          db,
		clock:       clock,
		maxEventGap: 23 * s_Minute,
	}
	go s.syncWatchesLoop()
	return s, nil
}

// Tick handles the /tick http endpoint. Note that watch events don't go through
// this endpoint because watch events should also update the last_write field of
// the relevant watch (which happens in a transaction with the new tick)
func (s *server) Tick(req *client.TickRequest) error {
	// Write tick to DB
	s.dbMu.Lock()
	defer s.dbMu.Unlock()

	_, err := s.db.Exec(fmt.Sprintf(
		"INSERT INTO ticks (time, labels) VALUES (%d, %q)",
		s.clock.Now().Unix(), escape.Escape(req.Label),
	))
	return err
}

// addWatchToDB is a helper for Watch(), which essentially wraps the part of a
// Watch() operation that must be done while s.dbMu is held
//
// s.syncWatches, which happens at the end of Watch(), must be done outside this
// function, which is why it's separate from the main RPC handler
func (s *server) addWatchToDB(dir, label string) error {
	// Note: the dbMu prevents a concurrent write elsewhere from adding a
	// redundant watch for req.Dir between this existence check and the write
	// below.
	//
	// I keep wanting to use a transaction to manage this R+M+W operation, but
	// SQLite transactions, AFAICT, aren't useful for preventing R+M+W races (in
	// this case, that would be two Watch() RPCs adding the same directory at the
	// same time and both succeeding. I think that could happen even with MVCC
	// consistency that SQLite txns provide).
	//
	// I think SQLite txns are for preventing multiple writes that must all
	// succeed to maintain consistency from succeeding or failing independently
	// (so, above, where we register a tick and update a watch's last write time,
	// is a sensible use of a transaction, as we need both writes to succeed or
	// fail for consistency between the tables).
	s.dbMu.Lock()
	defer s.dbMu.Unlock()
	dbWatches, err := s.getDBWatches()
	if err != nil {
		return fmt.Errorf("error checking if watch on %q already exists: %v", dir, err)
	}
	for _, wi := range dbWatches {
		if strings.HasPrefix(dir, wi.dir) {
			return &WatchExistsErr{dir: wi.dir}
		}
		if strings.HasPrefix(wi.dir, dir) {
			// TODO(msteffen): instead of erroring, we should remove the old watch and
			// add the new, higher-level watch
			return &WatchExistsErr{dir: wi.dir}
		}
	}

	// Insert new watch into watches (syncWatchLoop() will eventually pick it up)
	_, err = s.db.Exec(fmt.Sprintf(
		"INSERT INTO watches (last_write, dir, label) VALUES (%d, %q, %q);",
		s.clock.Now().Unix(), escape.Escape(dir), escape.Escape(label)))
	if err != nil {
		return fmt.Errorf("error creating new watch in DB: %v", err)
	}
	return nil
}

// Watch handles the /watch http endpoint
func (s *server) Watch(req *client.WatchRequest) error {
	if err := s.addWatchToDB(req.Dir, req.Label); err != nil {
		return err
	}
	if err := s.syncWatches(); err != nil {
		return err
	}
	return nil
}

// getDBWatches factors out code to read the set of existing watches from the
// DB, ordered by the watched dir.
//
// Note: dbMu must be held by the caller
func (s *server) getDBWatches() ([]*dbWatchInfo, error) {
	rows, err := s.db.Query(fmt.Sprintf("SELECT * FROM watches ORDER BY dir ASC"))
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("could not read existing watches: %v", err)
	}
	dbWatches := make([]*dbWatchInfo, 0, maxWatches)
	for rows.Next() {
		// parse SQL record
		var lastWrite int64
		var escapedDir, escapedLabel string
		if err := rows.Scan(&lastWrite, &escapedDir, &escapedLabel); err != nil {
			return nil, fmt.Errorf("error scanning watch rows: %v", err)
		}
		dbWatches = append(dbWatches, &dbWatchInfo{
			lastWrite: time.Unix(lastWrite, 0),
			dir:       escape.Unescape(escapedDir),
			label:     escape.Unescape(escapedLabel),
		})
	}
	// watches are already sorted in ascending order of last write by SQLite
	return dbWatches, nil
}

func (s *server) syncWatchesLoop() {
	errCount := 0
	for {
		err := s.syncWatches()
		if err != nil {
			errCount++
		}
		if errCount >= 3 {
			panic("giving up syncing watches: too many errors")
		}
		time.Sleep(watchSyncFrequency)
	}
}

type op byte

const (
	create op = iota
	remove
)

// syncWatches reads the current set of watched directories in s.db (the source
// of truth), and updates s.watches and the inotify watches that have been set
// up in the kernel to align with s.db.
func (s *server) syncWatches() error {
	s.dbMu.Lock()
	// (1) delete any watches in excess of the maximum number of watches (by
	// selecting the first |actualWatches - maxWatches| watches that were written
	// furthest in the past)
	_, err := s.db.Exec(fmt.Sprintf(`
	  DELETE FROM watches WHERE dir IN (
	    SELECT dir FROM watches
	    ORDER BY last_write ASC
	    LIMIT MAX(0, (SELECT COUNT(*) FROM watches) - %d)
	  );
	`, maxWatches))

	// (2) Align set of active watches with watch processes
	// (2.1) get target set of watches from DB
	dbWatches, err := s.getDBWatches()
	if err != nil {
		return fmt.Errorf("error syncing watches: %v", err)
	}
	s.dbMu.Unlock()

	s.watchMu.Lock()
	defer s.watchMu.Unlock()
	// (2.2) read existing watches into slice, sorted by name (matches sort order
	// from getDBWatches)
	existingWatches := make([]string, 0, len(s.watches))
	for d := range s.watches {
		existingWatches = append(existingWatches, d)
	}
	sort.Strings(existingWatches)

	// compare watches from DB with existing watch goros & fix diffs
	var i, j, i2, j2 int
	for i2 < len(existingWatches) || j2 < len(dbWatches) {
		i, j = i2, j2 // increment i, j from previous loop
		var o op
		switch {
		case i >= len(existingWatches):
			o = create
			j2++
		case j >= len(dbWatches):
			o = remove
			i2++
		case existingWatches[i] > dbWatches[j].dir:
			o = create
			j2++
		case existingWatches[i] < dbWatches[j].dir:
			o = remove
			i2++
		case existingWatches[i] == dbWatches[j].dir:
			i2++
			j2++
			continue // nothing to do -- watch should & does exist
		}

		switch o {
		case create:
			log.Infof("syncWatches is setting up watch on [%s]", dbWatches[j].dir)
			ctx, cancel := context.WithCancel(context.Background())
			w := &watch{
				dir:    dbWatches[j].dir,
				server: s,
				label:  dbWatches[j].label,
				ctx:    ctx,
				cancel: cancel,
				start:  time.Now(),
			}
			s.watches[dbWatches[j].dir] = w
			go w.recordWritesInDB() // start goro that makes ticks in the DB
			go func(dir string) {   // start watching for writes to shouldExist
				if err := watcher.Watch(ctx, dir, w.handleEvent); err != nil {
					log.Warningf("watch on [%s] failed: %v", dir, err)
				}
			}(dbWatches[j].dir) // j will increment--pass dir to fix value inside goro
		case remove:
			log.Infof("syncWatches is removing watch on [%s]", existingWatches[i])
			// kill existing watch -- doesn't exist in DB
			s.watches[existingWatches[i]].cancel()
			delete(s.watches, existingWatches[i])
			n := copy(existingWatches[i:], existingWatches[i+1:])
			existingWatches = existingWatches[:n]
		}
	}
	return nil
}

// GetWatches implements the corresponding method of the client.TimeTrackerAPI
// interface
func (s *server) GetWatches(req *client.GetWatchesRequest) (*client.GetWatchesResponse, error) {
	s.watchMu.Lock()
	defer s.watchMu.Unlock()

	response := &client.GetWatchesResponse{
		Watches: make([]*client.WatchInfo, 0, len(s.watches)),
	}
	for dir, w := range s.watches {
		response.Watches = append(response.Watches, &client.WatchInfo{
			Dir:   dir,
			Label: w.label,
		})
	}
	return response, nil
}

func (s *server) GetIntervals(req *client.GetIntervalsRequest) (*client.GetIntervalsResponse, error) {
	// Get list of times in the 'req' range from DB
	var rows *sql.Rows
	var err error
	func() {
		s.dbMu.RLock()
		defer s.dbMu.RUnlock()
		// check maxEventGap before and after request, to handle the case where a time
		// interval overlaps with the request interval
		start := req.Start - s.maxEventGap
		end := req.End + s.maxEventGap
		rows, err = s.db.Query(fmt.Sprintf(
			"SELECT * FROM ticks WHERE time BETWEEN %d AND %d", start, end,
		))
	}()
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	// Iterate through 'times' and break it up into intervals
	collector := make(map[string]*Collector) // map label to collector
	collector[""] = NewCollector(req.Start, req.End, s.maxEventGap, s.clock.Now().Unix())
	var (
		prevLabel string // label that no tick will have initially
		prevT     int64  // prev tick's time (unix seconds)
	)
	for rows.Next() {
		// parse SQL record
		var escapedLabel string
		var t int64
		rows.Scan(&t, &escapedLabel)
		if err := rows.Scan(&t, &escapedLabel); err != nil {
			return nil, fmt.Errorf("error scanning tick row: %v", err)
		}
		label := escape.Unescape(escapedLabel)

		// initialize collector for current activity
		if collector[label] == nil {
			collector[label] = NewCollector(req.Start, req.End, s.maxEventGap, s.clock.Now().Unix())
			collector[label].label = label
		}

		if prevLabel != label {
			// New activity was started--this activity's interval starts at the end
			// of the previous activity's interval (if there is one)
			if prevT > 0 {
				collector[label].Add(prevT)
			}
			prevLabel = label
		}

		// Add timestamp to collectors
		prevT = t
		collector[label].Add(t)
		collector[""].Add(t)
	}

	// If we could extend the leftmost interval, proactively extend it and
	// indicate how much time has elapsed since the past tick to the caller
	now := s.clock.Now().Unix()
	endGap := int64(0)
	if (now - prevT) < s.maxEventGap {
		collector[prevLabel].Add(now)
		collector[""].Add(now)
		endGap = now - prevT
	}

	// TODO include labelled intervals in response
	return &client.GetIntervalsResponse{
		Intervals: collector[""].Finish(),
		EndGap:    endGap,
	}, nil
}

func (s *server) Clear() error {
	s.dbMu.Lock()
	defer s.dbMu.Unlock()
	if _, err := s.db.Exec(`
	  DROP TABLE ticks;
	  DROP TABLE watches;
	  CREATE TABLE IF NOT EXISTS ticks (time INTEGER PRIMARY KEY ASC, labels TEXT);
	  CREATE TABLE IF NOT EXISTS watches (last_write INTEGER PRIMARY KEY ASC, dir TEXT, label TEXT);
	`); err != nil {
		return err
	}
	return nil
}
