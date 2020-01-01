package watchd

import (
	"encoding/base64"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/msteffen/golang-time-tracker/client"
	"github.com/msteffen/golang-time-tracker/pkg/check"

	"github.com/google/uuid"
)

const testDir = "/dev/shm/time-tracker-test"

const tEpsilon = 500 * time.Millisecond

// TestParsing does a basic test of the TimeTracker API (registering 4 ticks
// that create two intervals
func TestParsing(t *testing.T) {
	s := StartTestServer(t)
	t.Log("Test server started")
	ts := time.Date(
		/* date */ 2017, 7, 1,
		/* time */ 12, 0, 0,
		/* nsec, location */ 0, time.Local)
	s.Set(ts)

	// Make several calls to /tick via the HTTP API (simulating that they arrive
	// several minutes apart, so that there are two distinct intervals here).
	// Don't use TickAt, to test json parsing.
	for _, i := range []int64{0, 1, 1, 30, 1} {
		s.Add(time.Duration(i * int64(time.Minute)))
		check.T(t, check.Nil(s.Tick("work")))
	}

	// Make a call to /intervals and make sure the two expected intervals
	// are returned
	morning := time.Date(2017, 7, 1, 0, 0, 0, 0, time.Local)
	night := morning.Add(24 * time.Hour)
	actual, err := s.GetIntervals(morning, night)
	check.T(t,
		check.Nil(err),
		check.Eq(actual, &client.GetIntervalsResponse{
			Intervals: []client.Interval{
				{
					Start: ts.Unix(),
					End:   ts.Add(2 * time.Minute).Unix(),
				},
				{
					Start: ts.Add(32 * time.Minute).Unix(),
					End:   ts.Add(33 * time.Minute).Unix(),
				},
			},
		}))
}

// TestGetIntervalsBoundary checks that GetIntervals only returns intervals
// within the given time range
func TestGetIntervalsBoundary(t *testing.T) {
	s := StartTestServer(t)
	ts := time.Date(
		/* date */ 2017, 7, 1,
		/* time */ 6, 0, 0,
		/* nsec, location */ 0, time.Local)
	s.Set(ts)

	// tick every 20 minutes for 12 hours, so we have a single interval from 6am
	// to 6pm
	hours, ticksPerHour := 12, 3
	s.TickAt("work", 0)
	for i := 0; i < (hours * ticksPerHour); i++ {
		s.TickAt("work", 20)
	}

	// Enumerate test cases
	name := []string{
		"day-before",
		"overlap-morning",
		"full-day",
		"overlap-evening",
		"day-after",
	}
	reqStartTs := []time.Time{
		time.Date(2017, 6, 30, 0, 0, 0, 0, time.Local),  // day before
		time.Date(2017, 6, 30, 12, 0, 0, 0, time.Local), // overlap morning
		time.Date(2017, 7, 1, 0, 0, 0, 0, time.Local),   // full day
		time.Date(2017, 7, 1, 12, 0, 0, 0, time.Local),  // overlap evening
		time.Date(2017, 7, 2, 0, 0, 0, 0, time.Local),   // day after
	}
	expected := [][]client.Interval{
		// no overlap
		{},
		// end at noon (req end)
		{{Start: ts.Unix(), End: ts.Add(6 * time.Hour).Unix()}},
		// full interval
		{{Start: ts.Unix(), End: ts.Add(12 * time.Hour).Unix()}},
		// begin at noon (req start)
		{{Start: ts.Add(6 * time.Hour).Unix(), End: ts.Add(12 * time.Hour).Unix()}},
		// no overlap
		{},
	}

	// Make a call to /intervals and make sure the two expected intervals
	// are returned
	for i := 0; i < len(name); i++ {
		t.Run(name[i], func(t *testing.T) {
			reqStart, reqEnd := reqStartTs[i], reqStartTs[i].Add(24*time.Hour)
			actual, err := s.GetIntervals(reqStart, reqEnd)
			check.T(t,
				check.Nil(err),
				check.Eq(actual, &client.GetIntervalsResponse{Intervals: expected[i]}))
		})
	}
}

// a persistent, incrementing counter used by getFileNumber
var fileNumber int

// getFileNumber is used by testWatchBasic to create unique file names across
// tests
func getFileNumber() int {
	fileNumber++
	return fileNumber - 1
}

