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
	hookSymbol  unsafe.Pointer
	hookVersion int
}

// InstallPath returns the path that was passed to OpenLibrary.
func (l *Library) InstallPath() string { return l.installPath }

// HookVersion returns 2 for libreofficekit_hook_2, 1 for libreofficekit_hook.
func (l *Library) HookVersion() int { return l.hookVersion }

// HookSymbol returns the resolved function pointer. Callers must cast
// to the right signature.
func (l *Library) HookSymbol() unsafe.Pointer { return l.hookSymbol }

// OpenLibrary dlopens <installPath>/libsofficeapp.{so,dylib} and resolves
// libreofficekit_hook_2, falling back to libreofficekit_hook.
func OpenLibrary(installPath string) (*Library, error) {
	if installPath == "" {
		return nil, ErrInstallPathRequired
	}
	handle, err := dlOpen(filepath.Join(installPath, soFilename()))
	if err != nil {
		return nil, err
	}
	for _, attempt := range []struct {
		name    string
		version int
	}{
		{"libreofficekit_hook_2", 2},
		{"libreofficekit_hook", 1},
	} {
		if sym, symErr := dlSym(handle, attempt.name); symErr == nil {
			return &Library{
				installPath: installPath,
				hookSymbol:  sym,
				hookVersion: attempt.version,
			}, nil
		} else {
			err = symErr
		}
	}
	return nil, err
}

func soFilename() string {
	if runtime.GOOS == "darwin" {
		return "libsofficeapp.dylib"
	}
	return "libsofficeapp.so"
}
