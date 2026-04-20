//go:build linux || darwin

package lokc

/*
#include <stdlib.h>
*/
import "C"

// cstringMalloc allocates a C string using malloc for use in tests.
// The returned pointer must either be passed to copyAndFree (which
// frees it) or freed explicitly with C.free. This helper exists because
// Go prohibits import "C" in _test.go files.
func cstringMalloc(s string) *C.char {
	return C.CString(s)
}