func installWatch(s *TestServer, dir string) {
	// make sure that if a tick was just recorded, setting up the watch won't
	// cause uniqueness errors in SQLite.
	s.Add(time.Second)
	// likewise, recording a tick after setting up the watch won't cause a
	// uniqueness error
	defer s.Add(time.Second)

	// create watch on "dir"
	err := s.Watch(dir, "test")
	check.T(s.T, check.Nil(err))
}

func testWritesCreateWorkInterval(s *TestServer, dir string) {
	s.T.Helper()
	eventGap := (time.Duration(s.maxEventGap) - 1) * time.Second
	// separate the work interval we're about to create from any previous work
	// interval. This allows us to confirm that 'dir' is being watched by querying
	// /intervals and only inspecting the last interval
	s.Add(time.Duration(s.maxEventGap+1) * time.Second)

	// write to test dir (should create a work interval starting at 'ts')
	ts := s.TestingClock.Now()
	for tick := 0; tick < 3; tick++ {
		newFilePath := path.Join(dir, fmt.Sprintf("file-%d", getFileNumber()))
		f, err := os.OpenFile(newFilePath, os.O_CREATE|os.O_RDWR, 0644)
		check.T(s.T,
			check.Nil(err),
			check.Nil(f.Close()))
		time.Sleep(tickSyncFrequency + tEpsilon) // wait for write-batching watcher
		s.Add(eventGap)
	}

	// query /intervals & check that the last interval wraps the writes
	morning := time.Date(
		/* date */ ts.Year(), ts.Month(), ts.Day(),
		/* time */ 0, 0, 0,
		/* nsec, location */ 0, time.Local)
	actual, err := s.GetIntervals(morning, morning.Add(24*time.Hour))
	check.T(s.T,
		check.Nil(err),
		check.True(len(actual.Intervals) > 0))
	last := len(actual.Intervals) - 1
	s.T.Logf("checking that watch is active for %q", dir)
	check.T(s.T,
		check.Eq(actual.Intervals[last], client.Interval{
			Start: ts.Unix(),
			// expect 3*eventGap (vs 2x) b/c the /intervals api proactively
			// extends the rightmost interval to the current time
			// TODO Get rid of EndGap!!
			End: ts.Add(3 * eventGap).Unix(),
		}),
		check.Eq(actual.EndGap, int64(eventGap/time.Second)),
	)
}

// testWatchBasic contains the core logic of TestWatchBasic. It's factored into
// a helper because both TestWatchBasic and TestMaxWatches use this code
func testWatchBasic(s *TestServer, dir string) {
	s.T.Helper()
	installWatch(s, dir)
	testWritesCreateWorkInterval(s, dir)
}

func randomSuffix(s string) string {
	u := uuid.New()
	return fmt.Sprintf("%s-%s", s, base64.RawURLEncoding.EncodeToString(u[:]))
}

func TestWatchBasic(t *testing.T) {
	s := StartTestServer(t)
	ts := time.Date(
		/* date */ 2017, 7, 1,
		/* time */ 6, 0, 0,
		/* nsec, location */ 0, time.Local)
	s.Set(ts)

	// create "dir"
	dir := path.Join(testDir, randomSuffix(t.Name()))
	check.T(t, check.Nil(os.Mkdir(dir, 0755)))
	// test watch on 'dir'
	testWatchBasic(s, dir)
}

