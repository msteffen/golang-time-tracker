package watchd

import (
	"testing"

	"github.com/msteffen/golang-time-tracker/client"
	"github.com/msteffen/golang-time-tracker/pkg/check"
)

func TestBasic(t *testing.T) {
	c := NewCollector(
		/* Start */ 0,
		/* End*/ s_Day,
		/* maxEventGap */ 23*s_Minute,
		/* now */ 0)
	curT := int64(0)
	c.Add(curT)
	for _, delta := range []int64{
		1, 1, 1, 1, 1,
		23*60 + 1, 1, 1, 1, 1, 1,
	} {
		curT += delta
		c.Add(curT)
	}
	c.Finish()
	check.T(t, check.Eq(c.intervals, []client.Interval{
		{Start: 0, End: 5},
		{Start: 23*60 + 6, End: 23*60 + 11},
	}))
}
