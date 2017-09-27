// escape provides a generic way to encode sequences of strings as a single
// string. The watch daemon happens to use it to encode each tick's labels in
// single column in SQLite

package escape

import (
	"bytes"
)

func countSpecialChars(s string) int {
	var result int
	for _, c := range s {
		if c == '\\' || c == '"' {
			result++
		}
	}
	return result
}

// Escape escapes all instances of '"' and '\'
func Escape(label string) string {
	buf := &bytes.Buffer{}
	// Grow buf to hold the result
	buf.Grow(len(label) + countSpecialChars(label))
	// Write all labels into 'buf'
	for _, c := range label {
		switch c {
		case '"':
			buf.Write([]byte{'\\', '"'})
		case '\\':
			buf.Write([]byte{'\\', '\\'})
		default:
			buf.WriteRune(c)
		}
	}
	return buf.String()
}

// Unescape unescapes all instances of '"' and '\'
func Unescape(in string) string {
	out := &bytes.Buffer{}
	escaped := false
	for _, c := range in {
		if !escaped && c == '\\' {
			escaped = true
			continue
		}
		out.WriteRune(c)
		escaped = false
	}
	return out.String()
}
