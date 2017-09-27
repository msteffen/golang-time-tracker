package webui

import (
	"html/template"
	"net/http"
	"time"

	watchd "github.com/msteffen/golang-time-tracker/watch_daemon"
)

type div struct {
	Left, Width int
}

// TodayOp has all of the internal data structures retrieved/computed while
// generating the /today page
type TodayOp struct {
	//// Not Owned
	// The 'server' that handles incoming requests
	Server watchd.APIServer
	// The clock used by 'server' for testing
	Clock watchd.Clock
	// The http response writer that must receive the result of /today
	Writer http.ResponseWriter

	//// Owned
	// the set of intervals we request from 'server' and must render
	intervals []watchd.Interval
	// The intervals in 'intervals' converted to an IR that is easy to render
	divs []div
	// The width of the result html page's background
	BgWidth float64
}

// Start begins rendering the "today" page
func (t *TodayOp) Start() {
	t.getIntervals()
}

// getIntervals generates 'div' structs indicating where "work" divs should be
// placed (which indicate time when I was working)
func (t *TodayOp) getIntervals() {
	now := t.Clock.Now()
	morning := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	result, err := t.Server.GetIntervals(&watchd.GetIntervalsRequest{
		Start: morning.Unix(),
		End:   morning.Add(24 * time.Hour).Unix(),
	})
	if err != nil {
		http.Error(t.Writer, err.Error(), http.StatusInternalServerError)
		return
	}
	t.intervals = result.Intervals
	t.computeDivs()
}

func (t *TodayOp) computeDivs() {
	morning := func() int64 {
		now := t.Clock.Now()
		m := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		return m.Unix()
	}()
	daySecs := (24 * time.Hour).Seconds()
	t.divs = make([]div, 0, len(t.intervals))
	for _, i := range t.intervals {
		t.divs = append(t.divs, div{
			Left:  int(t.BgWidth * float64(i.Start-morning) / daySecs),
			Width: int(t.BgWidth * float64(i.End-i.Start) / daySecs),
		})
	}
	t.generateTemplate()
}

func (t *TodayOp) generateTemplate() {
	// Place generated divs into HTML template
	data, err := Asset(`today.html.template`)
	if err != nil {
		http.Error(t.Writer, "could not load today.html.template: "+err.Error(),
			http.StatusInternalServerError)
	}
	err = template.Must(template.New("").Funcs(template.FuncMap{
		"bgWidth": func() int { return int(t.BgWidth) },
	}).Parse(string(data))).Execute(t.Writer, t.divs)
	if err != nil {
		http.Error(t.Writer, err.Error(), http.StatusInternalServerError)
		return
	}
}
