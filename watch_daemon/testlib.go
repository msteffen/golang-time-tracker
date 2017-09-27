package watchd

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"path"
	"testing"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"github.com/msteffen/golang-time-tracker/client"
	"github.com/msteffen/golang-time-tracker/pkg/check"
)

// ReadBody is a helper function that reads resp.Body into a buffer and returns
// it as a string
func ReadBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	buf := &bytes.Buffer{}
	check.T(t, check.Nil(buf.ReadFrom(resp.Body)))
	return buf.String()
}

// TestServer identifies an in-process watch daemon that uses a testing clock to
// time ticks/watch writes, rather than the system clock
type TestServer struct {
	*testing.T
	*client.Client
	*TestingClock

	socketFile  string
	dbFile      string
	maxEventGap int64

	httpServer *http.Server
}

// StartTestServer brings up an in-process watch daemon, for the tests to talk
// to
func StartTestServer(t *testing.T, tmpDir string) *TestServer {
	// even though 'tmpDir' is a tempdir, it's shared by all tests currently
	// running. If we don't create a new temporary file for each invocation of
	// StartTestServer, then go test ... -count=N won't work, as all runs of a
	// given test will share the same DB and socket
	u := uuid.New()
	uuidStr := base64.RawURLEncoding.EncodeToString(u[:])
	socketFile := path.Join(tmpDir, fmt.Sprintf("%s.%s.sock", t.Name(), uuidStr))
	t.Logf("socketFile: %s\n", socketFile)
	dbFile := path.Join(tmpDir, fmt.Sprintf("%s.%s.db", t.Name(), uuidStr))
	t.Logf("dbFile: %s", dbFile)

	// Start server
	// Note: this used to use SQLite's ":memory:" built-in in-memory target, but
	// it caused races between tests. Now, tmpDir should always be in /dev/shm,
	// ubuntu's built-in ramfs mount, so this achieves the same thing (no
	// persisted data) but with different tests storing the DB at different paths,
	// and thus avoiding races.
	testClock := &TestingClock{}
	ttAPI, err := NewServer(testClock, dbFile)
	if err != nil {
		t.Fatalf("could not create API Server: %v", err)
	}
	l, err := NewSocketListener(socketFile)
	if err != nil {
		log.Fatalf("couldn't create test server listener: %v", err)
	}
	httpServer, err := ToHTTPServer(socketFile, testClock, ttAPI)
	if err != nil {
		log.Fatalf("couldn't create test server HTTP wrapper: %v", err)
	}
	// Start serving requests
	testServer := &TestServer{
		T:            t,
		Client:       client.GetClient(socketFile),
		TestingClock: testClock,
		socketFile:   socketFile,
		dbFile:       dbFile,
		maxEventGap:  ttAPI.(*server).maxEventGap,
		httpServer:   httpServer,
	}
	go httpServer.Serve(l)

	// Wait until the server is up before proceeding
	testServer.blockUntilActive()
	return testServer
}

// blockUntilActive is a helper for s.Restart() and StartTestServer()
func (s *TestServer) blockUntilActive() {
	secs := 60
	for i := 0; i < secs; i++ {
		log.Infof("waiting until server is up to continue (%d/%d)", i, secs)
		_, err := s.Client.Status()
		if err == nil {
			return // success
		}
		time.Sleep(time.Second)
	}
	log.Fatalf("test server didn't start after %d seconds", secs)
}

// TickAt is a helper function that sends ticks to the local TimeTracker server
// at the given intervals with the given labels
//
// (TickAt("l1", 1, 1, 1) would send a tick with the label "l1" at 1 minute past
// start, 2 minutes past start, and 3 minutes past start, logically)
func (s *TestServer) TickAt(label string, intervals ...int64) {
	for _, i := range intervals {
		s.TestingClock.Add(time.Duration(i * int64(time.Minute)))
		check.T(s.T, check.Nil(s.Client.Tick(label)))
	}
}

// Restart stops the TimeTrackerAPI server owned by 's', and then starts a new
// one. This is useful for tests that validate the time tracker's startup
// behavior (particularly with respect to watches)
func (s *TestServer) Restart() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ttAPI, err := NewServer(s.TestingClock, s.dbFile)
	if err != nil {
		log.Fatalf("could not create API Server: %v", err)
	}
	if err := s.httpServer.Shutdown(ctx); err != nil {
		log.Fatalf("couldn't shut down old test server: %v", err)
	}
	l, err := NewSocketListener(s.socketFile)
	if err != nil {
		log.Fatalf("couldn't create test server listener: %v", err)
	}
	httpServer, err := ToHTTPServer(s.socketFile, s.TestingClock, ttAPI)
	if err != nil {
		log.Fatalf("couldn't create test server HTTP wrapper: %v", err)
	}
	s.httpServer = httpServer
	// Start serving requests
	go httpServer.Serve(l)
	s.blockUntilActive()
}
