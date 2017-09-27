package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/msteffen/golang-time-tracker/client"
	"github.com/msteffen/golang-time-tracker/pkg/check"
)

var ts = time.Date(
	/* date */ 2017, 7, 1,
	/* time */ 9, 0, 0,
	/* nsec, location */ 0, time.UTC)

func TestBits(t *testing.T) {
	expected := []byte{
		1, 1, 1, 1, 1, 1, 1, 1,
		4, 4, 4, 4, 4, 4,
		6, 7, 7, 7, 8,
	}
	for i, c := range []byte{
		0x01, 0x02, 0x04, 0x08, 0x10, 0x20, 0x40, 0x80,
		0x55, 0xaa, 0x33, 0xcc, 0x0f, 0xf0,
		0xdd, 0xdf, 0xef, 0xfe, 0xff,
	} {
		check.T(t, check.Eq(numBits(c), expected[i]))
	}
}

func StripCtlChars(s string) string {
	var buf bytes.Buffer
	var discard bool
	var i = 0
	for i < len(s) {
		if i+1 < len(s) && s[i:i+2] == "\x1b[" {
			discard = true // CSI is starting
			i += 2
			continue
		}
		if discard {
			if strings.ContainsRune("ABCDEFGHJKSTfmsu", rune(s[i])) {
				discard = false // CSI has ended -- this is the last char
			}
			i++
			continue
		}
		buf.Write([]byte{s[i]})
		i++
	}
	return buf.String()
}

func TestEmptyBar(t *testing.T) {
	barStr := Bar(ts, []client.Interval{})
	check.T(t, check.Eq(StripCtlChars(barStr),
		"[████████████████████████████████████████████████████████████]"))
}

func TestBarBasic(t *testing.T) {
	barStr := StripCtlChars(Bar(ts, []client.Interval{
		{
			Start: ts.Add(4 * time.Minute).Unix(),
			End:   ts.Add(20 * time.Minute).Unix(),
		},
		{
			Start: ts.Add(60 * time.Minute).Unix(),
			End:   ts.Add(240 * time.Minute).Unix(),
		},
		{
			Start: ts.Add(270 * time.Minute).Unix(),
			End:   ts.Add(306 * time.Minute).Unix(),
		},
	}))

	check.T(t,
		check.HasPrefix(StripCtlChars(barStr), "[┃█▌████████▎▊███"),
		check.HasSuffix(barStr, "█████████████████████████████]"),
	)
}

func TestBarIntervalFitsInChar(t *testing.T) {
	barStr := StripCtlChars(Bar(ts, []client.Interval{
		{
			Start: ts.Add(4 * time.Minute).Unix(),
			End:   ts.Add(20 * time.Minute).Unix(),
		},
	}))
	check.T(t,
		check.HasPrefix(barStr, "[┃███"),
		check.HasSuffix(barStr, "████████████████████████████████████████████████████████]"),
	)
}

func TestSGR(t *testing.T) {
	check.T(t,
		check.Eq(sgr("A", "B", "C"), []byte("\x1b[A;B;Cm")),
		check.Eq(sgr("A"), []byte("\x1b[Am")),
		check.Eq(sgr(""), []byte("\x1b[m")),
	)
}
