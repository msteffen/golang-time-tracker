package escape

import (
	"testing"

	"github.com/msteffen/golang-time-tracker/pkg/check"
)

func TestEscape(t *testing.T) {
	for _, label := range []string{
		"th\"i\"s", "\"", "\\", "\\is\\", "\"\\\"", "a", "test",
	} {
		check.T(t, check.Eq(Unescape(Escape(label)), label))
	}
}
