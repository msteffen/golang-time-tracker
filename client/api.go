package client

// TickRequest is an object sent to the /tick http endpoint, to indicate a file
// save or some other task-related action has occurred
type TickRequest struct {
	// Label identifies the task on which the user is currently working
	Label string `json:"label"`
}

// WatchRequest is an object sent to the /watch http endpoint, to indicate that
// the watcher daemon should begin watching the directory
type WatchRequest struct {
	// Label identifies the task associated with 'Dir'
	Label string `json:"label"`

	// Dir is the directory that the daemon should watch
	Dir string `json:"dir"`
}

// GetIntervalsRequest is the object sent to the /intervals endpoint.
type GetIntervalsRequest struct {
	// The time period in which we want to get intervals, as seconds since epoch.
	// If an interval in the result overlaps with 'Start' or 'End', it will be
	// truncated.
	Start int64 `json:"start"`
	End   int64 `json:"end"`
}

// Interval represents a time interval in which the caller was working. Used in
// GetIntervalsResponse.
type Interval struct {
	// Start is the start time (secs since Unix epoch) of the interval
	Start int64 `json:"start"`
	// End is the end time (secs since Unix epoch) of the interval
	End int64 `json:"end"`

	// The activity that was done in this interval (or "" if multiple activities
	// may have occurred)
	Label string `json:"label"`
}

// GetIntervalsResponse contains all activity intervals, clamped to the
// requested start/end times, sorted by start time
type GetIntervalsResponse struct {
	// Intervals contain the set of intervals stored by the time-tracker server
	// that overlap with [req.Start, req.End] (and have been truncated to fit in
	// this range).
	Intervals []Interval `json:"intervals"`

	// EndGap is the amount of time (in seconds) added to the last interval in
	// 'Intervals', reflecting where the interval would end if a tick were sent
	// now.
	EndGap int64 `json:"end_gap"`
}

// GetWatchesRequest is the request object sent to the /watches endpoint.
type GetWatchesRequest struct{}

// WatchInfo describes a "watch" that has been installed in the watch daemon
type WatchInfo struct {
	// LastWrite is the time of the most recent write under 'Dir' (secs since Unix
	// epoch)
	LastWrite int64 `json:"last_write"`

	// Dir identifies a directory being watched
	Dir string `json:"dir"`

	// Label is the label associated with this watch
	Label string `json:"label"`
}

// GetWatchesResponse indicates all currently-watched directories
type GetWatchesResponse struct {
	// TODO(msteffen): should these be pointers or raw values?
	Watches []*WatchInfo
}

// TimeTrackerAPI is the interface exported by the watch daemon
type TimeTrackerAPI interface {
	Watch(req *WatchRequest) error
	GetWatches(req *GetWatchesRequest) (*GetWatchesResponse, error)
	Tick(req *TickRequest) error
	GetIntervals(req *GetIntervalsRequest) (*GetIntervalsResponse, error)
	Clear() error
}
