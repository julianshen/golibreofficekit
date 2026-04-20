//go:build linux || darwin

package lokc

/*
#include <stdlib.h>
*/
import "C"

// cstringMalloc allocates a C string using malloc for use in tests.
// The returned pointer must either be passed to copyAndFree (which
// frees it) or freed explicitly with C.free.
//
// This file is deliberately NOT named *_test.go because cmd/go does
// not allow `import "C"` in a _test.go when the containing package
// already uses cgo in production files (build error: "use of cgo in
// test not supported"). cstringMalloc therefore compiles into the
// production image even though only tests call it — a tiny, cheap
// overhead with no consumers outside the test binary. Do not delete
// as "dead code" on that basis.
func cstringMalloc(s string) *C.char {
	return C.CString(s)
}
