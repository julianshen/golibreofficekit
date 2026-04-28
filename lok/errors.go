package lok

import "errors"

// Sentinels usable with errors.Is.
var (
	ErrInstallPathRequired = errors.New("lok: install path is required")
	ErrAlreadyInitialised  = errors.New("lok: already initialised; Close the existing Office first")
	ErrClosed              = errors.New("lok: office is closed")
	ErrPathRequired        = errors.New("lok: document path is required")
	ErrInvalidOption       = errors.New("lok: invalid load option (contains reserved character)")
	ErrViewCreateFailed    = errors.New("lok: view creation failed")
	ErrUnsupported         = errors.New("lok: operation not supported by this LibreOfficeKit build")
	ErrMacroFailed         = errors.New("lok: macro execution failed")
	ErrSignFailed          = errors.New("lok: document signing failed")
	ErrPasteFailed         = errors.New("lok: paste rejected by LibreOffice")
	ErrNoValue             = errors.New("lok: LibreOffice produced no value for this call")
)

// LOKError wraps an error string returned by LibreOffice itself.
// When an internal error (e.g. from internal/lokc) is wrapped at the
// public-API boundary, err preserves the original so errors.Is against
// unexported sentinels and errors.As against the original concrete
// type continue to work.
type LOKError struct {
	Op     string // "VersionInfo", "Save", ...
	Detail string // LOK-returned error text
	err    error  // wrapped internal error, may be nil
}

func (e *LOKError) Error() string {
	if e.Op == "" {
		return "lok: " + e.Detail
	}
	return "lok: " + e.Op + ": " + e.Detail
}

// Unwrap returns the wrapped internal error so errors.Is / errors.As
// can traverse the chain.
func (e *LOKError) Unwrap() error { return e.err }

// wrapErr builds an *LOKError that carries both a human-readable
// operation tag and the original error for errors.Is/As traversal.
func wrapErr(op string, err error) error {
	return &LOKError{Op: op, Detail: err.Error(), err: err}
}

// wrapLOErr builds an *LOKError that prefers LibreOffice's own error
// string (read via the backend's OfficeGetError) as Detail, falling
// back to the underlying error's message when LO has nothing pending.
// The original err is preserved in the wrap chain so errors.Is /
// errors.As keep working.
//
// Callers must hold o.mu (the same critical section that performed the
// failing operation) so the OfficeGetError read sees the error LO
// stashed for THIS call, not one from a concurrent operation.
func wrapLOErr(op string, o *Office, err error) error {
	detail := o.be.OfficeGetError(o.h)
	if detail == "" {
		detail = err.Error()
	}
	return &LOKError{Op: op, Detail: detail, err: err}
}