func TestMaxWatches(t *testing.T) {
	s := StartTestServer(t)
	dirPrefix := randomSuffix(t.Name())
	ts := time.Date(
		/* date */ 2017, 7, 1,
		/* time */ 6, 0, 0,
		/* nsec, location */ 0, time.Local)
	s.Set(ts)

	dirsToWatch := maxWatches + 1
	for i := 0; i < dirsToWatch; i++ {
		dir := path.Join(testDir, fmt.Sprintf("%s-%d", dirPrefix, i))
		// create "dir"
		check.T(t, check.Nil(os.Mkdir(dir, 0755)))
		// test watch on 'dir'
		t.Logf("testing watch on %q", dir)
		testWatchBasic(s, dir)
	}

	// query the set of watched dirs, and make sure they don't include the initial
	// dir (as we've exceeded the max by one)
	watches, err := s.GetWatches()
	check.T(t, check.Nil(err))
	watchedDirs := make(map[string]struct{}) // hold query results
	for _, w := range watches.Watches {
		relPath, err := filepath.Rel(testDir, w.Dir)
		check.T(t, check.Nil(err))
		watchedDirs[relPath] = struct{}{}
	}
	for i := 0; i < dirsToWatch; i++ {
		_, watched := watchedDirs[fmt.Sprintf("%s-%d", dirPrefix, i)]
		if i == 0 {
			check.T(t, check.False(watched))
		} else {
			check.T(t, check.True(watched))
		}
	}

	// confirm that writing to dir #1 still works
	dir := path.Join(testDir, fmt.Sprintf("%s-1", dirPrefix))
	testWritesCreateWorkInterval(s, dir)

	// Try to create an interval in dir 0 and make sure nothing happens
	// (1) get existing intervals (basis for comparison)
	getToday := func() *client.GetIntervalsResponse {
		morning := time.Date(
			/* date */ ts.Year(), ts.Month(), ts.Day(),
			/* time */ 0, 0, 0,
			/* nsec, location */ 0, time.Local)
		resp, err := s.GetIntervals(morning, morning.Add(24*time.Hour))
		check.T(t,
			check.Nil(err),
			check.True(len(resp.Intervals) > 0))

		// undo proactive interval extension, to simplify comparison
		last := len(resp.Intervals) - 1
		resp.Intervals[last].End -= resp.EndGap
		resp.EndGap = 0
		return resp
	}
	expected := getToday()

	// (2) write to watched dir #0
	// copied from testWritesCreateWorkInterval (can't use that, though, b/c this
	// shouldn't create an interval)
	eventGap := (time.Duration(s.maxEventGap) - 1) * time.Second
	dir = path.Join(testDir, fmt.Sprintf("%s-0", dirPrefix))
	tsNew := s.TestingClock.Now()
	check.T(t, check.False(tsNew == ts))
	for tick := 0; tick < 3; tick++ {
		newFilePath := path.Join(dir, fmt.Sprintf("file-%d", getFileNumber()))
		f, err := os.OpenFile(newFilePath, os.O_CREATE|os.O_RDWR, 0644)
		check.T(t, check.Nil(err), check.Nil(f.Close()))
		time.Sleep(tickSyncFrequency + tEpsilon) // wait for write-batching watcher
		s.Add(eventGap)
	}

	// (3) Check the work intervals created, and make sure they match what we
	// expect from the writes (the same set of intervals from before the ticks)
	actual := getToday()
	check.T(t, check.Eq(actual, expected))
}

func TestWatchesPersistOnRestart(t *testing.T) {
	s := StartTestServer(t)
	dirPrefix := randomSuffix(t.Name())
	ts := time.Date(
		/* date */ 2017, 7, 1,
		/* time */ 6, 0, 0,
		/* nsec, location */ 0, time.Local)
	s.Set(ts)

	// Create test dirs
	dirsToWatch := 3
	if maxWatches < dirsToWatch {
		dirsToWatch = maxWatches
	}
	for i := 0; i < dirsToWatch; i++ {
		dir := path.Join(testDir, fmt.Sprintf("%s-%d", dirPrefix, i))
		// create "dir"
		check.T(t, check.Nil(os.Mkdir(dir, 0755)))
		// test watch on 'dir'
		testWatchBasic(s, dir)
	}

	// query the set of watched dirs, and make sure they're as expected
	watches, err := s.GetWatches()
	check.T(t, check.Nil(err))
	watchedDirs := make(map[string]struct{}) // hold query results
	for _, w := range watches.Watches {
		relPath, err := filepath.Rel(testDir, w.Dir)
		check.T(t, check.Nil(err))
		watchedDirs[relPath] = struct{}{}
	}
	check.T(t, check.Eq(len(watchedDirs), dirsToWatch))
	for i := 0; i < dirsToWatch; i++ {
		_, watched := watchedDirs[fmt.Sprintf("%s-%d", dirPrefix, i)]
		check.T(t, check.True(watched))
	}

	// restart test server
	s.Restart(t)
	// TODO replace this sleep with a retry loop around the test below
	time.Sleep(5 * time.Second)

	for i := 0; i < dirsToWatch; i++ {
		dir := path.Join(testDir, fmt.Sprintf("%s-%d", dirPrefix, i))
		// test watch on 'dir'
		testWritesCreateWorkInterval(s, dir)
	}
}
