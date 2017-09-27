// This file has one signicant function, 'Bar()' that converts a slice of
// intervals spanning a 24-hour period into a textual bar that can be printed.
// The algorithm it uses for doing this is:
// 1. break the day up into 60 "characters" each of which represents 24 minutes,
//    and then break the character up into 8 bits, each representing 3 minutes
// 2. A bit is "on" if most of its 3 minutes is covered by intervals in the
//    slice of intervals, and off otherwise
// 3. Once all the bits in a character have been determined, compare it to each
//    of the bytes in 'blockMask' below, and choose the blockMask byte that is
//    bitwise closest.
// 4. Each of the bytes maps to a unicode character (possibly in inverted video
//    mode--e.g. a partially-filled left box that has been inverted to become a
//    partially-filled right box). Print the control character/unicode character
//    corresponding to the blockMask byte from (3).

package main

import (
	"bytes"
	// "fmt"
	"time"

	"github.com/msteffen/golang-time-tracker/client"
	"github.com/msteffen/golang-time-tracker/pkg/timecmp"
)

const resetAll = "0"
const invertColors = "7"
const uninvertColors = "27"
const boldText = "1"

// SGR code for setting the foreground or background color using 8-bit SGR
// terminal color codes
const setFGColor = "38;5"
const setBGColor = "48;5"

// BG color for 6-hour marks (chosen to look good on white or black BG)
const markColor = "247" // light gray

// barColor is the ANSI terminal code such that "\e[38;5;" + barColor + "m" sets
// the FG to be the color we use for the interval bar
const barColor = "220"

// darkBarColor is similar to 'barColor' but is the color we use for
// characters on the 6-hour mark
// (6-hour marks are highlighted so that bars are easier to read)
const barDarkColor = "178" // slightly lighter than 220

const shortColor = "69"     // purple, for intervals that don't count
const shortDarkColor = "27" // darker purple

// like barColor, but also has an SGR code for "bold"
const boldBarColor = "1;" + barColor

// Note that the unicode box drawing characters look like:
// full box -> left eighth box
// 0x2588 ...  0x258f
// █ ▉ ▊ ▋ ▌ ▍ ▎ ▏
const fullBlock = 0x2588

// Also used
const lightVerticalLine = 0x2502 // = [│], about 1/8
const thickVerticalLine = 0x2503 // = [┃], about 3/8

var blockMask = [...]byte{
	// 0 - off, 1 - full
	0x00, 0xff,

	// [2-8] left boxes:
	0xfe, // 11111110
	0xfc, // 11111100
	0xf8, // 11111000
	0xf0, // 11110000
	0xe0, // 11100000
	0xc0, // 11000000
	0x80, // 10000000

	// [9-15] right boxes
	0x7f, // 01111111
	0x3f, // 00111111
	0x1f, // 00011111
	0x0f, // 00001111
	0x07, // 00000111
	0x03, // 00000011
	0x01, // 00000001

	// [16-26] thin vertical line
	0x40, // 01000000
	0x20, // 00100000
	0x10, // 00010000
	0x08, // 00001000
	0x04, // 00000100
	0x02, // 00000010
	0x60, // 01100000
	0x30, // 00110000
	0x18, // 00011000
	0x0c, // 00001100
	0x06, // 00000110

	// [27-37] inverted thin vertical line
	0xbf, // 10111111
	0xdf, // 11011111
	0xef, // 11101111
	0xf7, // 11110111
	0xfb, // 11111011
	0xfd, // 11111101
	0x9f, // 10011111
	0xcf, // 11001111
	0xe7, // 11100111
	0xf3, // 11110011
	0xf9, // 11111001

	// [38-47] thick vertical line
	0x70, // 01110000
	0x78, // 01111000
	0x7c, // 01111100
	0x7e, // 01111110
	0x38, // 00111000
	0x3c, // 00111100
	0x3e, // 00111110
	0x1c, // 00011100
	0x1e, // 00011110
	0x0e, // 00001110

	// [48-57] inverted thick vertical line
	0x8f, // 10001111
	0x87, // 10000111
	0x83, // 10000011
	0x81, // 10000001
	0xc7, // 11000111
	0xc3, // 11000011
	0xc1, // 11000001
	0xe3, // 11100011
	0xe1, // 11100001
	0xf1, // 11110001
}

