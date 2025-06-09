package bigcache

import (
	"bytes"
	"fmt"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

// assertEqual checks if two values are equal using reflect.DeepEqual
func assertEqual(t *testing.T, expected, actual interface{}) {
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Not equal: \nexpected: %v\nactual  : %v", expected, actual)
	}
}

// assertEqualValues checks if two values are equal using reflect.DeepEqual
// It's an alias for assertEqual to help fix the tests
func assertEqualValues(t *testing.T, expected, actual interface{}) {
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Not equal: \nexpected: %v\nactual  : %v", expected, actual)
	}
}

// assertNotEqual checks if two values are not equal using reflect.DeepEqual
func assertNotEqual(t *testing.T, expected, actual interface{}) {
	if reflect.DeepEqual(expected, actual) {
		t.Errorf("Should not be equal: \nexpected: %v\nactual  : %v", expected, actual)
	}
}

// compareByteSlices is used in tests to compare byte slices
func compareByteSlices(a, b []byte) bool {
	return bytes.Equal(a, b)
}

// assertNoError checks that err is nil
func assertNoError(t *testing.T, err error) {
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
}

// assertEqualWithCaller checks if two values are equal and provides more detailed information about the caller
func assertEqualWithCaller(t *testing.T, expected, actual interface{}, msgAndArgs ...interface{}) {
	if !objectsAreEqual(expected, actual) {
		_, file, line, _ := runtime.Caller(1)
		file = filepath.Base(file)
		t.Errorf(fmt.Sprintf("\n%s:%d: Not equal: \n"+
			"expected: %T(%#v)\n"+
			"actual  : %T(%#v)\n",
			file, line, expected, expected, actual, actual), msgAndArgs...)
	}
}

// assertNoErrorWithCaller is an alternative to assertNoError that provides more detailed caller information
func assertNoErrorWithCaller(t *testing.T, e error) {
	if e != nil {
		_, file, line, _ := runtime.Caller(1)
		file = filepath.Base(file)
		t.Errorf(fmt.Sprintf("\n%s:%d: Error is not nil: \n"+
			"actual  : %T(%#v)\n", file, line, e, e))
	}
}

// objectsAreEqual is a helper function that handles byte slice comparison
func objectsAreEqual(expected, actual interface{}) bool {
	if expected == nil || actual == nil {
		return expected == actual
	}

	exp, ok := expected.([]byte)
	if !ok {
		return reflect.DeepEqual(expected, actual)
	}

	act, ok := actual.([]byte)
	if !ok {
		return false
	}
	if exp == nil || act == nil {
		return exp == nil && act == nil
	}
	return bytes.Equal(exp, act)
}
