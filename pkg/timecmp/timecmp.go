package timecmp

import "time"

// Leq returns true if l <= r
func Leq(l, r time.Time) bool {
	return l.Before(r) || l.Equal(r)
}

// Max returns whichever of l or r is greatest (farther in the future)
func Max(l, r time.Time) time.Time {
	if l.Before(r) {
		return r
	}
	return l
}

// Min returns whichever of l or r is least (farther in the past)
func Min(l, r time.Time) time.Time {
	if l.Before(r) {
		return l
	}
	return r
}
