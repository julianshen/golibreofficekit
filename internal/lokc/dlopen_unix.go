//go:build linux || darwin

package lokc

/*
#cgo LDFLAGS: -ldl
#include <dlfcn.h>
#include <stdlib.h>

// Wrapper so Go can call dlopen without preprocessor macros.
static void *go_dlopen(const char *path, int flag) {
	// An empty Go string arrives here as a zero-length C string. Pass
	// NULL instead so the main-program handle is returned on both
	// Linux and macOS (macOS dlopen("") returns NULL+error).
	if (path != NULL && path[0] == '\0') {
		return dlopen(NULL, flag);
	}
	return dlopen(path, flag);
}

static void *go_dlsym(void *handle, const char *name) {
	// Clear any pending error first so a genuine NULL return is
	// disambiguated from a stale dlerror.
	(void)dlerror();
	return dlsym(handle, name);
}

static char *go_dlerror(void) {
	return dlerror();
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// DLError is returned when dlopen or dlsym fails.
type DLError struct {
	Op     string // "dlopen" or "dlsym"
	Target string // path for dlopen, symbol name for dlsym
	Detail string // dlerror() output
}

func (e *DLError) Error() string {
	return fmt.Sprintf("%s %q: %s", e.Op, e.Target, e.Detail)
}

// dlOpen resolves a shared library path via dlopen(RTLD_LAZY). This
// matches LibreOfficeKitInit.h's lok_loadlib, which LO's plugin graph
// relies on for cross-library symbol resolution. An empty path opens
// the main-program handle (portable across Linux and macOS via the
// NULL-translation in go_dlopen).
//
// Callers must not dlclose: LibreOffice's static init cannot be re-run
// cleanly within the same process.
func dlOpen(path string) (unsafe.Pointer, error) {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))

	handle := C.go_dlopen(cpath, C.RTLD_LAZY)
	if handle == nil {
		return nil, &DLError{Op: "dlopen", Target: path, Detail: lastDLError()}
	}
	return unsafe.Pointer(handle), nil
}

// dlSym resolves a symbol in a handle obtained from dlOpen.
func dlSym(handle unsafe.Pointer, symbol string) (unsafe.Pointer, error) {
	if handle == nil {
		return nil, &DLError{Op: "dlsym", Target: symbol, Detail: "handle is nil"}
	}
	csym := C.CString(symbol)
	defer C.free(unsafe.Pointer(csym))

	ptr := C.go_dlsym(handle, csym)
	if ptr == nil {
		return nil, &DLError{Op: "dlsym", Target: symbol, Detail: lastDLError()}
	}
	return unsafe.Pointer(ptr), nil
}

func lastDLError() string {
	cs := C.go_dlerror()
	if cs == nil {
		return "(no dlerror)"
	}
	return C.GoString(cs)
}