// numBits counts the number of ones in 'c'
func numBits(c byte) byte {
	for i, m := range []byte{0x55, 0x33, 0x0f} {
		var p byte = 1 << byte(i)
		c = ((c >> p) & m) + (c & m)
	}
	return c
}

// eighths rounds the given duration in seconds to the nearest 1/8 of 24 minutes
// The result is a rune so that it can be used to do unicode arithmetic
// (e.g. fullBlock + x)
func eighths(duration int64) rune {
	// 1/8 of (24 * 60) seconds = 180. (x+90)/180 => round to the nearest eighth
	return rune((duration + 90) / 180)
}

type barBuf struct {
	// buf contains result of computing the day's bar
	buf bytes.Buffer

	// number of block characters that have been written
	size int

	// whether the buffer has set the terminal to be inverted
	inverted bool

	// whether the buffer is using the short color (currently purple) as FG or the
	// regular color (currently yellow)
	short bool

	// whether the buffer is on a 6-h mark character or not
	// (6-hour marks are highlighted so that bars are easier to read)
	onMark bool
}

// sgr takes the ANSI sgr codes in "codes", concatenates them, and wraps them
// in the ANSI escape code for sgr commands (yielding "0x1b[...m")
func sgr(codes ...string) []byte {
	var fullLen int
	for _, s := range codes {
		fullLen += len(s)
	}
	fullLen += len(codes) - 1 // also add space for intermediate semicolons

	// allocate result && copy initial SGR sequence
	result := make([]byte, fullLen+3)
	copy(result, "\x1b[")
	curLen := 2

	// copy codes separated by semicolons
	for i, s := range codes {
		if i == 0 {
			copy(result[curLen:], s) // no semicolon
			curLen += len(s)
			continue
		}
		copy(result[curLen:], ";")
		copy(result[curLen+1:], s)
		curLen += len(s) + 1
	}

	// copy final "m"
	result[curLen] = 'm'
	return result
}

func newBarBuf() *barBuf {
	op := &barBuf{}
	op.buf.WriteByte('[')
	return op
}

// writeInverted is a helper function that writes 'r' to b.buf with inverted
// colors (i.e. if colors are already inverted, it just writes 'r')
func (b *barBuf) writeInverted(r rune) {
	if !b.inverted {
		// SGR code -- 7 = invert colors
		b.buf.Write(sgr(invertColors))
		b.inverted = true
	}
	b.buf.WriteRune(r)
}

// writeNormal is a helper function that writes 'r' to b.buf with non-inverted
// colors (if colors are already normal, it just writes 'r')
func (b *barBuf) writeNormal(r rune) {
	if b.inverted {
		// SGR code -- 27 = inverted colors off (regular colors)
		b.buf.Write(sgr(uninvertColors))
		b.inverted = false
	}
	b.buf.WriteRune(r)
}

// finish adds the necessary trailing characters to b.buf and returns it as a
// string
func (b *barBuf) finish() string {
	// reset colors completely and close with ']'
	b.buf.Write(sgr(resetAll))
	b.buf.WriteByte(']')
	return b.buf.String()
}

func (b *barBuf) put(i int, short bool) {
	// Determine FG and BG color changes needed
	onMark := b.size > 0 && b.size%15 == 0
	enableMark, disableMark := !b.onMark && onMark, !onMark && b.onMark
	enableShort := i > 0 && !b.short && short // don't set short if i == 0 (empty)
	disableShort := i > 0 && b.short && !short

	if enableMark {
		b.buf.Write(sgr(setBGColor, markColor))
		b.onMark = true
	} else if disableMark {
		b.buf.Write(sgr(resetAll))
		b.onMark = false
		b.inverted = false
	}
	// for {en,dis}ableMark, we always need to set the FG color
	// Otherwise, we only set the FG color if {en,dis}ableShort is set
	changeFG := enableMark || disableMark || enableShort || disableShort
	if changeFG {
		switch {
		case b.onMark && short:
			b.buf.Write(sgr(setFGColor, shortDarkColor))
			b.short = true
		case b.onMark && !short:
			b.buf.Write(sgr(setFGColor, barDarkColor))
			b.short = false
		case !b.onMark && short:
			b.buf.Write(sgr(setFGColor, shortColor))
			b.short = true
		case !b.onMark && !short:
			b.buf.Write(sgr(setFGColor, barColor))
			b.short = false
		}
	}

	// write 'i' to buf
	switch {
	case i == 0:
		// 0 - off
		b.writeInverted(fullBlock)
	case i == 1:
		// 1 - full
		b.writeNormal(fullBlock)
	case i <= 8:
		// [2-8] left boxes:
		b.writeNormal(fullBlock + rune(i) - 1)
	case i <= 15:
		// [9-15] right boxes
		b.writeInverted(fullBlock + 16 - rune(i))
	case i <= 26:
		// [16-26] thin vertical line
		b.writeNormal(lightVerticalLine)
	case i <= 37:
		// [27-37] inverted thin vertical line
		b.writeInverted(lightVerticalLine)
	case i <= 47:
		// [38-47] thick vertical line
		b.writeNormal(thickVerticalLine)
	case i <= 57:
		// [48-57] inverted thick vertical line
		b.writeInverted(thickVerticalLine)
	}
	b.size++
}

