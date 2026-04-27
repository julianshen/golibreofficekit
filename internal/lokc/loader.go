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

// OpenLibrary dlopens the LibreOffice runtime under installPath and
// resolves libreofficekit_hook_2, falling back to libreofficekit_hook.
//
// On Linux it walks soCandidates() — first libsofficeapp.so (the
// upstream LOK layout) and then libmergedlo.so (Debian/Ubuntu's
// apt libreoffice merged build). On macOS only libsofficeapp.dylib
// is tried.
func OpenLibrary(installPath string) (*Library, error) {
	if installPath == "" {
		return nil, ErrInstallPathRequired
	}
	var lastErr error
	for _, name := range soCandidates() {
		handle, err := dlOpen(filepath.Join(installPath, name))
		if err != nil {
			lastErr = err
			continue
		}
		// Found a runtime; the hook symbol must be in this one — we
		// don't fall through to other candidates if a runtime opened
		// but lacks the hook, because mixing runtimes from the same
		// installPath is never correct.
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
				lastErr = symErr
			}
		}
		return nil, lastErr
	}
	return nil, lastErr
}

// soCandidates returns the runtime filenames OpenLibrary will try, in
// preference order. Linux walks both the upstream layout and Ubuntu's
// merged-build layout; darwin uses the upstream name only.
func soCandidates() []string {
	if runtime.GOOS == "darwin" {
		return []string{"libsofficeapp.dylib"}
	}
	return []string{"libsofficeapp.so", "libmergedlo.so"}
}
