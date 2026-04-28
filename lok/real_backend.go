//go:build linux || darwin

package lok

import (
	"errors"

	"github.com/julianshen/golibreofficekit/internal/lokc"
)

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
func (realBackend) DocumentDestroyView(d documentHandle, id int) error {
	return mapLokErr(lokc.DocumentDestroyView(mustDoc(d).d, id))
}
func (realBackend) DocumentSetView(d documentHandle, id int) error {
	return mapLokErr(lokc.DocumentSetView(mustDoc(d).d, id))
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
func (realBackend) DocumentSetViewLanguage(d documentHandle, id int, lang string) error {
	return mapLokErr(lokc.DocumentSetViewLanguage(mustDoc(d).d, id, lang))
}
func (realBackend) DocumentSetViewReadOnly(d documentHandle, id int, readOnly bool) error {
	return mapLokErr(lokc.DocumentSetViewReadOnly(mustDoc(d).d, id, readOnly))
}
func (realBackend) DocumentSetAccessibilityState(d documentHandle, id int, enabled bool) error {
	return mapLokErr(lokc.DocumentSetAccessibilityState(mustDoc(d).d, id, enabled))
}
func (realBackend) DocumentSetViewTimezone(d documentHandle, id int, tz string) error {
	return mapLokErr(lokc.DocumentSetViewTimezone(mustDoc(d).d, id, tz))
}

func (realBackend) DocumentGetParts(d documentHandle) int {
	return lokc.DocumentGetParts(mustDoc(d).d)
}
func (realBackend) DocumentGetPart(d documentHandle) int {
	return lokc.DocumentGetPart(mustDoc(d).d)
}
func (realBackend) DocumentSetPart(d documentHandle, n int) error {
	return mapLokErr(lokc.DocumentSetPart(mustDoc(d).d, n))
}
func (realBackend) DocumentSetPartMode(d documentHandle, mode int) error {
	return mapLokErr(lokc.DocumentSetPartMode(mustDoc(d).d, mode))
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
func (realBackend) DocumentSetOutlineState(d documentHandle, column bool, level, index int, hidden bool) error {
	return mapLokErr(lokc.DocumentSetOutlineState(mustDoc(d).d, column, level, index, hidden))
}

func (realBackend) DocumentInitializeForRendering(d documentHandle, args string) error {
	return mapLokErr(lokc.DocumentInitializeForRendering(mustDoc(d).d, args))
}
func (realBackend) DocumentGetTileMode(d documentHandle) int {
	return lokc.DocumentGetTileMode(mustDoc(d).d)
}
func (realBackend) DocumentSetClientZoom(d documentHandle, tpw, tph, ttw, tth int) error {
	return mapLokErr(lokc.DocumentSetClientZoom(mustDoc(d).d, tpw, tph, ttw, tth))
}
func (realBackend) DocumentSetClientVisibleArea(d documentHandle, x, y, w, h int) error {
	return mapLokErr(lokc.DocumentSetClientVisibleArea(mustDoc(d).d, x, y, w, h))
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

func (realBackend) DocumentPostKeyEvent(d documentHandle, typ, charCode, keyCode int) error {
	return mapLokErr(lokc.DocumentPostKeyEvent(mustDoc(d).d, typ, charCode, keyCode))
}
func (realBackend) DocumentPostMouseEvent(d documentHandle, typ, x, y, count, buttons, mods int) error {
	return mapLokErr(lokc.DocumentPostMouseEvent(mustDoc(d).d, typ, x, y, count, buttons, mods))
}
func (realBackend) DocumentPostUnoCommand(d documentHandle, cmd, args string, notifyWhenFinished bool) error {
	return mapLokErr(lokc.DocumentPostUnoCommand(mustDoc(d).d, cmd, args, notifyWhenFinished))
}

// mapLokErr translates internal lokc sentinels to their public lok
// counterparts. New realBackend forwarders that surface a lokc error
// should pipe through here so the translation stays in one place.
//
// ErrNilOffice / ErrNilDocument indicate a zero-valued handle reached
// the C layer — the public-API methods always check o.closed / d.closed
// under the mutex first, so a real-world path here is a programmer
// error (zero-value Office{} or use-after-free), not the documented
// lifecycle "user called Close()". Surfaced as *LOKError so callers
// don't accidentally errors.Is(err, ErrClosed) past a real bug.
func mapLokErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, lokc.ErrUnsupported):
		return ErrUnsupported
	case errors.Is(err, lokc.ErrNoValue):
		return ErrNoValue
	case errors.Is(err, lokc.ErrNilOffice):
		return &LOKError{Detail: "nil office handle reached backend; programmer error", err: err}
	case errors.Is(err, lokc.ErrNilDocument):
		return &LOKError{Detail: "nil document handle reached backend; programmer error", err: err}
	case errors.Is(err, lokc.ErrMacroFailed):
		return ErrMacroFailed
	case errors.Is(err, lokc.ErrSignFailed):
		return ErrSignFailed
	case errors.Is(err, lokc.ErrPasteFailed):
		return ErrPasteFailed
	case errors.Is(err, lokc.ErrSaveFailed):
		// Detail intentionally minimal: mapLokErr has no office
		// handle, so it cannot consult OfficeGetError. The typical
		// save path is Document.SaveAs / Save, which use wrapLOErr
		// to populate Detail from LO. This case is the safety net
		// for any future forwarder that surfaces a save failure
		// without an Office in scope.
		return &LOKError{Op: "Save", Detail: err.Error(), err: err}
	case errors.Is(err, lokc.ErrClipboardFailed):
		return ErrClipboardFailed
	}
	return err
}

