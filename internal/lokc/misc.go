//go:build linux || darwin

package lokc

/*
#cgo CFLAGS: -I${SRCDIR}/../../third_party/lok -DLOK_USE_UNSTABLE_API
#include <stdlib.h>
#include "LibreOfficeKit/LibreOfficeKit.h"

static char* loke_office_get_filter_types(LibreOfficeKit *p) {
    if (p == NULL || p->pClass == NULL || p->pClass->getFilterTypes == NULL) return NULL;
    return p->pClass->getFilterTypes(p);
}

static int loke_doc_paste(LibreOfficeKitDocument *d, const char *mime,
                          const char *data, int size) {
    if (d == NULL || d->pClass == NULL || d->pClass->paste == NULL) return -1;
    return d->pClass->paste(d, mime, data, (size_t)size) ? 1 : 0;
}

static void loke_doc_select_part(LibreOfficeKitDocument *d, int part, int sel) {
    if (d == NULL || d->pClass == NULL || d->pClass->selectPart == NULL) return;
    d->pClass->selectPart(d, part, sel);
}

static void loke_doc_move_selected_parts(LibreOfficeKitDocument *d, int pos, int dup) {
    if (d == NULL || d->pClass == NULL || d->pClass->moveSelectedParts == NULL) return;
    d->pClass->moveSelectedParts(d, pos, (bool)dup);
}

static unsigned char* loke_doc_render_font(LibreOfficeKitDocument *d, const char *font_name,
                                           const char *ch, int *out_w, int *out_h) {
    if (d == NULL || d->pClass == NULL || d->pClass->renderFont == NULL) return NULL;
    return d->pClass->renderFont(d, font_name, ch, out_w, out_h);
}
*/
import "C"

import "unsafe"

// OfficeGetFilterTypes calls pClass->getFilterTypes and returns the
// JSON payload as a Go string. The C buffer is freed before return.
func OfficeGetFilterTypes(h OfficeHandle) (string, error) {
	if !h.IsValid() {
		return "", ErrNilOffice
	}
	s := C.loke_office_get_filter_types(h.p)
	if s == nil {
		return "", ErrUnsupported
	}
	return copyAndFree(s), nil
}

// DocumentPaste calls pClass->paste. Returns ErrUnsupported on a
// vtable miss; LOK's own "paste failed" return also surfaces as
// ErrUnsupported because the LOK ABI does not distinguish them
// (both come back as a 0/false, conflated by design here).
func DocumentPaste(d DocumentHandle, mimeType string, data []byte) error {
	if !d.IsValid() {
		return ErrNilDocument
	}
	cMime := C.CString(mimeType)
	defer C.free(unsafe.Pointer(cMime))
	var dataPtr *C.char
	var dataLen C.int
	if len(data) > 0 {
		dataPtr = (*C.char)(unsafe.Pointer(&data[0]))
		dataLen = C.int(len(data))
	}
	rc := C.loke_doc_paste(d.p, cMime, dataPtr, dataLen)
	if rc != 1 {
		return ErrUnsupported
	}
	return nil
}

// DocumentSelectPart calls pClass->selectPart. Returns ErrNilDocument
// for the zero handle; otherwise no error channel (LOK returns void).
func DocumentSelectPart(d DocumentHandle, part int, selected bool) error {
	if !d.IsValid() {
		return ErrNilDocument
	}
	sel := C.int(0)
	if selected {
		sel = 1
	}
	C.loke_doc_select_part(d.p, C.int(part), sel)
	return nil
}

// DocumentMoveSelectedParts calls pClass->moveSelectedParts.
func DocumentMoveSelectedParts(d DocumentHandle, position int, duplicate bool) error {
	if !d.IsValid() {
		return ErrNilDocument
	}
	dup := C.int(0)
	if duplicate {
		dup = 1
	}
	C.loke_doc_move_selected_parts(d.p, C.int(position), dup)
	return nil
}

// DocumentRenderFont calls pClass->renderFont. The returned buffer is
// premultiplied BGRA of size 4*w*h; copied to Go and the LOK buffer
// freed before return.
func DocumentRenderFont(d DocumentHandle, fontName, char string) ([]byte, int, int, error) {
	if !d.IsValid() {
		return nil, 0, 0, ErrNilDocument
	}
	cFont := C.CString(fontName)
	defer C.free(unsafe.Pointer(cFont))
	cChar := C.CString(char)
	defer C.free(unsafe.Pointer(cChar))
	var w, h C.int
	ptr := C.loke_doc_render_font(d.p, cFont, cChar, &w, &h)
	if ptr == nil {
		return nil, 0, 0, ErrUnsupported
	}
	size := 4 * int(w) * int(h)
	buf := C.GoBytes(unsafe.Pointer(ptr), C.int(size))
	C.free(unsafe.Pointer(ptr))
	return buf, int(w), int(h), nil
}
