//go:build linux || darwin

package lokc

/*
#cgo CFLAGS: -I${SRCDIR}/../../third_party/lok -DLOK_USE_UNSTABLE_API
#include <stdlib.h>
#include "LibreOfficeKit/LibreOfficeKit.h"

// Setters return 1 on success, 0 when the vtable slot is NULL
// (caller maps to ErrUnsupported). Phase 8 surfaced the absence
// instead of silently swallowing it because real-LO builds may
// expose only a subset of these slots in future ABIs.
static int go_doc_set_text_selection(LibreOfficeKitDocument* d, int typ, int x, int y) {
    if (d == NULL || d->pClass == NULL || d->pClass->setTextSelection == NULL) return 0;
    d->pClass->setTextSelection(d, typ, x, y);
    return 1;
}
static int go_doc_reset_selection(LibreOfficeKitDocument* d) {
    if (d == NULL || d->pClass == NULL || d->pClass->resetSelection == NULL) return 0;
    d->pClass->resetSelection(d);
    return 1;
}
static int go_doc_set_graphic_selection(LibreOfficeKitDocument* d, int typ, int x, int y) {
    if (d == NULL || d->pClass == NULL || d->pClass->setGraphicSelection == NULL) return 0;
    d->pClass->setGraphicSelection(d, typ, x, y);
    return 1;
}
static int go_doc_set_blocked_command_list(LibreOfficeKitDocument* d, int viewId, const char* csv) {
    if (d == NULL || d->pClass == NULL || d->pClass->setBlockedCommandList == NULL) return 0;
    d->pClass->setBlockedCommandList(d, viewId, csv);
    return 1;
}
// go_doc_get_text_selection writes 1 to *ok on success or 0 when the
// vtable slot is NULL. The text return is independent: it may be NULL
// even on success when nothing is selected.
static char* go_doc_get_text_selection(LibreOfficeKitDocument* d, const char* mime, char** usedMime, int* ok) {
    if (d == NULL || d->pClass == NULL || d->pClass->getTextSelection == NULL) {
        if (usedMime != NULL) *usedMime = NULL;
        *ok = 0;
        return NULL;
    }
    *ok = 1;
    return d->pClass->getTextSelection(d, mime, usedMime);
}

// go_doc_get_selection_type writes 1 to *ok on success or 0 when the
// vtable slot is NULL.
static int go_doc_get_selection_type(LibreOfficeKitDocument* d, int* ok) {
    if (d == NULL || d->pClass == NULL || d->pClass->getSelectionType == NULL) {
        *ok = 0;
        return 0;
    }
    *ok = 1;
    return d->pClass->getSelectionType(d);
}

// Returns:
//   0 when the slot is NULL (unsupported)
//   1 when the call was made; *outKind, *outText, *outMime are populated
static int go_doc_get_selection_type_and_text(LibreOfficeKitDocument* d,
                                              const char* mime,
                                              int* outKind,
                                              char** outText,
                                              char** outMime) {
    if (d == NULL || d->pClass == NULL || d->pClass->getSelectionTypeAndText == NULL) {
        *outKind = -1;
        *outText = NULL;
        *outMime = NULL;
        return 0;
    }
    *outKind = d->pClass->getSelectionTypeAndText(d, mime, outText, outMime);
    return 1;
}
*/
import "C"

import "unsafe"

// DocumentSetTextSelection forwards to pClass->setTextSelection.
// typ is LOK_SETTEXTSELECTION_START|END|RESET; x, y are twips.
// Returns ErrUnsupported when the vtable slot is NULL.
func DocumentSetTextSelection(d DocumentHandle, typ, x, y int) error {
	if !d.IsValid() {
		return ErrUnsupported
	}
	if C.go_doc_set_text_selection(d.p, C.int(typ), C.int(x), C.int(y)) == 0 {
		return ErrUnsupported
	}
	return nil
}

// DocumentResetSelection forwards to pClass->resetSelection.
// Returns ErrUnsupported when the vtable slot is NULL.
func DocumentResetSelection(d DocumentHandle) error {
	if !d.IsValid() {
		return ErrUnsupported
	}
	if C.go_doc_reset_selection(d.p) == 0 {
		return ErrUnsupported
	}
	return nil
}

// DocumentSetGraphicSelection forwards to pClass->setGraphicSelection.
// typ is LOK_SETGRAPHICSELECTION_START|END; x, y are twips.
// Returns ErrUnsupported when the vtable slot is NULL.
func DocumentSetGraphicSelection(d DocumentHandle, typ, x, y int) error {
	if !d.IsValid() {
		return ErrUnsupported
	}
	if C.go_doc_set_graphic_selection(d.p, C.int(typ), C.int(x), C.int(y)) == 0 {
		return ErrUnsupported
	}
	return nil
}

// DocumentSetBlockedCommandList forwards to
// pClass->setBlockedCommandList. csv is a comma-separated list of
// .uno:* command names. Returns ErrUnsupported when the vtable slot
// is NULL.
func DocumentSetBlockedCommandList(d DocumentHandle, viewID int, csv string) error {
	if !d.IsValid() {
		return ErrUnsupported
	}
	ccsv := C.CString(csv)
	defer C.free(unsafe.Pointer(ccsv))
	if C.go_doc_set_blocked_command_list(d.p, C.int(viewID), ccsv) == 0 {
		return ErrUnsupported
	}
	return nil
}

// DocumentGetTextSelection copies the current text selection as the
// requested mime type. Returns ErrUnsupported when the vtable slot is
// NULL; otherwise (text, usedMime, nil). Either text or usedMime may
// be empty even on success when LOK has nothing to return — the
// error return distinguishes "no selection" (nil) from "unsupported
// build" (ErrUnsupported).
func DocumentGetTextSelection(d DocumentHandle, mimeType string) (text, usedMime string, err error) {
	if !d.IsValid() {
		return "", "", ErrUnsupported
	}
	cmime := C.CString(mimeType)
	defer C.free(unsafe.Pointer(cmime))
	var cMime *C.char
	var ok C.int
	cText := C.go_doc_get_text_selection(d.p, cmime, &cMime, &ok)
	if ok == 0 {
		return "", "", ErrUnsupported
	}
	return copyAndFree(cText), copyAndFree(cMime), nil
}

// DocumentGetSelectionType returns the LOK_SELTYPE_* value for the
// current selection. Returns ErrUnsupported when the handle or
// vtable slot is NULL.
func DocumentGetSelectionType(d DocumentHandle) (int, error) {
	if !d.IsValid() {
		return 0, ErrUnsupported
	}
	var ok C.int
	v := C.go_doc_get_selection_type(d.p, &ok)
	if ok == 0 {
		return 0, ErrUnsupported
	}
	return int(v), nil
}

// DocumentGetSelectionTypeAndText reads both the selection kind and
// the selected text in one LOK call (LO 7.4+). Returns ErrUnsupported
// when the pClass slot is NULL.
func DocumentGetSelectionTypeAndText(d DocumentHandle, mimeType string) (kind int, text, usedMime string, err error) {
	if !d.IsValid() {
		return -1, "", "", ErrUnsupported
	}
	cmime := C.CString(mimeType)
	defer C.free(unsafe.Pointer(cmime))
	var ck C.int
	var cText, cMime *C.char
	ok := C.go_doc_get_selection_type_and_text(d.p, cmime, &ck, &cText, &cMime)
	if ok == 0 {
		return -1, "", "", ErrUnsupported
	}
	return int(ck), copyAndFree(cText), copyAndFree(cMime), nil
}
