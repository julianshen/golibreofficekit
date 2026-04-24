//go:build linux || darwin

package lokc

/*
#cgo CFLAGS: -I${SRCDIR}/../../third_party/lok -DLOK_USE_UNSTABLE_API
#include <stdlib.h>
#include <stdbool.h>
#include "LibreOfficeKit/LibreOfficeKit.h"

static bool go_doc_render_search_result(LibreOfficeKitDocument* d, const char* q,
    unsigned char** outBuf, int* outW, int* outH, size_t* outSize) {
    *outBuf = NULL; *outW = 0; *outH = 0; *outSize = 0;
    if (d == NULL || d->pClass == NULL || d->pClass->renderSearchResult == NULL) return false;
    return d->pClass->renderSearchResult(d, q, outBuf, outW, outH, outSize);
}
static size_t go_doc_render_shape_selection(LibreOfficeKitDocument* d, char** outBuf) {
    *outBuf = NULL;
    if (d == NULL || d->pClass == NULL || d->pClass->renderShapeSelection == NULL) return 0;
    return d->pClass->renderShapeSelection(d, outBuf);
}
*/
import "C"

import "unsafe"

// DocumentRenderSearchResult renders the first match of the LOK
// search-item JSON query as a premul BGRA bitmap. Returns
// (nil, 0, 0, false) when LOK returns false (no match) or when the
// handle/vtable is unavailable. The caller-owned bitmap is copied
// into a Go slice and the C buffer is freed before return.
func DocumentRenderSearchResult(d DocumentHandle, query string) (buf []byte, pxW, pxH int, ok bool) {
	if !d.IsValid() {
		return nil, 0, 0, false
	}
	cq := C.CString(query)
	defer C.free(unsafe.Pointer(cq))

	var cbuf *C.uchar
	var cw, ch C.int
	var csize C.size_t
	res := C.go_doc_render_search_result(d.p, cq, &cbuf, &cw, &ch, &csize)
	if !bool(res) {
		// LOK guarantees a NULL buffer on false; defensive free anyway.
		if cbuf != nil {
			C.free(unsafe.Pointer(cbuf))
		}
		return nil, 0, 0, false
	}
	return copyAndFreeBytes(unsafe.Pointer(cbuf), csize), int(cw), int(ch), true
}

// DocumentRenderShapeSelection returns the current shape selection's
// bytes (SVG in practice on LO 24.8). Returns nil when nothing is
// selected or the handle/vtable is unavailable. The LOK-allocated
// buffer is copied and freed before return.
func DocumentRenderShapeSelection(d DocumentHandle) []byte {
	if !d.IsValid() {
		return nil
	}
	var cbuf *C.char
	n := C.go_doc_render_shape_selection(d.p, &cbuf)
	return copyAndFreeBytes(unsafe.Pointer(cbuf), n)
}
