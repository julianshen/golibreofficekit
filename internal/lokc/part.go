//go:build linux || darwin

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
static void go_doc_set_part(LibreOfficeKitDocument* d, int n) {
    if (d == NULL || d->pClass == NULL || d->pClass->setPart == NULL) return;
    d->pClass->setPart(d, n);
}
static void go_doc_set_part_mode(LibreOfficeKitDocument* d, int mode) {
    if (d == NULL || d->pClass == NULL || d->pClass->setPartMode == NULL) return;
    d->pClass->setPartMode(d, mode);
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
static void go_doc_set_outline_state(LibreOfficeKitDocument* d, bool col, int level, int idx, bool hidden) {
    if (d == NULL || d->pClass == NULL || d->pClass->setOutlineState == NULL) return;
    d->pClass->setOutlineState(d, col, level, idx, hidden);
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

// DocumentSetPart forwards to pClass->setPart.
func DocumentSetPart(d DocumentHandle, n int) {
	if !d.IsValid() {
		return
	}
	C.go_doc_set_part(d.p, C.int(n))
}

// DocumentSetPartMode forwards to pClass->setPartMode. The mode
// enum values live in LibreOfficeKitEnums.h (LOK_PARTMODE_*).
func DocumentSetPartMode(d DocumentHandle, mode int) {
	if !d.IsValid() {
		return
	}
	C.go_doc_set_part_mode(d.p, C.int(mode))
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
func DocumentSetOutlineState(d DocumentHandle, column bool, level, index int, hidden bool) {
	if !d.IsValid() {
		return
	}
	C.go_doc_set_outline_state(d.p, C.bool(column), C.int(level), C.int(index), C.bool(hidden))
}
