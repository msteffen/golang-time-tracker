// interactive_test.go is not a test in the conventional sense. Its job is to
// load test data into the watch daemon and then wait for me to preview the
// /today page.

package watchd

import (
	"os"
	"testing"
	"time"
)

import "fmt"

func TestInteractive(t *testing.T) {
	if os.Getenv("INTERACTIVE") == "" {
		t.Skip("Skip interactive tests during regular testing")
	}
	s := StartTestServer(t)
	fmt.Printf(">>> 1 now: %v", s.TestingClock.Now())
	ts := time.Date(
		/* date */ 2017, 7, 1,
		/* time */ 9, 0, 0,
		/* nsec, location */ 0, time.UTC)
	fmt.Printf(">>> 2 now: %v", s.TestingClock.Now())
	s.Set(ts)
	fmt.Printf(">>> 3 now: %v", s.TestingClock.Now())
	s.TickAt("work", 0, 20, 60, 20)
	fmt.Printf(">>> 4 now: %v", s.TestingClock.Now())
	s.Add(5)
	fmt.Printf(">>> 5 now: %v", s.TestingClock.Now())
	time.Sleep(12 * time.Hour)
}
