//go:build linux || darwin

package lok

import "fmt"

// SignatureState mirrors LOK's getSignatureState integer.
// Values follow LibreOffice's DocumentState enum.
type SignatureState int

const (
	SignatureNotSigned    SignatureState = 0 // NEITHER
	SignatureOK           SignatureState = 1 // signature valid
	SignatureNotValidated SignatureState = 2 // present but not validated
	SignatureInvalid      SignatureState = 3 // signature invalid
	SignatureUnknown      SignatureState = 4 // unknown state
)

func (s SignatureState) String() string {
	switch s {
	case SignatureNotSigned:
		return "NotSigned"
	case SignatureOK:
		return "OK"
	case SignatureNotValidated:
		return "NotValidated"
	case SignatureInvalid:
		return "Invalid"
	case SignatureUnknown:
		return "Unknown"
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
// certificate and private key. The document must not be open by this
// Office instance — LOK signs on disk, not in memory.
// Returns ErrSignFailed when LOK reports failure and ErrUnsupported
// when the vtable slot is missing.
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
	return o.be.OfficeSignDocument(o.h, docURL, certificate, privateKey)
}

// SignatureState returns the document's current cryptographic
// signature state. Returns ErrClosed on a closed document and
// ErrUnsupported when the LO build does not expose getSignatureState.
func (d *Document) SignatureState() (SignatureState, error) {
	unlock, err := d.guard()
	if err != nil {
		return SignatureNotSigned, err
	}
	defer unlock()
	v, err := d.office.be.DocumentGetSignatureState(d.h)
	if err != nil {
		return SignatureNotSigned, err
	}
	return SignatureState(v), nil
}

// InsertCertificate inserts a certificate and (optional) private key
// into the document's certificate store. Used before signing to ensure
// the certificate is available. Empty certificate returns *LOKError.
func (d *Document) InsertCertificate(certificate, privateKey []byte) error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	if len(certificate) == 0 {
		return &LOKError{Op: "InsertCertificate", Detail: "certificate is required"}
	}
	return d.office.be.DocumentInsertCertificate(d.h, certificate, privateKey)
}

// AddCertificate adds a certificate (without private key) to the
// document's certificate store.
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
