package watchd

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"os"
	p "path"
	"strconv"
	"time"

	"github.com/msteffen/golang-time-tracker/client"
	log "github.com/sirupsen/logrus"
)

// httpServer implements HTTP API wrappers around the apiServer's methods. It's
// stateless (except for its start time and the apiServer it owns) but does all
// validation and parsing, so that any error returned by apiServer can be an
// internalServerError
type httpServer struct {
	// Not Owned
	clock Clock // not owned b/c time isn't modified, but is read by /today

	// Owned
	apiServer client.TimeTrackerAPI
	startTime time.Time
}

func (d *httpServer) watch(w http.ResponseWriter, r *http.Request) {
	// Unmarshal and validate request
	if r.Method != "POST" {
		http.Error(w, "must use POST to access /watch", http.StatusMethodNotAllowed)
		return
	}
	var req client.WatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		msg := fmt.Sprintf("request did not match expected type: %v", err)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	req.Dir = p.Clean(req.Dir)
	if !p.IsAbs(req.Dir) {
		msg := fmt.Sprintf("must provide absolute path to /watch: %q", req.Dir)
		http.Error(w, msg, http.StatusBadRequest)
	}
	if req.Label == "" {
		req.Label = p.Base(req.Dir)
	}

	// apply request
	var err error
	err = d.apiServer.Watch(&req)
	if err != nil {
		if err, ok := err.(*WatchExistsErr); ok {
			w.Write([]byte(err.Error())) // just indicate that this call was a no-op w/ no err
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (d *httpServer) tick(w http.ResponseWriter, r *http.Request) {
	// Unmarshal and validate request
	if r.Method != "POST" {
		http.Error(w, "must use POST to access /tick", http.StatusMethodNotAllowed)
		return
	}
	var req client.TickRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		msg := fmt.Sprintf("request did not match expected type: %v", err)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	if req.Label == "" {
		msg := "tick request must have a label (\"\" is used to " +
			"indicate intervals formed by the union of all ticks in GetIntervals"
		http.Error(w, msg, http.StatusBadRequest)
	}

	// Process request
	var err error
	err = d.apiServer.Tick(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (d *httpServer) getIntervals(w http.ResponseWriter, r *http.Request) {
	// Unmarshal and validate request
	if r.Method != "GET" {
		http.Error(w, "must use GET to access /intervals", http.StatusMethodNotAllowed)
		return
	}

	// Trasform GET params into request struct
	boundary := []int64{0, math.MaxInt32} // start and end
	var err error
	for i, param := range []string{"start", "end"} {
		if s := r.URL.Query().Get(param); s != "" {
			boundary[i], err = strconv.ParseInt(s, 10, 64)
			if err != nil {
				msg := fmt.Sprintf("invalid \"%s\" value: %s", param, err.Error())
				http.Error(w, msg, http.StatusBadRequest)
				return
			}
		}
	}
	req := client.GetIntervalsRequest{
		Start: boundary[0],
		End:   boundary[1],
	}

	// Process request
	var result *client.GetIntervalsResponse
	result, err = d.apiServer.GetIntervals(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		log.Errorf("could not serialize /intervals result: %v", err)
		http.Error(w, "could not serialize result: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(resultJSON)
}

func (d *httpServer) getWatches(w http.ResponseWriter, r *http.Request) {
	// Unmarshal and validate request
	if r.Method != "GET" {
		http.Error(w, "must use GET to access /watches", http.StatusMethodNotAllowed)
		return
	}

	// Process request
	var result *client.GetWatchesResponse
	var err error
	result, err = d.apiServer.GetWatches(&client.GetWatchesRequest{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		log.Errorf("could not serialize /watches result: %v", err)
		http.Error(w, "could not serialize result: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(resultJSON)
}

func (d *httpServer) clear(w http.ResponseWriter, r *http.Request) {
	// Unmarshal and validate request
	if r.Method != "POST" {
		http.Error(w, "must use POST to access /clear", http.StatusMethodNotAllowed)
		return
	}

	// Require a request body to prevent me from accidentally clearing my data
	// from my browser
	req := make(map[string]interface{})
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		msg := fmt.Sprintf("request did not match expected type: %v", err)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	if req["confirm"] != "yes" {
		http.Error(w, "Must send confirmation message to delete all server data", http.StatusBadRequest)
		return
	}

	// Process request
	var err error
	err = d.apiServer.Clear()
	if err != nil {
		http.Error(w, fmt.Sprintf("Could not clear DB: %v", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (d *httpServer) status(w http.ResponseWriter, r *http.Request) {
	// Unmarshal and validate request
	if r.Method != "GET" {
		http.Error(w, "must use GET to access /status", http.StatusMethodNotAllowed)
		return
	}
	log.Infof("/status: %v", time.Now().Sub(d.startTime).String())
	w.Write([]byte(time.Now().Sub(d.startTime).String()))
}

// NewSocketListener is a helper for ServerOverHTTP, StartTestServer,
// TestServer.Restart, and any other function that needs to start a
// TimeTrackerAPI server. It creates a new golang net.Listener that's attached
// to a socket at 'socketPath', and includes the ability to check for an
// existing server (useful outside of tests, where time tracker servers
// typically listen on $HOME/.time-tracker/sock) and delete the socket/retry if
// the current socket is unused.
func NewSocketListener(socketPath string) (net.Listener, error) {
	var listener net.Listener
	for retry := 0; retry < 3; retry++ {
		// Stat socket file
		info, err := os.Stat(socketPath)
		log.Infof("socket stat error (socket should not exist): %v", err)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("could not stat socket at %q: %v", socketPath, err)
			}

			// Happy path: socket doesn't exist. Try to create it with net.Listen()
			log.Infof("http server about to listen on %s", socketPath)
			listener, err = net.Listen("unix", socketPath)
			if err != nil {
				log.Warningf("could not listen on unix socket at %s (will retry): %v",
					socketPath, err)
				continue
			}
			return listener, nil // success
		} else {
			// Check if socket is unexpected file type. Don't remove it in case it
			// belongs to another application somehow
			if info.Mode()&os.ModeType != os.ModeSocket {
				return nil, fmt.Errorf("socket file had unexpected file type: %s (maybe "+
					"it's owned by another application?)", info.Mode())
			}

			// See if server is running by sending request
			_, err = client.GetClient(socketPath).Status()
			if err == nil {
				return nil, errors.New("watch daemon is already running")
			}

			// Socket is non-responsive, try to delete it
			log.Warning("socket file exists but isn't responding to commands. " +
				"Attempting to remove it...")
			if err := os.Remove(socketPath); err != nil {
				return nil, fmt.Errorf("could not remove watch daemon at %q. Try 'lsof "+
					"%s'", socketPath, socketPath)
			}
		}
	}
	return nil, fmt.Errorf("watch daemon is already running with a socket "+
		"at %q but not responding. Try: 'lsof %s'", socketPath, socketPath)
}

// ToHTTPServer wraps 'server' in a golang http.Server that uses 'server' to
// serve the TimeTrackerAPI over HTTP on a new socket at 'socketPath'. This
// function is a helper that returns the HTTP server to the caller so that it
// can be shut down later (for tests).  Non-testing users will likely prefer
// ServerOverHTTP, which calls, effectively,
// ToHTTPServer(...).Serve()
func ToHTTPServer(socketPath string, clock Clock, server client.TimeTrackerAPI) (*http.Server, error) {
	h := httpServer{
		clock:     clock,
		apiServer: &LoggingAPI{inner: server},
		startTime: time.Now(),
	}
	mux := http.NewServeMux()
	mux.HandleFunc(socketPath+"/status", h.status)
	mux.HandleFunc(socketPath+"/watch", h.watch)
	mux.HandleFunc(socketPath+"/watches", h.getWatches)
	mux.HandleFunc(socketPath+"/tick", h.tick)
	mux.HandleFunc(socketPath+"/clear", h.clear)
	mux.HandleFunc(socketPath+"/intervals", h.getIntervals)
	mux.Handle(socketPath, http.NotFoundHandler()) // Return to non-endpoint calls with 404
	return &http.Server{Handler: mux}, nil
}

// ServeOverHTTP serves the Server API over HTTP, managing HTTP
// reqests/responses
func ServeOverHTTP(socketPath string, clock Clock, server client.TimeTrackerAPI) error {
	l, err := NewSocketListener(socketPath)
	if err != nil {
		log.Fatal(err.Error())
	}
	s, err := ToHTTPServer(socketPath, clock, server)
	if err != nil {
		log.Fatal(err.Error())
	}
	// Start serving requests
	return s.Serve(l)
}
