package watchd

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"path"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/msteffen/golang-time-tracker/client"
	"github.com/msteffen/golang-time-tracker/pkg/check"
)

var (
	// address is the address on which the test server serves. It's different
	// from the default host port used by t serve (i.e. the default value of the
	// --address flag defined in cli/t/main.go) so that I can run tests while a
	// real instance of the time tracker is running
	address = "localhost:9090"

	// defaultDBDir is the default location of the DB file used by the test
	// server. /dev/shm is a pre-mounted in-memory filesystem that exists by
	// default on most linux distros (incl. ubuntu on my laptop)
	dbDir = "/dev/shm/time-tracker-test"

	// lastHTTPServer stores the most recent httpServer returned by
	// StartTestServer. It will be shut down by any subsequent call to
	// StartTestServer() (to guarantee that only one goro is listening on
	// 'address' at a time
	lastHTTPServer *http.Server
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
func StartTestServer(t *testing.T) *TestServer {
	// 'dbDir' is shared by all tests currently running. If we don't remove the
	// existing tmp directory for invocation of StartTestServer, then go test ...
	// -count=N won't work, as all runs of a given test will share the same DB
	if err := os.RemoveAll(dbDir); err != nil {
		t.Fatalf("couldn't remove existing dir %q: %v", dbDir, err)
	}
	if err := os.Mkdir(dbDir, 0700); err != nil {
		t.Fatalf("couldn't create dir %q: %v", dbDir, err)
	}
	dbFile := path.Join(dbDir, t.Name())
	t.Logf("dbFile: %s", dbFile)

	// Create request handling struct
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
	maxEventGap := ttAPI.(*server).maxEventGap

	// Start listening for HTTP requests
	testServer := &TestServer{
		T:            t,
		Client:       &client.Client{Address: address},
		TestingClock: testClock,
		dbFile:       dbFile,
		maxEventGap:  maxEventGap,
		httpServer:   ToHTTPServer(address, testClock, ttAPI),
	}
	testServer.StartServing(t)
	return testServer
}

func (ts *TestServer) StartServing(t *testing.T) {
	// Shut down any prior HTTP server
	if lastHTTPServer != nil {
		if err := lastHTTPServer.Shutdown(context.Background()); err != nil {
			t.Fatalf("couldn't shut down existing server: %v", err)
		}
	}
	lastHTTPServer = ts.httpServer
	go func() {
		err := ts.httpServer.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			// panic vs t.Fatalf() as this may run after the test has finished
			panic(fmt.Sprintf("error from ts.ListenAndServe(): %v", err))
		}
	}()

	// Wait until the server is up before proceeding
	secs := 60
	for i := 0; i < secs; i++ {
		log.Infof("waiting until server is up to continue (%d/%d)", i, secs)
		_, err := ts.Client.Status()
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
func (s *TestServer) Restart(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ttAPI, err := NewServer(s.TestingClock, s.dbFile)
	if err != nil {
		log.Fatalf("could not create API Server: %v", err)
	}
	if err := s.httpServer.Shutdown(ctx); err != nil {
		log.Fatalf("couldn't shut down old test server: %v", err)
	}
	s.httpServer = ToHTTPServer(address, s.TestingClock, ttAPI)
	// Start serving requests
	s.StartServing(t)
}
