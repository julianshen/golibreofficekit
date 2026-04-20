//go:build linux || darwin

package lokc

/*
#include <stdlib.h>
*/
import "C"

import "unsafe"

// copyAndFree copies a C string into a Go string and frees the
// original with free(3). Safe on nil input (returns "").
//
// LOK returns char* from getError / getVersionInfo / etc. that the
// caller owns; every wrapper that sees such a pointer should pass it
// through here so the free cannot be forgotten.
func copyAndFree(cs *C.char) string {
	if cs == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(cs))
	return C.GoString(cs)
}
