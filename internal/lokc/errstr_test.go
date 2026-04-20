//go:build linux || darwin

package lokc

import "testing"

func TestCopyAndFree_RoundTrip(t *testing.T) {
	// Heap-allocate a C string the same way LOK does (malloc'd, caller frees).
	// cstringMalloc is used instead of C.CString directly because Go prohibits
	// import "C" in _test.go files.
	msg := cstringMalloc("hello, lok")
	// copyAndFree must free msg and return the Go string.
	got := copyAndFree(msg)
	if got != "hello, lok" {
		t.Errorf("got %q", got)
	}
}

func TestCopyAndFree_Nil(t *testing.T) {
	if got := copyAndFree(nil); got != "" {
		t.Errorf("nil input: got %q want \"\"", got)
	}
}
