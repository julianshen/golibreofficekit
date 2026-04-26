//go:build linux || darwin

package lok

import "fmt"

// SignatureState mirrors LibreOffice's SignatureState enum
// (defined in svl/sigstruct.hxx in the LO source tree). LOK has no
// vended header for this enum; values are tracked by hand against
// the LO source.
//
// Out-of-range values are surfaced as SignatureState(N) by String()
// and rejected by Valid() — a future LO release that grows the enum
// will surface here as an unknown ordinal rather than silently
// pretending to be a known state.
type SignatureState int

const (
	SignatureUnknown      SignatureState = 0 // UNKNOWN
	SignatureOK           SignatureState = 1 // OK
	SignatureBroken       SignatureState = 2 // BROKEN
	SignatureInvalid      SignatureState = 3 // INVALID
	SignatureNotValidated SignatureState = 4 // NOTVALIDATED
	SignaturePartialOK    SignatureState = 5 // PARTIAL_OK
	SignatureNoSignatures SignatureState = 6 // NOSIGNATURES
)

// Valid reports whether s names one of the LO enum values this binding
// knows about. A future LO version that grows the enum will surface
// here as Valid()==false; the caller can still use String() to log
// the unknown ordinal.
func (s SignatureState) Valid() bool {
	return s >= SignatureUnknown && s <= SignatureNoSignatures
}

func (s SignatureState) String() string {
	switch s {
	case SignatureUnknown:
		return "Unknown"
	case SignatureOK:
		return "OK"
	case SignatureBroken:
		return "Broken"
	case SignatureInvalid:
		return "Invalid"
	case SignatureNotValidated:
		return "NotValidated"
	case SignaturePartialOK:
		return "PartialOK"
	case SignatureNoSignatures:
		return "NoSignatures"
	default:
		return fmt.Sprintf("SignatureState(%d)", int(s))
	}
}

// RunMacro executes the LibreOffice macro identified by url.
// url is a UNO URL (e.g. "macro:///Standard.Module1.Main()").
// Returns ErrMacroFailed if LOK rejects the macro and ErrUnsupported
// when the LO build does not expose runMacro.
func (o *Office) RunMacro(url string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return ErrClosed
	}
	if url == "" {
		return &LOKError{Op: "RunMacro", Detail: "url is required"}
	}
	return o.be.OfficeRunMacro(o.h, url)
}

// SignDocument signs the document at docURL with the given PEM-encoded
// certificate and private key. LOK opens its own loader from docURL,
// so this operates on the on-disk file independently of any in-memory
// document — saving any open copy of the same path before calling is
// the caller's responsibility.
//
// Returns ErrSignFailed when LOK reports failure and ErrUnsupported
// when the vtable slot is missing. Empty docURL, empty certificate,
// or empty privateKey return *LOKError without invoking LOK; LO's
// signing path dereferences the key buffer without a NULL check.
func (o *Office) SignDocument(docURL string, certificate, privateKey []byte) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return ErrClosed
	}
	if docURL == "" {
		return &LOKError{Op: "SignDocument", Detail: "docURL is required"}
	}
	if len(certificate) == 0 {
		return &LOKError{Op: "SignDocument", Detail: "certificate is required"}
	}
	if len(privateKey) == 0 {
		return &LOKError{Op: "SignDocument", Detail: "privateKey is required"}
	}
	return o.be.OfficeSignDocument(o.h, docURL, certificate, privateKey)
}

// SignatureState returns the document's current cryptographic
// signature state. Returns ErrClosed on a closed document and
// ErrUnsupported when the LO build does not expose getSignatureState.
func (d *Document) SignatureState() (SignatureState, error) {
	unlock, err := d.guard()
	if err != nil {
		return SignatureUnknown, err
	}
	defer unlock()
	v, err := d.office.be.DocumentGetSignatureState(d.h)
	if err != nil {
		return SignatureUnknown, err
	}
	return SignatureState(v), nil
}

// InsertCertificate adds a self-signed certificate together with its
// private key to the document's certificate store, so the document
// can be signed with that certificate next. Use AddCertificate
// (no private key) for adding a trusted CA used to verify signatures.
//
// Empty certificate or empty privateKey return *LOKError without
// invoking LOK; LO's signing path dereferences the key buffer without
// a NULL check.
func (d *Document) InsertCertificate(certificate, privateKey []byte) error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	if len(certificate) == 0 {
		return &LOKError{Op: "InsertCertificate", Detail: "certificate is required"}
	}
	if len(privateKey) == 0 {
		return &LOKError{Op: "InsertCertificate", Detail: "privateKey is required"}
	}
	return d.office.be.DocumentInsertCertificate(d.h, certificate, privateKey)
}

// AddCertificate adds a trusted certificate (no private key) to the
// document's certificate store. Used to register a CA whose signatures
// are considered valid; for adding a signing certificate (which needs
// a private key), use InsertCertificate.
func (d *Document) AddCertificate(certificate []byte) error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	if len(certificate) == 0 {
		return &LOKError{Op: "AddCertificate", Detail: "certificate is required"}
	}
	return d.office.be.DocumentAddCertificate(d.h, certificate)
}
