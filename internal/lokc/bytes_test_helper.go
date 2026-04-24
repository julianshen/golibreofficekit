//go:build linux || darwin

package lokc

/*
#include <stdlib.h>
#include <string.h>
*/
import "C"

import "unsafe"

// cmallocCopy malloc's a fresh C buffer the size of b, copies b's
// contents into it, and returns the raw pointer. Test-only; helpers
// that use cgo live in non-test files so go-test's
// "cgo in _test.go files not supported" restriction doesn't fire.
func cmallocCopy(b []byte) unsafe.Pointer {
	if len(b) == 0 {
		return C.malloc(1) // non-nil, irrelevant contents
	}
	p := C.malloc(C.size_t(len(b)))
	C.memcpy(p, unsafe.Pointer(&b[0]), C.size_t(len(b)))
	return p
}

// cmallocRaw returns a fresh C buffer of n bytes (uninitialised).
// Used when we want a non-nil C pointer for a zero-length test.
func cmallocRaw(n int) unsafe.Pointer {
	if n == 0 {
		n = 1
	}
	return C.malloc(C.size_t(n))
}

// copyAndFreeBytesTest is a thin cgo-free wrapper around
// copyAndFreeBytes so tests can exercise it without importing "C".
func copyAndFreeBytesTest(p unsafe.Pointer, n int) []byte {
	return copyAndFreeBytes(p, C.size_t(n))
}
