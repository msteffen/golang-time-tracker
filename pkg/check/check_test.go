package check

import (
	"errors"
	"testing"
)

func TestCheckBasic(t *testing.T) {
	T(t,
		True(true),
		False(false),
		Nil(nil),
		NotNil(errors.New("test error")),
		Eq(1, 1),
		Eq("Testing", "Testing"))
}

// TestEq checks the main feature of Check.Eq, which is that it reguards empty
// slices and nil slices as identical
func TestEq(t *testing.T) {
	T(t,
		Eq([]int{}, []int(nil)),
		Eq([]int(nil), []int{}))
}

// TestEqInStruct checks that empty slices and nil slices are reguarded as
// identical even when they're embedded in struct fields
func TestEqInStruct(t *testing.T) {
	type s struct {
		Empty []int
		Nil   []int
	}

	// actual is opposite of expected
	Eq(
		s{Empty: nil,
			Nil: make([]int, 0)},
		s{Empty: make([]int, 0),
			Nil: nil})
	// test struct pointer as well
	Eq(
		&s{Empty: nil,
			Nil: make([]int, 0)},
		&s{Empty: make([]int, 0),
			Nil: nil})
}

// TestEqInStruct checks that empty slices and nil slices are reguarded as
// identical even when they're embedded in a parent slice
func TestEqInSlice(t *testing.T) {
	// actual is opposite of expected
	Eq(
		[][]int{nil, make([]int, 0)},
		[][]int{make([]int, 0), nil})
}
