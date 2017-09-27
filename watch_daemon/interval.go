// collector.go is a library for converting ticks into intervals. Ticks are
// "added" to a collector, and when all ticks have been processed, the collector
// is "finished" and all contained intervals are extracted

package watchd

import "github.com/msteffen/golang-time-tracker/client"

// Collector is a data structure for converting a sequence of ticks into a
// sequence of intervals (ticks separated by t < maxEventGap)
type Collector struct {
	// TODO instead of passing 'maxEventGap' and 'now' from the creating server,
	// include a pointer to the parent server
	//
	// maxEventGap is the maximum amount of time, in seconds, that may elapse
	// between two ticks in the same interval. Gaps of time larger than this will
	// "break" runs of ticks into separate intervals.
	// NewCollector gives this the default value of 23 minutes
	maxEventGap int64
	// now is the current moment in time, as a unix time (seconds since epoch).
	// Used for dealing with in-progress interval)
	now int64

	// lower ((l)eft) and upper ((r)ight) bound times for all intervals in the
	// collection (overlapping intervals are truncated)
	l, r int64

	// Start and end time of the 'current' interval (end advances until a 'wide'
	// gap is encountered)
	start, end int64
	intervals  []client.Interval
	label      string
}

func NewCollector(start, end, maxEventGap, now int64) *Collector {
	return &Collector{
		l:           start,
		r:           end,
		maxEventGap: maxEventGap,
		now:         now,
	}
}

// Add adds a tick to 'c'. 's' is the time at which the tick occurred, as a Unix
// timestamp (seconds since epoch). Returns 'true' if future timestamps should
// be added to 'c', and 'false' otherwise (so that timestamps can be fed to 'c'
// inside a loop of the form 'for c.Add(t) {}')
func (c *Collector) Add(t int64) bool {
	if c.start > c.r { // no overlap with [l, r]. Nothing to do
		return false
	} else if t-c.end <= c.maxEventGap { // Check for interval break
		c.end = t // work interval still going: move 'end' to the right
		return true
	}
	c.addInterval()
	c.start, c.end = t, t // start/end of next interval (end will advance)
	return true
}

// Finish indicates that no more ticks will be added. It closes the last
// interval and returns the complete collectiono
func (c *Collector) Finish() []client.Interval {
	c.addInterval()
	return c.intervals
}

func (c *Collector) addInterval() {
	toAdd := client.Interval{
		Start: max(c.l, c.start),
		End:   min(c.r, c.end),
		Label: c.label,
	}
	if toAdd.End <= toAdd.Start {
		return // toAdd has duration of 0 (or req.End < toAdd.Start) -- skip
	}
	c.intervals = append(c.intervals, toAdd)
}

func min(l, r int64) int64 {
	if l < r {
		return l
	}
	return r
}

func max(l, r int64) int64 {
	if l > r {
		return l
	}
	return r
}
