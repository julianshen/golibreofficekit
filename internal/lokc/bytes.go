//go:build linux || darwin

package lokc

/*
#include <stdlib.h>
#include <string.h>
*/
import "C"

import "unsafe"

// copyAndFreeBytes copies n bytes from the C-allocated region p into
// a fresh Go slice and frees p with free(3). Returns nil for nil p
// or n==0. Used by LOK wrappers that return unsigned char* plus an
// explicit size_t length (renderSearchResult, renderShapeSelection
// via its char** output).
func copyAndFreeBytes(p unsafe.Pointer, n C.size_t) []byte {
	if p == nil {
		return nil
	}
	defer C.free(p)
	if n == 0 {
		return nil
	}
	return C.GoBytes(p, C.int(n))
}