func (realBackend) DocumentSetTextSelection(d documentHandle, typ, x, y int) error {
	return mapLokErr(lokc.DocumentSetTextSelection(mustDoc(d).d, typ, x, y))
}
func (realBackend) DocumentResetSelection(d documentHandle) error {
	return mapLokErr(lokc.DocumentResetSelection(mustDoc(d).d))
}
func (realBackend) DocumentSetGraphicSelection(d documentHandle, typ, x, y int) error {
	return mapLokErr(lokc.DocumentSetGraphicSelection(mustDoc(d).d, typ, x, y))
}
func (realBackend) DocumentSetBlockedCommandList(d documentHandle, viewID int, csv string) error {
	return mapLokErr(lokc.DocumentSetBlockedCommandList(mustDoc(d).d, viewID, csv))
}
func (realBackend) DocumentGetTextSelection(d documentHandle, mimeType string) (string, string, error) {
	text, mime, err := lokc.DocumentGetTextSelection(mustDoc(d).d, mimeType)
	return text, mime, mapLokErr(err)
}
func (realBackend) DocumentGetSelectionType(d documentHandle) (int, error) {
	v, err := lokc.DocumentGetSelectionType(mustDoc(d).d)
	return v, mapLokErr(err)
}
func (realBackend) DocumentGetSelectionTypeAndText(d documentHandle, mimeType string) (int, string, string, error) {
	kind, text, mime, err := lokc.DocumentGetSelectionTypeAndText(mustDoc(d).d, mimeType)
	return kind, text, mime, mapLokErr(err)
}

func (realBackend) DocumentGetClipboard(d documentHandle, mimeTypes []string) ([]clipboardItemInternal, error) {
	items, err := lokc.DocumentGetClipboard(mustDoc(d).d, mimeTypes)
	if err != nil {
		return nil, mapLokErr(err)
	}
	out := make([]clipboardItemInternal, len(items))
	for i, it := range items {
		out[i] = clipboardItemInternal{MimeType: it.MimeType, Data: it.Data}
	}
	return out, nil
}

func (realBackend) DocumentSetClipboard(d documentHandle, items []clipboardItemInternal) error {
	lokItems := make([]lokc.ClipboardItem, len(items))
	for i, it := range items {
		lokItems[i] = lokc.ClipboardItem{MimeType: it.MimeType, Data: it.Data}
	}
	return mapLokErr(lokc.DocumentSetClipboard(mustDoc(d).d, lokItems))
}

func (realBackend) RegisterOfficeCallback(h officeHandle, handle uintptr) error {
	return mapLokErr(lokc.RegisterOfficeCallback(must(h).h, lokc.DispatchHandleFromUintptr(handle)))
}
func (realBackend) RegisterDocumentCallback(d documentHandle, handle uintptr) error {
	return mapLokErr(lokc.RegisterDocumentCallback(mustDoc(d).d, lokc.DispatchHandleFromUintptr(handle)))
}

// --- Command & window operations ---

func (realBackend) GetCommandValues(d documentHandle, command string) (string, error) {
	v, err := lokc.DocumentGetCommandValues(mustDoc(d).d, command)
	return v, mapLokErr(err)
}

func (realBackend) CompleteFunction(d documentHandle, name string) error {
	return mapLokErr(lokc.DocumentCompleteFunction(mustDoc(d).d, name))
}

func (realBackend) SendDialogEvent(d documentHandle, windowID uint64, argsJSON string) error {
	return mapLokErr(lokc.DocumentSendDialogEvent(mustDoc(d).d, windowID, argsJSON))
}

func (realBackend) SendContentControlEvent(d documentHandle, argsJSON string) error {
	return mapLokErr(lokc.DocumentSendContentControlEvent(mustDoc(d).d, argsJSON))
}

