//go:build linux || darwin

package lokc

/*
#cgo CFLAGS: -I${SRCDIR}/../../third_party/lok -DLOK_USE_UNSTABLE_API
#include <stdlib.h>
#include <string.h>
#include "LibreOfficeKit/LibreOfficeKit.h"

// go_doc_get_clipboard calls pClass->getClipboard and passes the
// returned triple arrays back by pointer. Returns:
//   -1 when the vtable slot is NULL (unsupported)
//    0 when LOK reported failure
//    1 on success; *pCount / *pMimes / *pSizes / *pStreams populated
//      and owned by the caller (must be freed with go_doc_free_clipboard).
static int go_doc_get_clipboard(LibreOfficeKitDocument* d,
                                const char** inMimes,
                                size_t* pCount,
                                char*** pMimes,
                                size_t** pSizes,
                                char*** pStreams) {
    *pCount   = 0;
    *pMimes   = NULL;
    *pSizes   = NULL;
    *pStreams = NULL;
    if (d == NULL || d->pClass == NULL || d->pClass->getClipboard == NULL) return -1;
    int ok = d->pClass->getClipboard(d, inMimes, pCount, pMimes, pSizes, pStreams);
    return ok ? 1 : 0;
}

// go_doc_free_clipboard releases the triple arrays returned by
// go_doc_get_clipboard. Safe on NULL inputs and on zero count.
static void go_doc_free_clipboard(size_t count, char** mimes, size_t* sizes, char** streams) {
    if (mimes != NULL) {
        for (size_t i = 0; i < count; ++i) free(mimes[i]);
        free(mimes);
    }
    if (sizes != NULL) free(sizes);
    if (streams != NULL) {
        for (size_t i = 0; i < count; ++i) free(streams[i]);
        free(streams);
    }
}

// Accessors — cgo cannot index char** / size_t* from Go directly.
static char*  go_doc_clipboard_mime(char** mimes, size_t i)     { return mimes[i]; }
static size_t go_doc_clipboard_size(size_t* sizes, size_t i)    { return sizes[i]; }
static char*  go_doc_clipboard_stream(char** streams, size_t i) { return streams[i]; }

// go_doc_set_clipboard mirrors go_doc_get_clipboard's return contract:
//   -1 when the vtable slot is NULL (unsupported)
//    0 when LOK reported failure
//    1 on success
static int go_doc_set_clipboard(LibreOfficeKitDocument* d,
                                size_t count,
                                const char** mimes,
                                const size_t* sizes,
                                const char** streams) {
    if (d == NULL || d->pClass == NULL || d->pClass->setClipboard == NULL) return -1;
    int ok = d->pClass->setClipboard(d, count, mimes, sizes, streams);
    return ok ? 1 : 0;
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

// ClipboardItem is the in-package representation of one per-view
// clipboard entry returned by DocumentGetClipboard. Data is nil when
// LOK had no payload for the corresponding mime.
type ClipboardItem struct {
	MimeType string
	Data     []byte
}

// DocumentGetClipboard invokes pClass->getClipboard. A nil mimeTypes
// slice is forwarded as C NULL (all natively-offered types); an
// empty slice is also forwarded as NULL (LOK treats a zero-length
// NULL-terminated list identically). Returns ErrUnsupported when the
// vtable slot is NULL.
func DocumentGetClipboard(d DocumentHandle, mimeTypes []string) ([]ClipboardItem, error) {
	if !d.IsValid() {
		return nil, ErrUnsupported
	}

	// Build a NULL-terminated **char or nil.
	var inMimes **C.char
	if len(mimeTypes) > 0 {
		carr := C.malloc(C.size_t(len(mimeTypes)+1) * C.size_t(unsafe.Sizeof(uintptr(0))))
		defer C.free(carr)
		slice := (*[1 << 20]*C.char)(carr)[: len(mimeTypes)+1 : len(mimeTypes)+1]
		for i, m := range mimeTypes {
			slice[i] = C.CString(m)
			defer C.free(unsafe.Pointer(slice[i]))
		}
		slice[len(mimeTypes)] = nil
		inMimes = (**C.char)(carr)
	}

	var count C.size_t
	var outMimes, outStreams **C.char
	var outSizes *C.size_t
	ok := C.go_doc_get_clipboard(d.p, inMimes, &count, &outMimes, &outSizes, &outStreams)
	switch ok {
	case -1:
		return nil, ErrUnsupported
	case 0:
		// LOK reported failure; clean up any partial allocation.
		C.go_doc_free_clipboard(count, outMimes, outSizes, outStreams)
		return nil, errors.New("lokc: getClipboard returned failure")
	}
	defer C.go_doc_free_clipboard(count, outMimes, outSizes, outStreams)

	n := int(count)
	items := make([]ClipboardItem, n)
	for i := range n {
		cmime := C.go_doc_clipboard_mime(outMimes, C.size_t(i))
		sz := C.go_doc_clipboard_size(outSizes, C.size_t(i))
		cstream := C.go_doc_clipboard_stream(outStreams, C.size_t(i))
		items[i].MimeType = C.GoString(cmime)
		if cstream != nil {
			items[i].Data = C.GoBytes(unsafe.Pointer(cstream), C.int(sz))
		}
	}
	return items, nil
}

// DocumentSetClipboard invokes pClass->setClipboard. An empty items
// slice is forwarded as count=0 (LOK accepts this; the platform
// convention is that callers who want to clear the clipboard use
// ResetSelection or the .uno:Clear command). Returns ErrUnsupported
// when the vtable slot is NULL.
func DocumentSetClipboard(d DocumentHandle, items []ClipboardItem) error {
	if !d.IsValid() {
		return ErrUnsupported
	}

	n := len(items)
	var (
		mimesPtr   unsafe.Pointer
		sizesPtr   unsafe.Pointer
		streamsPtr unsafe.Pointer
		cMimes     **C.char
		cSizes     *C.size_t
		cStreams   **C.char
	)
	// Build three parallel C-heap arrays; skip malloc on empty input.
	if n > 0 {
		mimesPtr = C.malloc(C.size_t(n) * C.size_t(unsafe.Sizeof(uintptr(0))))
		sizesPtr = C.malloc(C.size_t(n) * C.size_t(unsafe.Sizeof(C.size_t(0))))
		streamsPtr = C.malloc(C.size_t(n) * C.size_t(unsafe.Sizeof(uintptr(0))))
		defer C.free(mimesPtr)
		defer C.free(sizesPtr)
		defer C.free(streamsPtr)
		cMimes = (**C.char)(mimesPtr)
		cSizes = (*C.size_t)(sizesPtr)
		cStreams = (**C.char)(streamsPtr)

		mimesSlice := (*[1 << 20]*C.char)(mimesPtr)[:n:n]
		sizesSlice := (*[1 << 20]C.size_t)(sizesPtr)[:n:n]
		streamsSlice := (*[1 << 20]*C.char)(streamsPtr)[:n:n]

		for i, it := range items {
			mimesSlice[i] = C.CString(it.MimeType)
			defer C.free(unsafe.Pointer(mimesSlice[i]))
			sizesSlice[i] = C.size_t(len(it.Data))
			if len(it.Data) == 0 {
				streamsSlice[i] = nil
			} else {
				streamsSlice[i] = (*C.char)(C.CBytes(it.Data))
				defer C.free(unsafe.Pointer(streamsSlice[i]))
			}
		}
	}

	ok := C.go_doc_set_clipboard(d.p, C.size_t(n), cMimes, cSizes, cStreams)
	switch ok {
	case -1:
		return ErrUnsupported
	case 0:
		return errors.New("lokc: setClipboard returned failure")
	}
	return nil
}
