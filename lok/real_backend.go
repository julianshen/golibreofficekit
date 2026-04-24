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

func (realBackend) DocumentGetParts(d documentHandle) int {
	return lokc.DocumentGetParts(mustDoc(d).d)
}
func (realBackend) DocumentGetPart(d documentHandle) int {
	return lokc.DocumentGetPart(mustDoc(d).d)
}
func (realBackend) DocumentSetPart(d documentHandle, n int) {
	lokc.DocumentSetPart(mustDoc(d).d, n)
}
func (realBackend) DocumentSetPartMode(d documentHandle, mode int) {
	lokc.DocumentSetPartMode(mustDoc(d).d, mode)
}
func (realBackend) DocumentGetPartName(d documentHandle, n int) string {
	return lokc.DocumentGetPartName(mustDoc(d).d, n)
}
func (realBackend) DocumentGetPartHash(d documentHandle, n int) string {
	return lokc.DocumentGetPartHash(mustDoc(d).d, n)
}
func (realBackend) DocumentGetPartInfo(d documentHandle, n int) string {
	return lokc.DocumentGetPartInfo(mustDoc(d).d, n)
}
func (realBackend) DocumentGetPartPageRectangles(d documentHandle) string {
	return lokc.DocumentGetPartPageRectangles(mustDoc(d).d)
}
func (realBackend) DocumentGetDocumentSize(d documentHandle) (int64, int64) {
	return lokc.DocumentGetDocumentSize(mustDoc(d).d)
}
func (realBackend) DocumentSetOutlineState(d documentHandle, column bool, level, index int, hidden bool) {
	lokc.DocumentSetOutlineState(mustDoc(d).d, column, level, index, hidden)
}

func (realBackend) DocumentInitializeForRendering(d documentHandle, args string) {
	lokc.DocumentInitializeForRendering(mustDoc(d).d, args)
}
func (realBackend) DocumentGetTileMode(d documentHandle) int {
	return lokc.DocumentGetTileMode(mustDoc(d).d)
}
func (realBackend) DocumentSetClientZoom(d documentHandle, tpw, tph, ttw, tth int) {
	lokc.DocumentSetClientZoom(mustDoc(d).d, tpw, tph, ttw, tth)
}
func (realBackend) DocumentSetClientVisibleArea(d documentHandle, x, y, w, h int) {
	lokc.DocumentSetClientVisibleArea(mustDoc(d).d, x, y, w, h)
}
func (realBackend) DocumentPaintTile(d documentHandle, buf []byte, pxW, pxH, x, y, w, h int) {
	lokc.DocumentPaintTile(mustDoc(d).d, buf, pxW, pxH, x, y, w, h)
}
func (realBackend) DocumentPaintPartTile(d documentHandle, buf []byte, part, mode, pxW, pxH, x, y, w, h int) {
	lokc.DocumentPaintPartTile(mustDoc(d).d, buf, part, mode, pxW, pxH, x, y, w, h)
}
func (realBackend) DocumentRenderSearchResult(d documentHandle, q string) ([]byte, int, int, bool) {
	return lokc.DocumentRenderSearchResult(mustDoc(d).d, q)
}
func (realBackend) DocumentRenderShapeSelection(d documentHandle) []byte {
	return lokc.DocumentRenderShapeSelection(mustDoc(d).d)
}

func (realBackend) DocumentPostKeyEvent(d documentHandle, typ, charCode, keyCode int) {
	lokc.DocumentPostKeyEvent(mustDoc(d).d, typ, charCode, keyCode)
}
func (realBackend) DocumentPostMouseEvent(d documentHandle, typ, x, y, count, buttons, mods int) {
	lokc.DocumentPostMouseEvent(mustDoc(d).d, typ, x, y, count, buttons, mods)
}
func (realBackend) DocumentPostUnoCommand(d documentHandle, cmd, args string, notifyWhenFinished bool) {
	lokc.DocumentPostUnoCommand(mustDoc(d).d, cmd, args, notifyWhenFinished)
}

func (realBackend) DocumentSetTextSelection(d documentHandle, typ, x, y int) {
	lokc.DocumentSetTextSelection(mustDoc(d).d, typ, x, y)
}
func (realBackend) DocumentResetSelection(d documentHandle) {
	lokc.DocumentResetSelection(mustDoc(d).d)
}
func (realBackend) DocumentSetGraphicSelection(d documentHandle, typ, x, y int) {
	lokc.DocumentSetGraphicSelection(mustDoc(d).d, typ, x, y)
}
func (realBackend) DocumentSetBlockedCommandList(d documentHandle, viewID int, csv string) {
	lokc.DocumentSetBlockedCommandList(mustDoc(d).d, viewID, csv)
}
func (realBackend) DocumentGetTextSelection(d documentHandle, mimeType string) (string, string) {
	return lokc.DocumentGetTextSelection(mustDoc(d).d, mimeType)
}
func (realBackend) DocumentGetSelectionType(d documentHandle) int {
	return lokc.DocumentGetSelectionType(mustDoc(d).d)
}
func (realBackend) DocumentGetSelectionTypeAndText(d documentHandle, mimeType string) (int, string, string, error) {
	kind, text, mime, err := lokc.DocumentGetSelectionTypeAndText(mustDoc(d).d, mimeType)
	if err == lokc.ErrUnsupported {
		return -1, "", "", ErrUnsupported
	}
	return kind, text, mime, err
}

// var _ backend = realBackend{} is a compile-time assertion that
// realBackend satisfies the full backend interface.
var _ backend = realBackend{}

func init() {
	setBackend(realBackend{})
}