func (realBackend) SendFormFieldEvent(d documentHandle, argsJSON string) error {
	return mapLokErr(lokc.DocumentSendFormFieldEvent(mustDoc(d).d, argsJSON))
}

func (realBackend) PostWindowKeyEvent(d documentHandle, windowID uint32, typ, charCode, keyCode int) error {
	return mapLokErr(lokc.DocumentPostWindowKeyEvent(mustDoc(d).d, windowID, typ, charCode, keyCode))
}

func (realBackend) PostWindowMouseEvent(d documentHandle, windowID uint32, typ int, x, y int64, count int, buttons, mods int) error {
	return mapLokErr(lokc.DocumentPostWindowMouseEvent(mustDoc(d).d, windowID, typ, x, y, count, buttons, mods))
}

func (realBackend) PostWindowGestureEvent(d documentHandle, windowID uint32, typ string, x, y, offset int64) error {
	return mapLokErr(lokc.DocumentPostWindowGestureEvent(mustDoc(d).d, windowID, typ, x, y, offset))
}

func (realBackend) PostWindowExtTextInputEvent(d documentHandle, windowID uint32, typ int, text string) error {
	return mapLokErr(lokc.DocumentPostWindowExtTextInputEvent(mustDoc(d).d, windowID, typ, text))
}

func (realBackend) ResizeWindow(d documentHandle, windowID uint32, w, h int) error {
	return mapLokErr(lokc.DocumentResizeWindow(mustDoc(d).d, windowID, w, h))
}

func (realBackend) PaintWindow(d documentHandle, windowID uint32, buf []byte, x, y, pxW, pxH int) error {
	return mapLokErr(lokc.DocumentPaintWindow(mustDoc(d).d, windowID, buf, x, y, pxW, pxH))
}

func (realBackend) PaintWindowDPI(d documentHandle, windowID uint32, buf []byte, x, y, pxW, pxH int, dpiScale float64) error {
	return mapLokErr(lokc.DocumentPaintWindowDPI(mustDoc(d).d, windowID, buf, x, y, pxW, pxH, dpiScale))
}

func (realBackend) PaintWindowForView(d documentHandle, windowID uint32, view int, buf []byte, x, y, pxW, pxH int, dpiScale float64) error {
	return mapLokErr(lokc.DocumentPaintWindowForView(mustDoc(d).d, windowID, view, buf, x, y, pxW, pxH, dpiScale))
}

// --- Advanced + gap-fill operations ---

func (realBackend) OfficeRunMacro(h officeHandle, url string) error {
	return mapLokErr(lokc.OfficeRunMacro(must(h).h, url))
}

func (realBackend) OfficeSignDocument(h officeHandle, url string, cert, key []byte) error {
	return mapLokErr(lokc.OfficeSignDocument(must(h).h, url, cert, key))
}

func (realBackend) OfficeGetFilterTypes(h officeHandle) (string, error) {
	v, err := lokc.OfficeGetFilterTypes(must(h).h)
	return v, mapLokErr(err)
}

func (realBackend) DocumentInsertCertificate(d documentHandle, cert, key []byte) error {
	return mapLokErr(lokc.DocumentInsertCertificate(mustDoc(d).d, cert, key))
}

func (realBackend) DocumentAddCertificate(d documentHandle, cert []byte) error {
	return mapLokErr(lokc.DocumentAddCertificate(mustDoc(d).d, cert))
}

func (realBackend) DocumentGetSignatureState(d documentHandle) (int, error) {
	v, err := lokc.DocumentGetSignatureState(mustDoc(d).d)
	return v, mapLokErr(err)
}

func (realBackend) DocumentPaste(d documentHandle, mimeType string, data []byte) error {
	return mapLokErr(lokc.DocumentPaste(mustDoc(d).d, mimeType, data))
}

func (realBackend) DocumentSelectPart(d documentHandle, part int, selected bool) error {
	return mapLokErr(lokc.DocumentSelectPart(mustDoc(d).d, part, selected))
}

func (realBackend) DocumentMoveSelectedParts(d documentHandle, position int, duplicate bool) error {
	return mapLokErr(lokc.DocumentMoveSelectedParts(mustDoc(d).d, position, duplicate))
}

func (realBackend) DocumentRenderFont(d documentHandle, fontName, char string) ([]byte, int, int, error) {
	buf, w, h, err := lokc.DocumentRenderFont(mustDoc(d).d, fontName, char)
	return buf, w, h, mapLokErr(err)
}

var _ backend = realBackend{}

func init() {
	setBackend(realBackend{})
}
