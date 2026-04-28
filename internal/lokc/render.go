//go:build linux || darwin

package lokc

/*
#cgo CFLAGS: -I${SRCDIR}/../../third_party/lok -DLOK_USE_UNSTABLE_API
#include <stdlib.h>
#include "LibreOfficeKit/LibreOfficeKit.h"

// Setters return 1 on success, 0 when the vtable slot is NULL (caller
// maps to ErrUnsupported). Mirrors the selection.go pattern.
static int go_doc_initialize_for_rendering(LibreOfficeKitDocument* d, const char* args) {
    if (d == NULL || d->pClass == NULL || d->pClass->initializeForRendering == NULL) return 0;
    d->pClass->initializeForRendering(d, args);
    return 1;
}
static int go_doc_get_tile_mode(LibreOfficeKitDocument* d) {
    if (d == NULL || d->pClass == NULL || d->pClass->getTileMode == NULL) return 0;
    return d->pClass->getTileMode(d);
}
static int go_doc_set_client_zoom(LibreOfficeKitDocument* d, int tpw, int tph, int ttw, int tth) {
    if (d == NULL || d->pClass == NULL || d->pClass->setClientZoom == NULL) return 0;
    d->pClass->setClientZoom(d, tpw, tph, ttw, tth);
    return 1;
}
static int go_doc_set_client_visible_area(LibreOfficeKitDocument* d, int x, int y, int w, int h) {
    if (d == NULL || d->pClass == NULL || d->pClass->setClientVisibleArea == NULL) return 0;
    d->pClass->setClientVisibleArea(d, x, y, w, h);
    return 1;
}
static void go_doc_paint_tile(LibreOfficeKitDocument* d, unsigned char* buf,
    int canvasW, int canvasH, int posX, int posY, int tileW, int tileH) {
    if (d == NULL || d->pClass == NULL || d->pClass->paintTile == NULL) return;
    d->pClass->paintTile(d, buf, canvasW, canvasH, posX, posY, tileW, tileH);
}
static void go_doc_paint_part_tile(LibreOfficeKitDocument* d, unsigned char* buf,
    int part, int mode, int canvasW, int canvasH, int posX, int posY, int tileW, int tileH) {
    if (d == NULL || d->pClass == NULL || d->pClass->paintPartTile == NULL) return;
    d->pClass->paintPartTile(d, buf, part, mode, canvasW, canvasH, posX, posY, tileW, tileH);
}
*/
import "C"

import "unsafe"

// DocumentInitializeForRendering forwards to pClass->initializeForRendering.
// args is LOK's JSON hint string (may be empty). Returns
// ErrUnsupported on a zero handle or when the vtable slot is NULL —
// previously a NULL slot was a silent no-op which then caused
// getTileMode to return its sentinel 0, surfacing as the misleading
// "unsupported tile mode 0" error one layer up.
func DocumentInitializeForRendering(d DocumentHandle, args string) error {
	if !d.IsValid() {
		return ErrUnsupported
	}
	cargs := C.CString(args)
	defer C.free(unsafe.Pointer(cargs))
	if C.go_doc_initialize_for_rendering(d.p, cargs) == 0 {
		return ErrUnsupported
	}
	return nil
}

// DocumentGetTileMode returns LOK's tile-mode enum (0=RGBA, 1=BGRA).
// Returns 0 on unavailable handle/vtable.
func DocumentGetTileMode(d DocumentHandle) int {
	if !d.IsValid() {
		return 0
	}
	return int(C.go_doc_get_tile_mode(d.p))
}

// DocumentSetClientZoom forwards to pClass->setClientZoom. Returns
// ErrUnsupported on a zero handle or when the vtable slot is NULL.
func DocumentSetClientZoom(d DocumentHandle, tilePxW, tilePxH, tileTwipW, tileTwipH int) error {
	if !d.IsValid() {
		return ErrUnsupported
	}
	if C.go_doc_set_client_zoom(d.p, C.int(tilePxW), C.int(tilePxH), C.int(tileTwipW), C.int(tileTwipH)) == 0 {
		return ErrUnsupported
	}
	return nil
}

// DocumentSetClientVisibleArea forwards to pClass->setClientVisibleArea.
// All coordinates are twips; LOK's C ABI takes int (32-bit) for these.
// Returns ErrUnsupported on a zero handle or when the vtable slot is
// NULL.
func DocumentSetClientVisibleArea(d DocumentHandle, x, y, w, h int) error {
	if !d.IsValid() {
		return ErrUnsupported
	}
	if C.go_doc_set_client_visible_area(d.p, C.int(x), C.int(y), C.int(w), C.int(h)) == 0 {
		return ErrUnsupported
	}
	return nil
}

// DocumentPaintTile forwards to pClass->paintTile. buf must hold at
// least 4*pxW*pxH bytes — callers validate. The backing array is
// pinned for the duration of this synchronous cgo call (cgo pointer
// rule): the pointer must NOT be retained or stored by C.
func DocumentPaintTile(d DocumentHandle, buf []byte, pxW, pxH, x, y, w, h int) {
	if !d.IsValid() || len(buf) == 0 {
		return
	}
	C.go_doc_paint_tile(d.p, (*C.uchar)(unsafe.Pointer(&buf[0])),
		C.int(pxW), C.int(pxH), C.int(x), C.int(y), C.int(w), C.int(h))
}

// DocumentPaintPartTile forwards to pClass->paintPartTile. mode is
// the LOK_PARTMODE_* enum (0 = normal). Same buffer-pinning rule
// as DocumentPaintTile.
func DocumentPaintPartTile(d DocumentHandle, buf []byte, part, mode, pxW, pxH, x, y, w, h int) {
	if !d.IsValid() || len(buf) == 0 {
		return
	}
	C.go_doc_paint_part_tile(d.p, (*C.uchar)(unsafe.Pointer(&buf[0])),
		C.int(part), C.int(mode), C.int(pxW), C.int(pxH),
		C.int(x), C.int(y), C.int(w), C.int(h))
}
