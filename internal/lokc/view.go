//go:build linux || darwin

package lokc

/*
#cgo CFLAGS: -I${SRCDIR}/../../third_party/lok -DLOK_USE_UNSTABLE_API
#include <stdbool.h>
#include <stdlib.h>
#include "LibreOfficeKit/LibreOfficeKit.h"

static int go_doc_create_view(LibreOfficeKitDocument* d) {
    if (d == NULL || d->pClass == NULL || d->pClass->createView == NULL) return -1;
    return d->pClass->createView(d);
}
static int go_doc_create_view_with_options(LibreOfficeKitDocument* d, const char* opts) {
    if (d == NULL || d->pClass == NULL || d->pClass->createViewWithOptions == NULL) return -1;
    return d->pClass->createViewWithOptions(d, opts);
}
static void go_doc_destroy_view(LibreOfficeKitDocument* d, int id) {
    if (d == NULL || d->pClass == NULL || d->pClass->destroyView == NULL) return;
    d->pClass->destroyView(d, id);
}
static void go_doc_set_view(LibreOfficeKitDocument* d, int id) {
    if (d == NULL || d->pClass == NULL || d->pClass->setView == NULL) return;
    d->pClass->setView(d, id);
}
static int go_doc_get_view(LibreOfficeKitDocument* d) {
    if (d == NULL || d->pClass == NULL || d->pClass->getView == NULL) return -1;
    return d->pClass->getView(d);
}
static int go_doc_get_views_count(LibreOfficeKitDocument* d) {
    // -1 signals "vtable unavailable" so DocumentGetViewIds can
    // distinguish it from the legitimate zero-views case (0).
    if (d == NULL || d->pClass == NULL || d->pClass->getViewsCount == NULL) return -1;
    return d->pClass->getViewsCount(d);
}
static bool go_doc_get_view_ids(LibreOfficeKitDocument* d, int* buf, size_t n) {
    if (d == NULL || d->pClass == NULL || d->pClass->getViewIds == NULL) return false;
    return d->pClass->getViewIds(d, buf, n);
}
static void go_doc_set_view_language(LibreOfficeKitDocument* d, int id, const char* lang) {
    if (d == NULL || d->pClass == NULL || d->pClass->setViewLanguage == NULL) return;
    d->pClass->setViewLanguage(d, id, lang);
}
static void go_doc_set_view_read_only(LibreOfficeKitDocument* d, int id, bool ro) {
    if (d == NULL || d->pClass == NULL || d->pClass->setViewReadOnly == NULL) return;
    d->pClass->setViewReadOnly(d, id, ro);
}
static void go_doc_set_accessibility_state(LibreOfficeKitDocument* d, int id, bool en) {
    if (d == NULL || d->pClass == NULL || d->pClass->setAccessibilityState == NULL) return;
    d->pClass->setAccessibilityState(d, id, en);
}
static void go_doc_set_view_timezone(LibreOfficeKitDocument* d, int id, const char* tz) {
    if (d == NULL || d->pClass == NULL || d->pClass->setViewTimezone == NULL) return;
    d->pClass->setViewTimezone(d, id, tz);
}
*/
import "C"

import "unsafe"

// DocumentCreateView returns the new view ID, or -1 if the document
// is invalid / the vtable entry is missing.
func DocumentCreateView(d DocumentHandle) int {
	if !d.IsValid() {
		return -1
	}
	return int(C.go_doc_create_view(d.p))
}

// DocumentCreateViewWithOptions forwards the raw options string. An
// empty string is passed through as a zero-length C string (not
// NULL) because LO's NULL-tolerance is undocumented for this entry.
func DocumentCreateViewWithOptions(d DocumentHandle, options string) int {
	if !d.IsValid() {
		return -1
	}
	copts := C.CString(options)
	defer C.free(unsafe.Pointer(copts))
	return int(C.go_doc_create_view_with_options(d.p, copts))
}

// DocumentDestroyView is idempotent on a zero handle / missing vtable.
func DocumentDestroyView(d DocumentHandle, id int) {
	if !d.IsValid() {
		return
	}
	C.go_doc_destroy_view(d.p, C.int(id))
}

// DocumentSetView activates the given view on the document.
func DocumentSetView(d DocumentHandle, id int) {
	if !d.IsValid() {
		return
	}
	C.go_doc_set_view(d.p, C.int(id))
}

// DocumentGetView returns the active view ID, or -1.
func DocumentGetView(d DocumentHandle) int {
	if !d.IsValid() {
		return -1
	}
	return int(C.go_doc_get_view(d.p))
}

// DocumentGetViewsCount returns the number of live views. A negative
// value signals unavailable functionality (nil handle, missing
// vtable). 0 means zero live views, not an error. Use the sign to
// distinguish.
func DocumentGetViewsCount(d DocumentHandle) int {
	if !d.IsValid() {
		return -1
	}
	return int(C.go_doc_get_views_count(d.p))
}

// DocumentGetViewIds returns (ids, true) on success, (nil, true)
// when there are legitimately zero live views, and (nil, false)
// when the backend call itself failed (invalid handle, missing
// vtable entry, or getViewIds returned false).
//
// NOT thread-safe: it performs two sequential cgo calls
// (getViewsCount then getViewIds). Callers must serialise
// externally; the public lok.Views() wrapper holds the Office
// mutex across both to prevent a racing CreateView/DestroyView
// from resizing LOK's internal view list mid-call.
func DocumentGetViewIds(d DocumentHandle) ([]int, bool) {
	if !d.IsValid() {
		return nil, false
	}
	n := int(C.go_doc_get_views_count(d.p))
	if n < 0 {
		return nil, false
	}
	if n == 0 {
		return nil, true
	}
	buf := make([]C.int, n)
	if !bool(C.go_doc_get_view_ids(d.p, (*C.int)(unsafe.Pointer(&buf[0])), C.size_t(n))) {
		return nil, false
	}
	out := make([]int, n)
	for i, v := range buf {
		out[i] = int(v)
	}
	return out, true
}

// DocumentSetViewLanguage forwards to pClass->setViewLanguage.
func DocumentSetViewLanguage(d DocumentHandle, id int, lang string) {
	if !d.IsValid() {
		return
	}
	c := C.CString(lang)
	defer C.free(unsafe.Pointer(c))
	C.go_doc_set_view_language(d.p, C.int(id), c)
}

// DocumentSetViewReadOnly forwards to pClass->setViewReadOnly.
func DocumentSetViewReadOnly(d DocumentHandle, id int, readOnly bool) {
	if !d.IsValid() {
		return
	}
	C.go_doc_set_view_read_only(d.p, C.int(id), C.bool(readOnly))
}

// DocumentSetAccessibilityState forwards to pClass->setAccessibilityState.
func DocumentSetAccessibilityState(d DocumentHandle, id int, enabled bool) {
	if !d.IsValid() {
		return
	}
	C.go_doc_set_accessibility_state(d.p, C.int(id), C.bool(enabled))
}

// DocumentSetViewTimezone forwards to pClass->setViewTimezone.
func DocumentSetViewTimezone(d DocumentHandle, id int, tz string) {
	if !d.IsValid() {
		return
	}
	c := C.CString(tz)
	defer C.free(unsafe.Pointer(c))
	C.go_doc_set_view_timezone(d.p, C.int(id), c)
}
