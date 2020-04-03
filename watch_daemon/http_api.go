package watchd

import (
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/http"
	p "path"
	"strconv"
	"strings"
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
	if r.Method != "POST" && r.Method != "GET" {
		http.Error(w, "must use POST to access /tick", http.StatusMethodNotAllowed)
		return
	}
	var req *client.TickRequest
	if r.Method == "POST" {
		req = &client.TickRequest{}
		if err := json.NewDecoder(r.Body).Decode(req); err != nil {
			msg := fmt.Sprintf("request did not match expected type: %v", err)
			http.Error(w, msg, http.StatusBadRequest)
			return
		}
		if req.Label == "" {
			msg := "tick request must have a label (\"\" is used to " +
				"indicate intervals formed by the union of all ticks in GetIntervals"
			http.Error(w, msg, http.StatusBadRequest)
		}
	}

	// Process request
	resp, err := d.apiServer.Tick(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respJSON, err := json.Marshal(resp)
	if err != nil {
		log.Errorf("could not serialize /tick result: %v", err)
		http.Error(w, "could not serialize result: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(respJSON)
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

func (d *httpServer) misc(w http.ResponseWriter, r *http.Request) {
	validFiletype := strings.HasSuffix(r.URL.Path, "js") ||
		strings.HasSuffix(r.URL.Path, "css")
	if r.Method == "GET" && validFiletype {
		data, err := Asset(p.Join("assets", r.URL.Path))
		if err == nil {
			switch {
			case strings.HasSuffix(r.URL.Path, "js"):
				w.Header().Set("Content-Type", "text/javascript")
			case strings.HasSuffix(r.URL.Path, "css"):
				w.Header().Set("Content-Type", "text/css")
			}
			w.Write(data)
			return
		}
	}
	log.Infof("request for unhandled path: %s", r.URL.Path)
	http.Error(w, "404 page not found", http.StatusNotFound)
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

func (d *httpServer) viz(w http.ResponseWriter, r *http.Request) {
	// Unmarshal and validate request
	if r.Method != "GET" {
		http.Error(w, "must use GET to access /viz", http.StatusMethodNotAllowed)
		return
	}
	log.Infof("/viz: %v", time.Now().Sub(d.startTime).String())
	t := TodayOp{
		Server: d.apiServer,
		Now:    d.clock.Now(),
		Writer: w,
	}
	t.Start()
}

// ToHTTPServer wraps 'server' in a golang http.Server that uses 'server' to
// serve the TimeTrackerAPI over HTTP on 'hostport'. This function is a helper
// that returns the HTTP server to the caller so that it can be shut down later
// (for tests). Non-testing users will likely prefer ServerOverHTTP, which
// calls, effectively, ToHTTPServer(...).Serve()
func ToHTTPServer(hostport string, clock Clock, server client.TimeTrackerAPI) *http.Server {
	h := httpServer{
		clock:     clock,
		apiServer: &LoggingAPI{inner: server},
		startTime: time.Now(),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/viz", h.viz)
	mux.HandleFunc("/status", h.status)
	mux.HandleFunc("/watch", h.watch)
	mux.HandleFunc("/watches", h.getWatches)
	mux.HandleFunc("/tick", h.tick)
	mux.HandleFunc("/clear", h.clear)
	mux.HandleFunc("/intervals", h.getIntervals)
	mux.HandleFunc("/", h.misc) // Serve all other assets (js files, or just 404)
	return &http.Server{
		Addr:    hostport,
		Handler: mux,
	}
}

// ServeOverHTTP serves the Server API over HTTP, on the interface/port
// specified by hostport
func ServeOverHTTP(address string, clock Clock, server client.TimeTrackerAPI) error {
	// Check for a running server
	c := &client.Client{Address: address}
	if _, err := c.Status(); err == nil {
		_, port, err := net.SplitHostPort(address)
		advice := fmt.Sprintf("(try 'sudo lsof -i :%s' to find the pid)", port)
		if err != nil {
			advice = fmt.Sprintf("(could not split hostport: %v)", err)
		}
		return fmt.Errorf("watch daemon is already running on address %q %s", address, advice)
	}

	// Start listening on 'address'
	s := ToHTTPServer(address, clock, server)
	return s.ListenAndServe()
}
