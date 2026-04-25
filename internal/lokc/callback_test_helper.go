//go:build linux || darwin

package lokc

/*
#include <stdlib.h>
#include <string.h>
#include <stdint.h>

// uintptr_to_voidp converts an integer handle to a void* at the C
// level so that Go's checkptr instrumentation (enabled by -race) does
// not flag the integer-to-pointer conversion.
static void* uintptr_to_voidp(uintptr_t v) { return (void*)v; }
*/
import "C"

import "unsafe"

// testDispatchOffice calls goLOKDispatchOffice with a Go string payload
// converted to a C string. Used only by tests; lives in a non-_test.go
// file because _test.go files may not contain import "C".
func testDispatchOffice(typ int, payload *string, handle dispatchHandle) {
	pData := C.uintptr_to_voidp(C.uintptr_t(handle))
	if payload == nil {
		goLOKDispatchOffice(C.int(typ), nil, pData)
		return
	}
	cs := C.CString(*payload)
	defer C.free(unsafe.Pointer(cs))
	goLOKDispatchOffice(C.int(typ), cs, pData)
}

// testDispatchDocument calls goLOKDispatchDocument with a Go string payload.
func testDispatchDocument(typ int, payload *string, handle dispatchHandle) {
	pData := C.uintptr_to_voidp(C.uintptr_t(handle))
	if payload == nil {
		goLOKDispatchDocument(C.int(typ), nil, pData)
		return
	}
	cs := C.CString(*payload)
	defer C.free(unsafe.Pointer(cs))
	goLOKDispatchDocument(C.int(typ), cs, pData)
}
