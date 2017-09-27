// interactive_test.go is not a test in the conventional sense. Its job is to
// load test data into the watch daemon and then wait for me to preview the
// /today page.

package webui

import (
	"os"
	"testing"
	"time"

	watchd "github.com/msteffen/golang-time-tracker/watch_daemon"
)

func TestTwoIntervals(t *testing.T) {
	if os.Getenv("TIMETRACKER_INTERACTIVE_TESTS") == "" {
		t.Skip("Skip interactive tests during regular testing")
	}
	os.Mkdir("test-interactive", 0755)
	s := watchd.StartTestServer(t, "test-interactive")
	ts := time.Date(
		/* date */ 2017, 7, 1,
		/* time */ 9, 0, 0,
		/* nsec, location */ 0, time.UTC)
	s.Set(ts)
	s.TickAt("", 0, 20, 60, 20)
	s.Add(5)
	time.Sleep(12 * time.Hour)
}