var emptyBar = func() string {
	buf := newBarBuf()
	for i := 0; i < 60; i++ {
		buf.put(0, false)
	}
	return buf.finish()
}()

// Bar generates a bar containing a day's worth of intervals (for raw 't' cmd)
func Bar(morning time.Time, intervals []client.Interval) (res string) {
	if len(intervals) == 0 {
		return emptyBar // special case; no intervals
	}

	// - A bar/line represents one day
	// - each bar/line is 60 chars => each char is 24 minutes (60*24 mins per day)
	// - each char is 8 bits. Because bars are rendered from left to right, bits
	//   are "reversed" within their byte (high bit = earlier/lower time of day):
	//         0            0            0            1             1       ...
	//   [0:00, 0:03) [0:03, 0:06) [0:06, 0:09) [0:09, 0:12), [0:12, 0:15), ....
	var (
		buf = newBarBuf()

		// left and right boundary of current window (3 minutes/one bit, in loop)
		cl, cr = time.Time{}, morning

		// Current interval index, and left/right boundaries
		n      = 0
		il, ir = time.Unix(intervals[0].Start, 0), time.Unix(intervals[0].End, 0)

		// The current "character" (24-minute window)
		window byte
		// true if the first interval overlapping this character is short (<1h)
		short             bool
		firstIntersection = true
	)
	for i := 0; i < (60 * 8); i++ {
		// each loop: render the i'th bit
		cl = cr
		cr = cl.Add(3 * time.Minute)

		// Determine amount of interval in [cl, cr]
		var duration time.Duration
		for {
			if n == len(intervals) {
				break // no more intervals to overlap
			}
			if cr.Before(il) {
				break
			}
			if timecmp.Leq(cl, ir) {
				// il <= cr and cl <= ir, there is overlap
				duration += timecmp.Min(cr, ir).Sub(timecmp.Max(cl, il))

				if firstIntersection {
					firstIntersection = false
					if ir.Sub(il) < time.Hour {
						short = true
					}
					// fmt.Printf(">>> %s (%t)\n", ir.Sub(il), short)
				}
			}
			if timecmp.Leq(cr, ir) {
				break // intervals[n] overlaps next bit as well
			}

			n++
			if n < len(intervals) {
				il, ir = time.Unix(intervals[n].Start, 0), time.Unix(intervals[n].End, 0)
			}
		}
		// set the bit in the current character being rendered
		if duration > 90*time.Second {
			window |= (1 << byte(7-(i%8)))
		}

		// we've reached end of a byte--set the current character
		if i%8 == 7 {
			// Window is filled out -- append to bar
			// fmt.Printf("window: %s\n", bin(window))
			if window == 0 || window == 0xff {
				// minor hack--calls put(0) (window == 0) or put(1) (window == 0xff)
				buf.put(int(window>>7), short)
			} else {
				best := -1
				bestCount := byte(8)

				// find element in blockMask that minimizes xor(window)
				for bi, b := range blockMask {
					diff := numBits(b ^ window)
					if diff < bestCount {
						best = bi
						bestCount = diff
					}
					if diff == 0 {
						break
					}
				}
				buf.put(best, short)
			}
			window, short, firstIntersection = 0, false, true
		}
	}
	return buf.finish()
}

func bin(x byte) string {
	var result [8]byte
	for i := 0; i < 8; i++ {
		if x%2 == 1 {
			result[7-i] = '1'
		} else {
			result[7-i] = '0'
		}
		x >>= 1
	}
	return string(result[:])
}
