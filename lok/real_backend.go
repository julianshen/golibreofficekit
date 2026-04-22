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

type realDocumentHandle struct{ d lokc.DocumentHandle }

func (realDocumentHandle) documentBrand() {}

func (realBackend) DocumentLoad(h officeHandle, url string) (documentHandle, error) {
	doc := lokc.DocumentLoad(must(h).h, url)
	if !doc.IsValid() {
		// Wrap lokc.ErrNilDocument so callers can errors.Is against
		// it through the public *LOKError.Unwrap() chain.
		return nil, &LOKError{Op: "Load", Detail: "documentLoad returned NULL", err: lokc.ErrNilDocument}
	}
	return realDocumentHandle{d: doc}, nil
}

func (realBackend) DocumentLoadWithOptions(h officeHandle, url, options string) (documentHandle, error) {
	doc := lokc.DocumentLoadWithOptions(must(h).h, url, options)
	if !doc.IsValid() {
		return nil, &LOKError{Op: "Load", Detail: "documentLoadWithOptions returned NULL", err: lokc.ErrNilDocument}
	}
	return realDocumentHandle{d: doc}, nil
}

func (realBackend) DocumentGetType(d documentHandle) int {
	return lokc.DocumentGetType(mustDoc(d).d)
}

func (realBackend) DocumentSaveAs(d documentHandle, url, format, filterOptions string) error {
	return lokc.DocumentSaveAs(mustDoc(d).d, url, format, filterOptions)
}

func (realBackend) DocumentDestroy(d documentHandle) {
	lokc.DocumentDestroy(mustDoc(d).d)
}

func mustDoc(d documentHandle) realDocumentHandle {
	rd, ok := d.(realDocumentHandle)
	if !ok {
		panic("lok: document handle does not match real backend")
	}
	return rd
}

func (realBackend) DocumentCreateView(d documentHandle) int {
	return lokc.DocumentCreateView(mustDoc(d).d)
}
func (realBackend) DocumentCreateViewWithOptions(d documentHandle, options string) int {
	return lokc.DocumentCreateViewWithOptions(mustDoc(d).d, options)
}
func (realBackend) DocumentDestroyView(d documentHandle, id int) {
	lokc.DocumentDestroyView(mustDoc(d).d, id)
}
func (realBackend) DocumentSetView(d documentHandle, id int) {
	lokc.DocumentSetView(mustDoc(d).d, id)
}
func (realBackend) DocumentGetView(d documentHandle) int {
	return lokc.DocumentGetView(mustDoc(d).d)
}
func (realBackend) DocumentGetViewsCount(d documentHandle) int {
	return lokc.DocumentGetViewsCount(mustDoc(d).d)
}
func (realBackend) DocumentGetViewIds(d documentHandle) ([]int, bool) {
	return lokc.DocumentGetViewIds(mustDoc(d).d)
}
func (realBackend) DocumentSetViewLanguage(d documentHandle, id int, lang string) {
	lokc.DocumentSetViewLanguage(mustDoc(d).d, id, lang)
}
func (realBackend) DocumentSetViewReadOnly(d documentHandle, id int, readOnly bool) {
	lokc.DocumentSetViewReadOnly(mustDoc(d).d, id, readOnly)
}
func (realBackend) DocumentSetAccessibilityState(d documentHandle, id int, enabled bool) {
	lokc.DocumentSetAccessibilityState(mustDoc(d).d, id, enabled)
}
func (realBackend) DocumentSetViewTimezone(d documentHandle, id int, tz string) {
	lokc.DocumentSetViewTimezone(mustDoc(d).d, id, tz)
}

func init() {
	setBackend(realBackend{})
}
