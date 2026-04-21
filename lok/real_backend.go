//go:build linux || darwin

package lok

import "github.com/julianshen/golibreofficekit/internal/lokc"

// realBackend wires into internal/lokc. It holds no state; the package
// uses a single instance (set by New) so tests can replace it via
// setBackend.
type realBackend struct{}

func (realBackend) OpenLibrary(installPath string) (libraryHandle, error) {
	lib, err := lokc.OpenLibrary(installPath)
	if err != nil {
		return nil, err
	}
	return realLibraryHandle{lib: lib}, nil
}

func (realBackend) InvokeHook(lib libraryHandle, userProfileURL string) (officeHandle, error) {
	rh, ok := lib.(realLibraryHandle)
	if !ok {
		// A fake libraryHandle reached the real backend; this is
		// always a test-wiring bug, never a runtime condition. Panic
		// to surface it immediately rather than return an error that
		// callers might swallow.
		panic("lok: library handle does not match real backend")
	}
	oh, err := lokc.InvokeHook(rh.lib, userProfileURL)
	if err != nil {
		return nil, err
	}
	return realOfficeHandle{h: oh}, nil
}

func (realBackend) OfficeGetError(h officeHandle) string {
	return lokc.OfficeGetError(must(h).h)
}
func (realBackend) OfficeGetVersionInfo(h officeHandle) string {
	return lokc.OfficeGetVersionInfo(must(h).h)
}
func (realBackend) OfficeSetOptionalFeatures(h officeHandle, features uint64) {
	lokc.OfficeSetOptionalFeatures(must(h).h, features)
}
func (realBackend) OfficeSetDocumentPassword(h officeHandle, url, password string) {
	lokc.OfficeSetDocumentPassword(must(h).h, url, password)
}
func (realBackend) OfficeSetAuthor(h officeHandle, author string) {
	lokc.OfficeSetAuthor(must(h).h, author)
}
func (realBackend) OfficeDumpState(h officeHandle) string {
	return lokc.OfficeDumpState(must(h).h)
}
func (realBackend) OfficeTrimMemory(h officeHandle, target int) {
	lokc.OfficeTrimMemory(must(h).h, target)
}
func (realBackend) OfficeDestroy(h officeHandle) {
	lokc.OfficeDestroy(must(h).h)
}

type realLibraryHandle struct{ lib *lokc.Library }

func (realLibraryHandle) libraryBrand() {}

type realOfficeHandle struct{ h lokc.OfficeHandle }

func (realOfficeHandle) officeBrand() {}

func must(h officeHandle) realOfficeHandle {
	rh, ok := h.(realOfficeHandle)
	if !ok {
		// Programmer error: a fake handle reached the real backend.
		panic("lok: handle/backend mismatch")
	}
	return rh
}
