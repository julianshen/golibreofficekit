//go:build linux || darwin

package lokc

/*
#cgo CFLAGS: -I${SRCDIR}/../../third_party/lok -DLOK_USE_UNSTABLE_API
#include <stdlib.h>
#include "LibreOfficeKit/LibreOfficeKit.h"

static void go_doc_initialize_for_rendering(LibreOfficeKitDocument* d, const char* args) {
    if (d == NULL || d->pClass == NULL || d->pClass->initializeForRendering == NULL) return;
    d->pClass->initializeForRendering(d, args);
}
static int go_doc_get_tile_mode(LibreOfficeKitDocument* d) {
    if (d == NULL || d->pClass == NULL || d->pClass->getTileMode == NULL) return 0;
    return d->pClass->getTileMode(d);
}
static void go_doc_set_client_zoom(LibreOfficeKitDocument* d, int tpw, int tph, int ttw, int tth) {
    if (d == NULL || d->pClass == NULL || d->pClass->setClientZoom == NULL) return;
    d->pClass->setClientZoom(d, tpw, tph, ttw, tth);
}
static void go_doc_set_client_visible_area(LibreOfficeKitDocument* d, int x, int y, int w, int h) {
    if (d == NULL || d->pClass == NULL || d->pClass->setClientVisibleArea == NULL) return;
    d->pClass->setClientVisibleArea(d, x, y, w, h);
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
// args is LOK's JSON hint string (may be empty).
func DocumentInitializeForRendering(d DocumentHandle, args string) {
	if !d.IsValid() {
		return
	}
	cargs := C.CString(args)
	defer C.free(unsafe.Pointer(cargs))
	C.go_doc_initialize_for_rendering(d.p, cargs)
}

// DocumentGetTileMode returns LOK's tile-mode enum (0=RGBA, 1=BGRA).
// Returns 0 on unavailable handle/vtable.
func DocumentGetTileMode(d DocumentHandle) int {
	if !d.IsValid() {
		return 0
	}
	return int(C.go_doc_get_tile_mode(d.p))
}

// DocumentSetClientZoom forwards to pClass->setClientZoom.
func DocumentSetClientZoom(d DocumentHandle, tilePxW, tilePxH, tileTwipW, tileTwipH int) {
	if !d.IsValid() {
		return
	}
	C.go_doc_set_client_zoom(d.p, C.int(tilePxW), C.int(tilePxH), C.int(tileTwipW), C.int(tileTwipH))
}

// DocumentSetClientVisibleArea forwards to pClass->setClientVisibleArea.
// All coordinates are twips; LOK's C ABI takes int (32-bit) for these.
func DocumentSetClientVisibleArea(d DocumentHandle, x, y, w, h int) {
	if !d.IsValid() {
		return
	}
	C.go_doc_set_client_visible_area(d.p, C.int(x), C.int(y), C.int(w), C.int(h))
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
