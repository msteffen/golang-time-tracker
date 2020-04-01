package watchd

import "fmt"

import (
	"encoding/json"
	"html/template"
	"net/http"
	"time"

	"github.com/msteffen/golang-time-tracker/client"
)

type day struct {
	// the date that this day (set of intervals) falls on
	Date time.Time

	// the set of intervals we request from 'server' and must render
	Intervals []client.Interval `json:"intervals"`
}

func (d *day) MarshalJSON() ([]byte, error) {
	result := make(map[string]interface{})
	result["date"] = d.Date
	if d.Intervals != nil {
		result["intervals"] = d.Intervals
		var totalTime int64
		for _, i := range d.Intervals {
			totalTime += (i.End - i.Start)
		}
		result["minutes"] = (totalTime / 60) % 60
		result["hours"] = (totalTime / 3600)
	} else {
		result["intervals"] = []struct{}{} // just needs to be a non-nil empty slice
		result["minutes"] = 0
		result["hours"] = 0
	}
	return json.Marshal(result)
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
	days [5]*day
}

// Start begins rendering the "today" page
func (t *TodayOp) Start() {
	for i := 0; i < 5; i++ {
		t.days[i] = &day{
			Date: time.Date(t.Now.Year(), t.Now.Month(), t.Now.Day()-4+i,
				/* hour */ 0 /* minute */, 0 /* second */, 0 /* nsec */, 0,
				t.Now.Location()),
		}

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
	templateBytes, err := Asset(`assets/viz.html`)
	if err != nil {
		http.Error(t.Writer, "could not load today.html.template: "+err.Error(),
			http.StatusInternalServerError)
	}
	tmpl, err := template.New("").Parse(string(templateBytes))
	if err != nil {
		http.Error(t.Writer, err.Error(), http.StatusInternalServerError)
		return
	}
	// html/template automatically converts t.days to JSON, which lower-cases
	// field names and canonicalizes nil fields
	if err := tmpl.Execute(t.Writer, t.days); err != nil {
		http.Error(t.Writer, err.Error(), http.StatusInternalServerError)
		return
	}
}
