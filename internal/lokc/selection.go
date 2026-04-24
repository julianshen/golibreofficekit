//go:build linux || darwin

package lokc

/*
#cgo CFLAGS: -I${SRCDIR}/../../third_party/lok -DLOK_USE_UNSTABLE_API
#include <stdlib.h>
#include "LibreOfficeKit/LibreOfficeKit.h"

static void go_doc_set_text_selection(LibreOfficeKitDocument* d, int typ, int x, int y) {
    if (d == NULL || d->pClass == NULL || d->pClass->setTextSelection == NULL) return;
    d->pClass->setTextSelection(d, typ, x, y);
}
static void go_doc_reset_selection(LibreOfficeKitDocument* d) {
    if (d == NULL || d->pClass == NULL || d->pClass->resetSelection == NULL) return;
    d->pClass->resetSelection(d);
}
static void go_doc_set_graphic_selection(LibreOfficeKitDocument* d, int typ, int x, int y) {
    if (d == NULL || d->pClass == NULL || d->pClass->setGraphicSelection == NULL) return;
    d->pClass->setGraphicSelection(d, typ, x, y);
}
static void go_doc_set_blocked_command_list(LibreOfficeKitDocument* d, int viewId, const char* csv) {
    if (d == NULL || d->pClass == NULL || d->pClass->setBlockedCommandList == NULL) return;
    d->pClass->setBlockedCommandList(d, viewId, csv);
}
static char* go_doc_get_text_selection(LibreOfficeKitDocument* d, const char* mime, char** usedMime) {
    if (d == NULL || d->pClass == NULL || d->pClass->getTextSelection == NULL) {
        if (usedMime != NULL) *usedMime = NULL;
        return NULL;
    }
    return d->pClass->getTextSelection(d, mime, usedMime);
}
*/
import "C"

import "unsafe"

// DocumentSetTextSelection forwards to pClass->setTextSelection.
// typ is LOK_SETTEXTSELECTION_START|END|RESET; x, y are twips.
func DocumentSetTextSelection(d DocumentHandle, typ, x, y int) {
	if !d.IsValid() {
		return
	}
	C.go_doc_set_text_selection(d.p, C.int(typ), C.int(x), C.int(y))
}

// DocumentResetSelection forwards to pClass->resetSelection.
func DocumentResetSelection(d DocumentHandle) {
	if !d.IsValid() {
		return
	}
	C.go_doc_reset_selection(d.p)
}

// DocumentSetGraphicSelection forwards to pClass->setGraphicSelection.
// typ is LOK_SETGRAPHICSELECTION_START|END; x, y are twips.
func DocumentSetGraphicSelection(d DocumentHandle, typ, x, y int) {
	if !d.IsValid() {
		return
	}
	C.go_doc_set_graphic_selection(d.p, C.int(typ), C.int(x), C.int(y))
}

// DocumentSetBlockedCommandList forwards to
// pClass->setBlockedCommandList. csv is a comma-separated list of
// .uno:* command names.
func DocumentSetBlockedCommandList(d DocumentHandle, viewID int, csv string) {
	if !d.IsValid() {
		return
	}
	ccsv := C.CString(csv)
	defer C.free(unsafe.Pointer(ccsv))
	C.go_doc_set_blocked_command_list(d.p, C.int(viewID), ccsv)
}

// DocumentGetTextSelection copies the current text selection as the
// requested mime type. Returns (text, usedMime). Both strings are
// empty when LOK has nothing to return or the vtable slot is NULL.
func DocumentGetTextSelection(d DocumentHandle, mimeType string) (string, string) {
	if !d.IsValid() {
		return "", ""
	}
	cmime := C.CString(mimeType)
	defer C.free(unsafe.Pointer(cmime))
	var usedMime *C.char
	text := C.go_doc_get_text_selection(d.p, cmime, &usedMime)
	return copyAndFree(text), copyAndFree(usedMime)
}
