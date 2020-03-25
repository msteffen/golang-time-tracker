package watchd

import (
	"html/template"
	"net/http"
	"time"

	"github.com/msteffen/golang-time-tracker/client"
)

type day struct {
	// the date that this day (set of intervals) falls on
	Date time.Time

	// the set of intervals we request from 'server' and must render
	Intervals []client.Interval
}

// TodayOp has all of the internal data structures retrieved/computed while
// generating the /viz HTML page
// Generating HTML templates:
//go:generate make generated_assets.go
type TodayOp struct {
	//// Not Owned
	// The 'server' that handles incoming requests
	Server client.TimeTrackerAPI
	// The current time (injected by creator for testing)
	Now time.Time
	// The http response writer that must receive the result of /today
	Writer http.ResponseWriter

	//// Owned
	// time representing 0:00 today. Computed from 'Now'
	morning time.Time

	// the days being rendered
	days [5]day
}

// Start begins rendering the "today" page
func (t *TodayOp) Start() {
	for i := 0; i < 5; i++ {
		t.days[i].Date = time.Date(t.Now.Year(), t.Now.Month(), t.Now.Day()-4+i,
			/* hour */ 0 /* minute */, 0 /* second */, 0 /* nsec */, 0,
			t.Now.Location())

		// getIntervals generates 'div' structs indicating where "work" divs should be
		// placed (which indicate time when I was working)
		result, err := t.Server.GetIntervals(&client.GetIntervalsRequest{
			Start: t.days[i].Date.Unix(),
			End:   t.days[i].Date.Add(24 * time.Hour).Unix(),
		})
		if err != nil {
			http.Error(t.Writer, err.Error(), http.StatusInternalServerError)
			return
		}
		t.days[i].Intervals = result.Intervals
	}

	// Compute divs and place generated divs into HTML template
	data, err := Asset(`assets/viz.html`)
	if err != nil {
		http.Error(t.Writer, "could not load today.html.template: "+err.Error(),
			http.StatusInternalServerError)
	}
	// intervalsJSON, err := json.Marshal(t.days)
	// if err != nil {
	// 	http.Error(t.Writer, err.Error(), http.StatusInternalServerError)
	// 	return
	// }
	err = template.Must(template.New("").Parse(string(data))).Execute(t.Writer, t.days)
	if err != nil {
		http.Error(t.Writer, err.Error(), http.StatusInternalServerError)
		return
	}
}
