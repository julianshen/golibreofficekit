package lok

// backend is the narrow seam between lok and internal/lokc. Production
// code wires the real implementation in real_backend.go; tests inject
// a fake in office_test.go. The interface stays private so it can
// evolve without breaking API compatibility.
type backend interface {
	OpenLibrary(installPath string) (libraryHandle, error)
	InvokeHook(lib libraryHandle, userProfileURL string) (officeHandle, error)
	OfficeGetError(h officeHandle) string
	OfficeGetVersionInfo(h officeHandle) string
	OfficeSetOptionalFeatures(h officeHandle, features uint64)
	OfficeSetDocumentPassword(h officeHandle, url, password string)
	OfficeSetAuthor(h officeHandle, author string)
	OfficeDumpState(h officeHandle) string
	OfficeTrimMemory(h officeHandle, target int)
	OfficeDestroy(h officeHandle)

	DocumentLoad(h officeHandle, url string) (documentHandle, error)
	DocumentLoadWithOptions(h officeHandle, url, options string) (documentHandle, error)
	DocumentGetType(d documentHandle) int
	DocumentSaveAs(d documentHandle, url, format, filterOptions string) error
	DocumentDestroy(d documentHandle)

	DocumentCreateView(d documentHandle) int
	DocumentCreateViewWithOptions(d documentHandle, options string) int
	DocumentDestroyView(d documentHandle, id int) error
	DocumentSetView(d documentHandle, id int) error
	DocumentGetView(d documentHandle) int
	DocumentGetViewsCount(d documentHandle) int
	DocumentGetViewIds(d documentHandle) (ids []int, ok bool)
	DocumentSetViewLanguage(d documentHandle, id int, lang string) error
	DocumentSetViewReadOnly(d documentHandle, id int, readOnly bool) error
	DocumentSetAccessibilityState(d documentHandle, id int, enabled bool) error
	DocumentSetViewTimezone(d documentHandle, id int, tz string) error

	DocumentGetParts(d documentHandle) int
	DocumentGetPart(d documentHandle) int
	DocumentSetPart(d documentHandle, n int)
	DocumentSetPartMode(d documentHandle, mode int)
	DocumentGetPartName(d documentHandle, n int) string
	DocumentGetPartHash(d documentHandle, n int) string
	DocumentGetPartInfo(d documentHandle, n int) string
	DocumentGetPartPageRectangles(d documentHandle) string
	DocumentGetDocumentSize(d documentHandle) (int64, int64)
	DocumentSetOutlineState(d documentHandle, column bool, level, index int, hidden bool)

	DocumentInitializeForRendering(d documentHandle, args string)
	DocumentGetTileMode(d documentHandle) int
	DocumentSetClientZoom(d documentHandle, tilePxW, tilePxH, tileTwipW, tileTwipH int)
	DocumentSetClientVisibleArea(d documentHandle, x, y, w, h int)
	DocumentPaintTile(d documentHandle, buf []byte, pxW, pxH, x, y, w, h int)
	DocumentPaintPartTile(d documentHandle, buf []byte, part, mode, pxW, pxH, x, y, w, h int)
	DocumentRenderSearchResult(d documentHandle, query string) (buf []byte, pxW, pxH int, ok bool)
	DocumentRenderShapeSelection(d documentHandle) []byte

	DocumentPostKeyEvent(d documentHandle, typ, charCode, keyCode int) error
	DocumentPostMouseEvent(d documentHandle, typ, x, y, count, buttons, mods int) error
	DocumentPostUnoCommand(d documentHandle, cmd, args string, notifyWhenFinished bool) error

	DocumentSetTextSelection(d documentHandle, typ, x, y int) error
	DocumentResetSelection(d documentHandle) error
	DocumentSetGraphicSelection(d documentHandle, typ, x, y int) error
	DocumentSetBlockedCommandList(d documentHandle, viewID int, csv string) error
	DocumentGetTextSelection(d documentHandle, mimeType string) (text, usedMime string, err error)
	DocumentGetSelectionType(d documentHandle) (int, error)
	DocumentGetSelectionTypeAndText(d documentHandle, mimeType string) (kind int, text, usedMime string, err error)

	DocumentGetClipboard(d documentHandle, mimeTypes []string) (items []clipboardItemInternal, err error)
	DocumentSetClipboard(d documentHandle, items []clipboardItemInternal) error

	RegisterOfficeCallback(h officeHandle, handle uintptr) error
	RegisterDocumentCallback(d documentHandle, handle uintptr) error

	// Command & window operations (Phase 10).
	GetCommandValues(d documentHandle, command string) (string, error)
	CompleteFunction(d documentHandle, name string) error
	SendDialogEvent(d documentHandle, windowID uint64, argsJSON string) error
	SendContentControlEvent(d documentHandle, argsJSON string) error
	SendFormFieldEvent(d documentHandle, argsJSON string) error
	PostWindowKeyEvent(d documentHandle, windowID uint32, typ int, charCode, keyCode int) error
	PostWindowMouseEvent(d documentHandle, windowID uint32, typ int, x, y int64, count int, buttons, mods int) error
	PostWindowGestureEvent(d documentHandle, windowID uint32, typ string, x, y, offset int64) error
	PostWindowExtTextInputEvent(d documentHandle, windowID uint32, typ int, text string) error
	ResizeWindow(d documentHandle, windowID uint32, w, h int) error
	PaintWindow(d documentHandle, windowID uint32, buf []byte, x, y, pxW, pxH int) error
	PaintWindowDPI(d documentHandle, windowID uint32, buf []byte, x, y, pxW, pxH int, dpiScale float64) error
	PaintWindowForView(d documentHandle, windowID uint32, view int, buf []byte, x, y, pxW, pxH int, dpiScale float64) error

	// Advanced + gap-fill operations (Phase 11).
	OfficeRunMacro(h officeHandle, url string) error
	OfficeSignDocument(h officeHandle, url string, cert, key []byte) error
	OfficeGetFilterTypes(h officeHandle) (string, error)
	DocumentInsertCertificate(d documentHandle, cert, key []byte) error
	DocumentAddCertificate(d documentHandle, cert []byte) error
	DocumentGetSignatureState(d documentHandle) (int, error)
	DocumentPaste(d documentHandle, mimeType string, data []byte) error
	DocumentSelectPart(d documentHandle, part int, selected bool) error
	DocumentMoveSelectedParts(d documentHandle, position int, duplicate bool) error
	DocumentRenderFont(d documentHandle, fontName, char string) (buf []byte, w, h int, err error)
}

// clipboardItemInternal mirrors the public lok.ClipboardItem (and
// lokc.ClipboardItem) at the backend-interface seam. The indirection
// lets the interface stay free of the internal/lokc import while the
// public type remains defined in lok/clipboard.go.
type clipboardItemInternal struct {
	MimeType string
	Data     []byte
}

// libraryHandle and officeHandle are opaque across the boundary.
type libraryHandle interface{ libraryBrand() }
type officeHandle interface{ officeBrand() }
type documentHandle interface{ documentBrand() }
