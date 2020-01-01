// interactive_test.go is not a test in the conventional sense. Its job is to
// load test data into the watch daemon and then wait for me to preview the
// /today page.

package watchd

import (
	"os"
	"testing"
	"time"
)

func TestInteractive(t *testing.T) {
	if os.Getenv("INTERACTIVE") == "" {
		t.Skip("Skip interactive tests during regular testing")
	}
	s := StartTestServer(t)
	ts := time.Date(
		/* date */ 2017, 7, 1,
		/* time */ 9, 0, 0,
		/* nsec, location */ 0, time.UTC)
	s.Set(ts)
	s.TickAt("work", 0, 20, 60, 20)
	s.Add(5)
	time.Sleep(12 * time.Hour)
}
