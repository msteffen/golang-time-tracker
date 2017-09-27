package watchd

import (
	log "github.com/sirupsen/logrus"

	"github.com/msteffen/golang-time-tracker/client"
)

// LoggingAPI wraps an APIServer, but logs all requests and responses
type LoggingAPI struct {
	inner client.TimeTrackerAPI
}

// Watch implements the corresponding method of the APIServer interface, passing
// the call to a.inner and logging the request and response
func (a *LoggingAPI) Watch(req *client.WatchRequest) (retErr error) {
	log.Infof("/watch <- %#v", req)
	defer func() {
		log.Infof("/watch %#v -> %#v", req, retErr)
	}()
	return a.inner.Watch(req)
}

// Tick implements the corresponding method of the APIServer interface, passing
// the call to a.inner and logging the request and response
func (a *LoggingAPI) Tick(req *client.TickRequest) (retErr error) {
	log.Infof("/tick <- %#v", req)
	defer func() {
		log.Infof("/tick %#v -> %#v", req, retErr)
	}()
	return a.inner.Tick(req)
}

// GetIntervals implements the corresponding method of the APIServer interface,
// passing the call to a.inner and logging the request and response
func (a *LoggingAPI) GetIntervals(req *client.GetIntervalsRequest) (resp *client.GetIntervalsResponse, retErr error) {
	log.Infof("/intervals <- [%v, %v]", req.Start, req.End)
	defer func() {
		endGapStr := ""
		if resp.EndGap != 0 {
			endGapStr = " + end gap"
		}
		numIntervals := 0
		if resp != nil && resp.Intervals != nil {
			numIntervals = len(resp.Intervals)
		}
		log.Infof("/intervals [%v, %v] -> (%d intervals%s, %v)",
			req.Start, req.End, numIntervals, endGapStr, retErr)
	}()
	return a.inner.GetIntervals(req)
}

// GetWatches implements the corresponding method of the APIServer interface,
// passing the call to a.inner and logging the request and response
func (a *LoggingAPI) GetWatches(req *client.GetWatchesRequest) (resp *client.GetWatchesResponse, retErr error) {
	log.Infof("/watches")
	defer func() {
		log.Infof("/watches -> (%d watches, %v)", len(resp.Watches), retErr)
	}()
	return a.inner.GetWatches(req)
}

// Clear implements the corresponding method of the APIServer interface, passing
// the call to a.inner and logging the request and response
func (a *LoggingAPI) Clear() (retErr error) {
	log.Infof("/clear")
	defer func() {
		log.Infof("/clear -> %#v", retErr)
	}()
	return a.inner.Clear()
}
