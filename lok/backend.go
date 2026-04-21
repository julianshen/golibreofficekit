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
}

// libraryHandle and officeHandle are opaque across the boundary.
type libraryHandle interface{ libraryBrand() }
type officeHandle interface{ officeBrand() }
