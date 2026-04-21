//go:build linux || darwin

package lokc

/*
#include <stdlib.h>
#include "LibreOfficeKit/LibreOfficeKit.h"

static LibreOfficeKitDocument* newFakeDoc(void) {
    return (LibreOfficeKitDocument*)calloc(1, sizeof(LibreOfficeKitDocument));
}

static void freeFakeDoc(LibreOfficeKitDocument* d) { free(d); }
*/
import "C"

// NewFakeDocumentHandle returns a DocumentHandle backed by a calloc'd
// LibreOfficeKitDocument with pClass == NULL. The wrappers' C-side
// guards make every call a safe no-op while the Go-side CString/free
// path runs, so tests can unit-test the Go layer without real LO.
func NewFakeDocumentHandle() DocumentHandle {
	return DocumentHandle{p: C.newFakeDoc()}
}

// FreeFakeDocumentHandle releases the backing memory.
func FreeFakeDocumentHandle(d DocumentHandle) {
	if d.p != nil {
		C.freeFakeDoc(d.p)
	}
}
