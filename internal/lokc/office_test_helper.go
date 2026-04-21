//go:build linux || darwin

package lokc

/*
#include <stdlib.h>
#include "LibreOfficeKit/LibreOfficeKit.h"

// newFakeLOK allocates a zeroed LibreOfficeKit (pClass == NULL). The
// office.go wrappers' C-side guards (p->pClass == NULL → early return)
// make every operation a safe no-op, so tests can exercise every
// Go-side statement without a real LO install.
static LibreOfficeKit* newFakeLOK(void) {
	return (LibreOfficeKit*)calloc(1, sizeof(LibreOfficeKit));
}

static void freeFakeLOK(LibreOfficeKit* p) {
	free(p);
}

// fakeHook ignores its arguments and returns a zeroed LibreOfficeKit.
static LibreOfficeKit* fakeHook(const char *install_path) {
	(void)install_path;
	return newFakeLOK();
}

static LibreOfficeKit* fakeHook2(const char *install_path, const char *user_profile_url) {
	(void)install_path; (void)user_profile_url;
	return newFakeLOK();
}

static void* fakeHookPtr(int version) {
	if (version == 2) return (void*)fakeHook2;
	return (void*)fakeHook;
}

// fakeHookNilPtr returns a function pointer that always returns NULL,
// used to exercise InvokeHook's "hook returned NULL" error path.
static LibreOfficeKit* fakeHookNullReturn(const char *install_path) {
	(void)install_path;
	return NULL;
}

static void* fakeHookNullReturnPtr(void) {
	return (void*)fakeHookNullReturn;
}
*/
import "C"

import "unsafe"

// NewFakeOfficeHandle returns an OfficeHandle backed by a calloc'd
// zeroed LibreOfficeKit. The wrappers treat pClass == NULL as a
// vtable-not-present signal and early-return in C, so every tested
// operation is a no-op at the C layer while exercising the Go path.
// The caller must call FreeFakeOfficeHandle to reclaim the allocation.
//
// Exported for the benefit of tests in the public lok package; the
// identifier stays invisible outside the module because lokc lives
// under internal/.
func NewFakeOfficeHandle() OfficeHandle {
	return OfficeHandle{p: C.newFakeLOK()}
}

// FreeFakeOfficeHandle releases the backing LibreOfficeKit.
func FreeFakeOfficeHandle(h OfficeHandle) {
	if h.p != nil {
		C.freeFakeLOK(h.p)
	}
}

// NewFakeLibrary returns a Library whose hookSymbol points at a C
// helper that returns a zeroed LibreOfficeKit (never the real LOK).
// version must be 1 or 2.
func NewFakeLibrary(version int) *Library {
	return &Library{
		installPath: "/fake/install",
		hookSymbol:  unsafe.Pointer(C.fakeHookPtr(C.int(version))),
		hookVersion: version,
	}
}

// NewFakeLibraryNullReturn returns a Library whose hook always returns
// NULL; useful for testing InvokeHook's "hook returned NULL" path.
func NewFakeLibraryNullReturn() *Library {
	return &Library{
		installPath: "/fake/install",
		hookSymbol:  unsafe.Pointer(C.fakeHookNullReturnPtr()),
		hookVersion: 1,
	}
}
