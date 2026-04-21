package lok

import "errors"

// Sentinels usable with errors.Is.
var (
	ErrInstallPathRequired = errors.New("lok: install path is required")
	ErrAlreadyInitialised  = errors.New("lok: already initialised; Close the existing Office first")
	ErrClosed              = errors.New("lok: office is closed")
)

// LOKError wraps an error string returned by LibreOffice itself.
type LOKError struct {
	Op     string // "VersionInfo", "Save", ...
	Detail string // LOK-returned error text
}

func (e *LOKError) Error() string {
	if e.Op == "" {
		return "lok: " + e.Detail
	}
	return "lok: " + e.Op + ": " + e.Detail
}
