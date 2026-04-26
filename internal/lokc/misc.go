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

// LOK's selectPart / moveSelectedParts are void; the shim returns 1 on
// success and 0 if the vtable slot is missing so the Go layer can
// surface ErrUnsupported instead of silently no-opping on old LO.
static int loke_doc_select_part(LibreOfficeKitDocument *d, int part, int sel) {
    if (d == NULL || d->pClass == NULL || d->pClass->selectPart == NULL) return 0;
    d->pClass->selectPart(d, part, sel);
    return 1;
}

static int loke_doc_move_selected_parts(LibreOfficeKitDocument *d, int pos, int dup) {
    if (d == NULL || d->pClass == NULL || d->pClass->moveSelectedParts == NULL) return 0;
    d->pClass->moveSelectedParts(d, pos, (bool)dup);
    return 1;
}

static unsigned char* loke_doc_render_font(LibreOfficeKitDocument *d, const char *font_name,
                                           const char *ch, int *out_w, int *out_h) {
    if (d == NULL || d->pClass == NULL || d->pClass->renderFont == NULL) return NULL;
    return d->pClass->renderFont(d, font_name, ch, out_w, out_h);
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

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

// ErrPasteFailed is returned by DocumentPaste when LOK's paste returned
// false (e.g. unsupported MIME type for the document type, or LO
// rejected the payload). Distinct from ErrUnsupported, which means the
// vtable slot itself is missing on this LO build.
var ErrPasteFailed = errors.New("lokc: paste returned false")

// DocumentPaste calls pClass->paste. The C shim distinguishes
// "vtable slot missing" (-1 → ErrUnsupported) from "LO rejected
// the payload" (0 → ErrPasteFailed); LOK's bool return alone could
// not, but the shim widens the channel.
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
	switch rc {
	case -1:
		return ErrUnsupported
	case 0:
		return ErrPasteFailed
	}
	return nil
}

// DocumentSelectPart calls pClass->selectPart. Returns ErrUnsupported
// when the vtable slot is missing so callers on older LO builds
// observe the no-op rather than silently moving on.
func DocumentSelectPart(d DocumentHandle, part int, selected bool) error {
	if !d.IsValid() {
		return ErrNilDocument
	}
	sel := C.int(0)
	if selected {
		sel = 1
	}
	if rc := C.loke_doc_select_part(d.p, C.int(part), sel); rc == 0 {
		return ErrUnsupported
	}
	return nil
}

// DocumentMoveSelectedParts calls pClass->moveSelectedParts. Returns
// ErrUnsupported when the vtable slot is missing.
func DocumentMoveSelectedParts(d DocumentHandle, position int, duplicate bool) error {
	if !d.IsValid() {
		return ErrNilDocument
	}
	dup := C.int(0)
	if duplicate {
		dup = 1
	}
	if rc := C.loke_doc_move_selected_parts(d.p, C.int(position), dup); rc == 0 {
		return ErrUnsupported
	}
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
