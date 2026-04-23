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
	DocumentDestroyView(d documentHandle, id int)
	DocumentSetView(d documentHandle, id int)
	DocumentGetView(d documentHandle) int
	DocumentGetViewsCount(d documentHandle) int
	DocumentGetViewIds(d documentHandle) (ids []int, ok bool)
	DocumentSetViewLanguage(d documentHandle, id int, lang string)
	DocumentSetViewReadOnly(d documentHandle, id int, readOnly bool)
	DocumentSetAccessibilityState(d documentHandle, id int, enabled bool)
	DocumentSetViewTimezone(d documentHandle, id int, tz string)

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
}

// libraryHandle and officeHandle are opaque across the boundary.
type libraryHandle interface{ libraryBrand() }
type officeHandle interface{ officeBrand() }
type documentHandle interface{ documentBrand() }
