package check

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// False confirms that 'b' is false
func False(b bool) error {
	if b {
		return fmt.Errorf("expected \"false\", but was \"true\"")
	}
	return nil
}

// True confirms that 'b' is true
func True(b bool) error {
	if !b {
		return fmt.Errorf("expected \"true\", but was \"false\"")
	}
	return nil
}

// Nil accpets a string of arguments and checks that the final argument is a nil
// error (accepting multiple arguments and ignoring the first N-1 is a
// convenience, so that functions returning multiple arguments, the last of
// which is an error, can be called directly inside check.Nil(...), rather than
// requiring users to ignore the non-error responses explicitly)
func Nil(vals ...interface{}) error {
	last := vals[len(vals)-1]
	if last == nil {
		return nil
	}
	err, ok := last.(error)
	if !ok {
		return fmt.Errorf("expected error as last argument to check.Nil, but was: %T", last)
	}
	if err != nil {
		return fmt.Errorf("expected <nil> error, but was:\n	%v", err)
	}
	return nil
}

// NotNil confirms that 'err' is non-nil
func NotNil(err error) error {
	if err == nil {
		return fmt.Errorf("expected non-<nil> error, but was:\n	%v", err)
	}
	return nil
}

// HasPrefix confirms that 'text' has the prefix 'prefix'
func HasPrefix(text interface{}, prefix interface{}) error {
	cStr, cOK := text.(string)
	pStr, pOK := prefix.(string)
	if cOK && pOK {
		if strings.HasPrefix(cStr, pStr) {
			return nil
		}
		return fmt.Errorf("expected: %q\n to be prefix of: %q\nbut it was not", pStr, cStr)
	}
	cBytes, cOK := text.([]byte)
	pBytes, pOK := prefix.([]byte)
	if cOK && pOK {
		if bytes.HasPrefix(cBytes, pBytes) {
			return nil
		}
		return fmt.Errorf("expected: %q\n to be prefix of: %q\nbut it was not", pBytes, cBytes)
	}
	return fmt.Errorf("expected (string, string) or ([]byte, []byte) but got (%T, %T)", text, prefix)
}

// HasSuffix confirms that 'text' has the suffix 'suffix'
func HasSuffix(text interface{}, suffix interface{}) error {
	cStr, cOK := text.(string)
	pStr, pOK := suffix.(string)
	if cOK && pOK {
		if strings.HasSuffix(cStr, pStr) {
			return nil
		}
		return fmt.Errorf("expected: %q\n to be suffix of: %q\nbut it was not", pStr, cStr)
	}
	cBytes, cOK := text.([]byte)
	pBytes, pOK := suffix.([]byte)
	if cOK && pOK {
		if bytes.HasSuffix(cBytes, pBytes) {
			return nil
		}
		return fmt.Errorf("expected: %q\n to be suffix of: %q\nbut it was not", pBytes, cBytes)
	}
	return fmt.Errorf("expected (string, string) or ([]byte, []byte) but got (%T, %T)", text, suffix)
}

// eq is a helper for Eq that takes reflect.Values instead of interface{}es.
// This is purely to prevent recursive calls to Eq from having to convert back
// and forth between interfaces and reflect.Values.
func eq(actual, expected reflect.Value) error {
	if expected.Kind() != actual.Kind() {
		return fmt.Errorf("expected: \"%#v\"\n but was: \"%#v\"", expected, actual)
	}
	switch expected.Kind() {
	case reflect.Slice, reflect.Array, reflect.String:
		return sliceEq(actual, expected)
	case reflect.Ptr, reflect.Interface:
		return eq(expected.Elem(), actual.Elem())
	case reflect.Struct:
		return structEq(actual, expected)
	default:
		// Handle all other cases
		if reflect.DeepEqual(expected.Interface(), actual.Interface()) {
			return nil
		}
		expectedStr := fmt.Sprintf("\"%#v\"", expected)
		if es, ok := expected.Interface().(fmt.Stringer); ok {
			expectedStr = es.String()
		}
		actualStr := fmt.Sprintf("\"%#v\"", actual)
		if as, ok := actual.Interface().(fmt.Stringer); ok {
			actualStr = as.String()
		}
		return fmt.Errorf("expected: %s\n but was: %s", expectedStr, actualStr)
	}
}

// sliceEq is a helper for Eq that, unlike reflect.DeepEqual, returns OK if one
// slice is nil and the other is empty (convenient for tests not to specify
// empty slices everywhere). It also recursively uses eq() for slice elements
// (so e.g. slices of slices are handled similarly)
func sliceEq(actual, expected reflect.Value) error {
	if expected.Len() != actual.Len() {
		return fmt.Errorf("expected: %#v (length %d)\n but was: %#v (length %d)",
			expected, expected.Len(), actual, actual.Len())
	}
	for i := 0; i < expected.Len(); i++ {
		if err := eq(actual.Index(i), expected.Index(i)); err != nil {
			return fmt.Errorf("expected: %#v\n but was: %#v\ndifference at index %d:\n%v",
				expected, actual, i, err)
		}
	}
	return nil
}

// structEq is a helper for eq() that recursively uses eq() (w/ special slice
// handling) rather that reflect.DeepEqual, which recursively uses
// reflect.DeepEqual
func structEq(actual, expected reflect.Value) error {
	for i := 0; i < expected.NumField(); i++ {
		if err := eq(actual.Field(i), expected.Field(i)); err != nil {
			return fmt.Errorf("at field %q:\n%v", actual.Type().Field(i).Name, err)
		}
	}
	return nil
}

// Eq confirms that 'expected' and 'actual' are equal
func Eq(actual, expected interface{}) error {
	return eq(reflect.ValueOf(actual), reflect.ValueOf(expected))
}

// T checks one or more testing conditions, and calls t.Fatal() if any aren't
// met
func T(t testing.TB, errs ...error) {
	t.Helper()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err.Error())
		}
	}

}
