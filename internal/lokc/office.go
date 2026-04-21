//go:build linux || darwin

package lokc

/*
#cgo CFLAGS: -I${SRCDIR}/../../third_party/lok -DLOK_USE_UNSTABLE_API
#include <stdlib.h>
#include "LibreOfficeKit/LibreOfficeKit.h"

typedef LibreOfficeKit* (*lok_hook_fn)(const char *install_path);
typedef LibreOfficeKit* (*lok_hook_2_fn)(const char *install_path, const char *user_profile_url);

static LibreOfficeKit* go_invoke_hook(void *hook_ptr, int version,
                                      const char *install_path,
                                      const char *user_profile_url) {
    if (hook_ptr == NULL) return NULL;
    if (version == 2) {
        return ((lok_hook_2_fn)hook_ptr)(install_path, user_profile_url);
    }
    return ((lok_hook_fn)hook_ptr)(install_path);
}

static char* go_office_getError(LibreOfficeKit *p) {
    if (p == NULL || p->pClass == NULL || p->pClass->getError == NULL) return NULL;
    return p->pClass->getError(p);
}

static char* go_office_getVersionInfo(LibreOfficeKit *p) {
    if (p == NULL || p->pClass == NULL || p->pClass->getVersionInfo == NULL) return NULL;
    return p->pClass->getVersionInfo(p);
}

static void go_office_setOptionalFeatures(LibreOfficeKit *p, unsigned long long features) {
    if (p == NULL || p->pClass == NULL || p->pClass->setOptionalFeatures == NULL) return;
    p->pClass->setOptionalFeatures(p, features);
}

static void go_office_setDocumentPassword(LibreOfficeKit *p, const char *url, const char *password) {
    if (p == NULL || p->pClass == NULL || p->pClass->setDocumentPassword == NULL) return;
    p->pClass->setDocumentPassword(p, url, password);
}

// setAuthor is NOT a direct vtable entry in LOK 24.8's LibreOfficeKit.h —
// we route it through setOption("Author", value), which IS in the header.
static void go_office_setAuthor(LibreOfficeKit *p, const char *author) {
    if (p == NULL || p->pClass == NULL || p->pClass->setOption == NULL) return;
    p->pClass->setOption(p, "Author", author);
}

// dumpState: LO allocates *pState via strdup/malloc and transfers
// ownership to the caller. We pass the pointer straight to copyAndFree,
// which calls C.free (matching LO's malloc-family allocation). Do not
// free with LOK's freeError — that is for error-string ownership only.
static char* go_office_dumpState(LibreOfficeKit *p) {
    if (p == NULL || p->pClass == NULL || p->pClass->dumpState == NULL) return NULL;
    // signature: void dumpState(LibreOfficeKit* pThis, const char* pOptions, char** pState);
    char *state = NULL;
    p->pClass->dumpState(p, "", &state);
    return state;
}

static void go_office_trimMemory(LibreOfficeKit *p, int target) {
    if (p == NULL || p->pClass == NULL || p->pClass->trimMemory == NULL) return;
    p->pClass->trimMemory(p, target);
}

static void go_office_destroy(LibreOfficeKit *p) {
    if (p == NULL || p->pClass == NULL || p->pClass->destroy == NULL) return;
    p->pClass->destroy(p);
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

// ErrNilLibrary is returned by office wrappers when the supplied *Library is nil.
var ErrNilLibrary = errors.New("lokc: library is nil")

// OfficeHandle is an opaque pointer to a LibreOfficeKit*. The zero value
// is invalid; callers must not dereference the inner pointer.
type OfficeHandle struct {
	p *C.struct__LibreOfficeKit
}

// IsValid reports whether the handle points at a live LOK instance.
func (h OfficeHandle) IsValid() bool { return h.p != nil }

// InvokeHook calls the hook function resolved by OpenLibrary. The
// user-profile URL may be empty; in that case hook_2 is called with
// NULL, matching upstream semantics.
func InvokeHook(lib *Library, userProfileURL string) (OfficeHandle, error) {
	if lib == nil {
		return OfficeHandle{}, ErrNilLibrary
	}
	cInstall := C.CString(lib.installPath)
	defer C.free(unsafe.Pointer(cInstall))

	var cProfile *C.char
	if userProfileURL != "" {
		cProfile = C.CString(userProfileURL)
		defer C.free(unsafe.Pointer(cProfile))
	}

	p := C.go_invoke_hook(lib.hookSymbol, C.int(lib.hookVersion), cInstall, cProfile)
	if p == nil {
		return OfficeHandle{}, &LOKError{Detail: "hook returned NULL"}
	}
	return OfficeHandle{p: p}, nil
}

// LOKError wraps an error string returned by getError or a synthetic
// string when the hook itself fails.
type LOKError struct {
	Detail string
}

func (e *LOKError) Error() string { return "lokc: " + e.Detail }

// OfficeGetError reads and frees the office-level error string.
// Returns "" when no error is pending.
func OfficeGetError(h OfficeHandle) string {
	if !h.IsValid() {
		return ""
	}
	return copyAndFree(C.go_office_getError(h.p))
}

// OfficeGetVersionInfo returns the raw JSON version payload.
func OfficeGetVersionInfo(h OfficeHandle) string {
	if !h.IsValid() {
		return ""
	}
	return copyAndFree(C.go_office_getVersionInfo(h.p))
}

// OfficeSetOptionalFeatures forwards to pClass->setOptionalFeatures.
func OfficeSetOptionalFeatures(h OfficeHandle, features uint64) {
	if !h.IsValid() {
		return
	}
	C.go_office_setOptionalFeatures(h.p, C.ulonglong(features))
}

// OfficeSetDocumentPassword forwards to pClass->setDocumentPassword.
func OfficeSetDocumentPassword(h OfficeHandle, url, password string) {
	if !h.IsValid() {
		return
	}
	cURL := C.CString(url)
	defer C.free(unsafe.Pointer(cURL))
	var cPwd *C.char
	if password != "" {
		cPwd = C.CString(password)
		defer C.free(unsafe.Pointer(cPwd))
	}
	C.go_office_setDocumentPassword(h.p, cURL, cPwd)
}

// OfficeSetAuthor forwards to pClass->setOption("Author", value).
// The vendored LibreOfficeKit.h has no dedicated setAuthor vtable
// entry; setOption is the documented route.
func OfficeSetAuthor(h OfficeHandle, author string) {
	if !h.IsValid() {
		return
	}
	cAuthor := C.CString(author)
	defer C.free(unsafe.Pointer(cAuthor))
	C.go_office_setAuthor(h.p, cAuthor)
}

// OfficeDumpState returns pClass->dumpState's allocated state string.
func OfficeDumpState(h OfficeHandle) string {
	if !h.IsValid() {
		return ""
	}
	return copyAndFree(C.go_office_dumpState(h.p))
}

// OfficeTrimMemory forwards to pClass->trimMemory with the caller's target level.
func OfficeTrimMemory(h OfficeHandle, target int) {
	if !h.IsValid() {
		return
	}
	C.go_office_trimMemory(h.p, C.int(target))
}

// OfficeDestroy is idempotent: calling on a zero handle is a no-op.
func OfficeDestroy(h OfficeHandle) {
	if !h.IsValid() {
		return
	}
	C.go_office_destroy(h.p)
}
