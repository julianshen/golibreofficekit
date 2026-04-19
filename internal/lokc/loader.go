//go:build linux || darwin

package lokc

import (
	"errors"
	"path/filepath"
	"runtime"
	"unsafe"
)

// ErrInstallPathRequired is returned by OpenLibrary when installPath is empty.
var ErrInstallPathRequired = errors.New("lokc: install path is required")

// Library is a dlopen'd LibreOffice runtime with a resolved hook symbol.
// Close is intentionally not provided: LibreOffice's static initialisers
// cannot be re-run within the same process.
type Library struct {
	installPath string
	handle      unsafe.Pointer
	hookSymbol  unsafe.Pointer
	hookVersion int // 2 for libreofficekit_hook_2, 1 for libreofficekit_hook
}

// InstallPath returns the path that was passed to OpenLibrary.
func (l *Library) InstallPath() string { return l.installPath }

// HookVersion returns 2 for libreofficekit_hook_2, 1 for libreofficekit_hook.
func (l *Library) HookVersion() int { return l.hookVersion }

// HookSymbol returns the resolved function pointer. Callers must cast
// to the right signature (done in Phase 2).
func (l *Library) HookSymbol() unsafe.Pointer { return l.hookSymbol }

// OpenLibrary dlopens <installPath>/libsofficeapp.so and resolves
// libreofficekit_hook_2 (falling back to libreofficekit_hook). It does
// NOT invoke the hook; that is Phase 2.
func OpenLibrary(installPath string) (*Library, error) {
	if installPath == "" {
		return nil, ErrInstallPathRequired
	}
	return openWithPath(
		filepath.Join(installPath, soFilename()),
		installPath,
		"libreofficekit_hook_2",
		"libreofficekit_hook",
	)
}

func soFilename() string {
	if runtime.GOOS == "darwin" {
		return "libsofficeapp.dylib"
	}
	return "libsofficeapp.so"
}

// openWithPath is the test-facing seam: it takes an absolute library
// path plus ordered symbol candidates and tries them in sequence.
func openWithPath(libPath, installPath, preferredSym, fallbackSym string) (*Library, error) {
	handle, err := dlOpen(libPath)
	if err != nil {
		return nil, err
	}
	if sym, err := dlSym(handle, preferredSym); err == nil {
		return &Library{installPath: installPath, handle: handle, hookSymbol: sym, hookVersion: 2}, nil
	}
	sym, err := dlSym(handle, fallbackSym)
	if err != nil {
		return nil, err
	}
	return &Library{installPath: installPath, handle: handle, hookSymbol: sym, hookVersion: 1}, nil
}
