//go:build (linux && (amd64 || arm64)) || (darwin && (amd64 || arm64))

package lokc

/*
#cgo CFLAGS: -I${SRCDIR}/../../third_party/lok -DLOK_USE_UNSTABLE_API
#include <stdbool.h>
#include <stdlib.h>
#include "LibreOfficeKit/LibreOfficeKit.h"

static int go_doc_get_parts(LibreOfficeKitDocument* d) {
    if (d == NULL || d->pClass == NULL || d->pClass->getParts == NULL) return -1;
    return d->pClass->getParts(d);
}
static int go_doc_get_part(LibreOfficeKitDocument* d) {
    if (d == NULL || d->pClass == NULL || d->pClass->getPart == NULL) return -1;
    return d->pClass->getPart(d);
}
// Setters return 1 on success, 0 when the vtable slot is NULL (caller
// maps to ErrUnsupported). Mirrors the selection.go pattern.
static int go_doc_set_part(LibreOfficeKitDocument* d, int n) {
    if (d == NULL || d->pClass == NULL || d->pClass->setPart == NULL) return 0;
    d->pClass->setPart(d, n);
    return 1;
}
static int go_doc_set_part_mode(LibreOfficeKitDocument* d, int mode) {
    if (d == NULL || d->pClass == NULL || d->pClass->setPartMode == NULL) return 0;
    d->pClass->setPartMode(d, mode);
    return 1;
}
static char* go_doc_get_part_name(LibreOfficeKitDocument* d, int n) {
    if (d == NULL || d->pClass == NULL || d->pClass->getPartName == NULL) return NULL;
    return d->pClass->getPartName(d, n);
}
static char* go_doc_get_part_hash(LibreOfficeKitDocument* d, int n) {
    if (d == NULL || d->pClass == NULL || d->pClass->getPartHash == NULL) return NULL;
    return d->pClass->getPartHash(d, n);
}
static char* go_doc_get_part_info(LibreOfficeKitDocument* d, int n) {
    if (d == NULL || d->pClass == NULL || d->pClass->getPartInfo == NULL) return NULL;
    return d->pClass->getPartInfo(d, n);
}
static char* go_doc_get_part_page_rects(LibreOfficeKitDocument* d) {
    if (d == NULL || d->pClass == NULL || d->pClass->getPartPageRectangles == NULL) return NULL;
    return d->pClass->getPartPageRectangles(d);
}
static void go_doc_get_document_size(LibreOfficeKitDocument* d, long* w, long* h) {
    *w = 0; *h = 0;
    if (d == NULL || d->pClass == NULL || d->pClass->getDocumentSize == NULL) return;
    d->pClass->getDocumentSize(d, w, h);
}
static int go_doc_set_outline_state(LibreOfficeKitDocument* d, bool col, int level, int idx, bool hidden) {
    if (d == NULL || d->pClass == NULL || d->pClass->setOutlineState == NULL) return 0;
    d->pClass->setOutlineState(d, col, level, idx, hidden);
    return 1;
}
*/
import "C"

// DocumentGetParts returns the number of parts (sheets/pages/slides),
// or -1 on unavailable handle/vtable.
func DocumentGetParts(d DocumentHandle) int {
	if !d.IsValid() {
		return -1
	}
	return int(C.go_doc_get_parts(d.p))
}

// DocumentGetPart returns the currently-active part index, or -1.
func DocumentGetPart(d DocumentHandle) int {
	if !d.IsValid() {
		return -1
	}
	return int(C.go_doc_get_part(d.p))
}

// DocumentSetPart forwards to pClass->setPart. Returns ErrUnsupported
// on a zero handle or when the vtable slot is NULL.
func DocumentSetPart(d DocumentHandle, n int) error {
	if !d.IsValid() {
		return ErrUnsupported
	}
	if C.go_doc_set_part(d.p, C.int(n)) == 0 {
		return ErrUnsupported
	}
	return nil
}

// DocumentSetPartMode forwards to pClass->setPartMode. The mode
// enum values live in LibreOfficeKitEnums.h (LOK_PARTMODE_*).
// Returns ErrUnsupported on a zero handle or when the vtable slot
// is NULL.
func DocumentSetPartMode(d DocumentHandle, mode int) error {
	if !d.IsValid() {
		return ErrUnsupported
	}
	if C.go_doc_set_part_mode(d.p, C.int(mode)) == 0 {
		return ErrUnsupported
	}
	return nil
}

// DocumentGetPartName returns the part's display name. Empty string
// on error or missing vtable.
func DocumentGetPartName(d DocumentHandle, n int) string {
	if !d.IsValid() {
		return ""
	}
	return copyAndFree(C.go_doc_get_part_name(d.p, C.int(n)))
}

// DocumentGetPartHash returns the part's stable hash string.
func DocumentGetPartHash(d DocumentHandle, n int) string {
	if !d.IsValid() {
		return ""
	}
	return copyAndFree(C.go_doc_get_part_hash(d.p, C.int(n)))
}

// DocumentGetPartInfo returns the LOK-allocated JSON blob for a part.
func DocumentGetPartInfo(d DocumentHandle, n int) string {
	if !d.IsValid() {
		return ""
	}
	return copyAndFree(C.go_doc_get_part_info(d.p, C.int(n)))
}

// DocumentGetPartPageRectangles returns LOK's semicolon-separated
// "x, y, w, h; …" rectangle string. Caller parses.
func DocumentGetPartPageRectangles(d DocumentHandle) string {
	if !d.IsValid() {
		return ""
	}
	return copyAndFree(C.go_doc_get_part_page_rects(d.p))
}

// DocumentGetDocumentSize returns (width, height) in twips. Both
// zero if unavailable. Assumes LP64 (Linux amd64, macOS arm64,
// macOS amd64) — `long` is 64-bit on all supported platforms, so
// int64(C.long) is lossless. 32-bit platforms are unsupported per
// the spec.
func DocumentGetDocumentSize(d DocumentHandle) (int64, int64) {
	if !d.IsValid() {
		return 0, 0
	}
	var w, h C.long
	C.go_doc_get_document_size(d.p, &w, &h)
	return int64(w), int64(h)
}

// DocumentSetOutlineState forwards to pClass->setOutlineState.
// Returns ErrUnsupported on a zero handle or when the vtable slot
// is NULL.
func DocumentSetOutlineState(d DocumentHandle, column bool, level, index int, hidden bool) error {
	if !d.IsValid() {
		return ErrUnsupported
	}
	if C.go_doc_set_outline_state(d.p, C.bool(column), C.int(level), C.int(index), C.bool(hidden)) == 0 {
		return ErrUnsupported
	}
	return nil
}
