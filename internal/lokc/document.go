//go:build linux || darwin

package lokc

/*
#cgo CFLAGS: -I${SRCDIR}/../../third_party/lok -DLOK_USE_UNSTABLE_API
#include <stdlib.h>
#include "LibreOfficeKit/LibreOfficeKit.h"

static LibreOfficeKitDocument* go_document_load(LibreOfficeKit* p, const char* url) {
    if (p == NULL || p->pClass == NULL || p->pClass->documentLoad == NULL) return NULL;
    return p->pClass->documentLoad(p, url);
}

static LibreOfficeKitDocument* go_document_load_with_options(LibreOfficeKit* p, const char* url, const char* options) {
    if (p == NULL || p->pClass == NULL || p->pClass->documentLoadWithOptions == NULL) return NULL;
    return p->pClass->documentLoadWithOptions(p, url, options);
}

static int go_document_get_type(LibreOfficeKitDocument* d) {
    if (d == NULL || d->pClass == NULL || d->pClass->getDocumentType == NULL) return -1;
    return d->pClass->getDocumentType(d);
}

static int go_document_save_as(LibreOfficeKitDocument* d, const char* url, const char* format, const char* filterOptions) {
    if (d == NULL || d->pClass == NULL || d->pClass->saveAs == NULL) return 0;
    return d->pClass->saveAs(d, url, format, filterOptions);
}

static void go_document_destroy(LibreOfficeKitDocument* d) {
    if (d == NULL || d->pClass == NULL || d->pClass->destroy == NULL) return;
    d->pClass->destroy(d);
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

// ErrSaveFailed is returned when LOK's saveAs returns 0 (false).
var ErrSaveFailed = errors.New("lokc: saveAs returned failure")

// ErrNilDocument is returned by document wrappers when the supplied
// DocumentHandle is invalid.
var ErrNilDocument = errors.New("lokc: document handle is invalid")

// DocumentHandle is an opaque pointer to a LibreOfficeKitDocument*.
type DocumentHandle struct {
	p *C.struct__LibreOfficeKitDocument
}

// IsValid reports whether the handle points at a live document.
func (d DocumentHandle) IsValid() bool { return d.p != nil }

// DocumentLoad calls pClass->documentLoad with the given file:// URL.
// Returns an invalid handle if LO rejects the URL; caller should
// then read OfficeGetError for the reason.
func DocumentLoad(h OfficeHandle, url string) DocumentHandle {
	if !h.IsValid() {
		return DocumentHandle{}
	}
	curl := C.CString(url)
	defer C.free(unsafe.Pointer(curl))
	return DocumentHandle{p: C.go_document_load(h.p, curl)}
}

// DocumentLoadWithOptions forwards to pClass->documentLoadWithOptions.
func DocumentLoadWithOptions(h OfficeHandle, url, options string) DocumentHandle {
	if !h.IsValid() {
		return DocumentHandle{}
	}
	curl := C.CString(url)
	defer C.free(unsafe.Pointer(curl))
	var copts *C.char
	if options != "" {
		copts = C.CString(options)
		defer C.free(unsafe.Pointer(copts))
	}
	return DocumentHandle{p: C.go_document_load_with_options(h.p, curl, copts)}
}

// DocumentGetType returns the LOK_DOCTYPE_* integer, or -1 if the
// handle is invalid or the vtable is missing.
func DocumentGetType(d DocumentHandle) int {
	if !d.IsValid() {
		return -1
	}
	return int(C.go_document_get_type(d.p))
}

// DocumentSaveAs forwards to pClass->saveAs. Returns ErrSaveFailed
// on a zero return from LOK, ErrNilDocument on an invalid handle.
func DocumentSaveAs(d DocumentHandle, url, format, filterOptions string) error {
	if !d.IsValid() {
		return ErrNilDocument
	}
	curl := C.CString(url)
	defer C.free(unsafe.Pointer(curl))
	var cformat *C.char
	if format != "" {
		cformat = C.CString(format)
		defer C.free(unsafe.Pointer(cformat))
	}
	var cfilter *C.char
	if filterOptions != "" {
		cfilter = C.CString(filterOptions)
		defer C.free(unsafe.Pointer(cfilter))
	}
	if C.go_document_save_as(d.p, curl, cformat, cfilter) == 0 {
		return ErrSaveFailed
	}
	return nil
}

// DocumentDestroy is idempotent on a zero handle.
func DocumentDestroy(d DocumentHandle) {
	if !d.IsValid() {
		return
	}
	C.go_document_destroy(d.p)
}
