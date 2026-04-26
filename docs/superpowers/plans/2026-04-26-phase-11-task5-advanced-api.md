# Phase 11 — Task 5: Public API — advanced.go + tests

**Parent plan:** `docs/superpowers/plans/2026-04-26-phase-11-advanced.md`

**Prerequisites:** Task 4 completed (backend interface extended).

---

## Files created

- `lok/advanced.go`
- `lok/advanced_test.go`

---

## Step 1: Create `lok/advanced.go`

```go
//go:build linux || darwin

package lok

import "fmt"

type SignatureState int

const (
	SignatureNotSigned    SignatureState = 0
	SignatureOK           SignatureState = 1
	SignatureNotValidated SignatureState = 2
	SignatureInvalid      SignatureState = 3
	SignatureUnknown      SignatureState = 4
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

func (d *Document) SignatureState() (SignatureState, error) {
	d.office.mu.Lock()
	defer d.office.mu.Unlock()
	if d.closed {
		return SignatureNotSigned, ErrClosed
	}
	v, err := d.office.be.DocumentGetSignatureState(d.h)
	if err != nil {
		return SignatureNotSigned, err
	}
	return SignatureState(v), nil
}

func (d *Document) InsertCertificate(certificate, privateKey []byte) error {
	d.office.mu.Lock()
	defer d.office.mu.Unlock()
	if d.closed {
		return ErrClosed
	}
	if len(certificate) == 0 {
		return &LOKError{Op: "InsertCertificate", Detail: "certificate is required"}
	}
	return d.office.be.DocumentInsertCertificate(d.h, certificate, privateKey)
}

func (d *Document) AddCertificate(certificate []byte) error {
	d.office.mu.Lock()
	defer d.office.mu.Unlock()
	if d.closed {
		return ErrClosed
	}
	if len(certificate) == 0 {
		return &LOKError{Op: "AddCertificate", Detail: "certificate is required"}
	}
	return d.office.be.DocumentAddCertificate(d.h, certificate)
}
```

- [ ] Create file and verify `go build ./lok`

## Step 2: Create `lok/advanced_test.go`

```go
//go:build linux || darwin

package lok

import (
	"errors"
	"testing"
)

func TestRunMacro(t *testing.T) {
	fb := &fakeBackend{}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()

	if err := o.RunMacro("macro:///Standard.Module1.Main()"); err != nil {
		t.Fatal(err)
	}
	if fb.lastMacroURL != "macro:///Standard.Module1.Main()" {
		t.Errorf("lastMacroURL=%q", fb.lastMacroURL)
	}
}

func TestRunMacro_Closed(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	o.Close()
	if err := o.RunMacro("macro:///x"); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestRunMacro_EmptyURL(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	err := o.RunMacro("")
	var lokErr *LOKError
	if !errors.As(err, &lokErr) || lokErr.Op != "RunMacro" {
		t.Errorf("want *LOKError{Op:RunMacro}, got %v", err)
	}
}

func TestRunMacro_Failed(t *testing.T) {
	fb := &fakeBackend{macroErr: ErrMacroFailed}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	if err := o.RunMacro("macro:///x"); !errors.Is(err, ErrMacroFailed) {
		t.Errorf("want ErrMacroFailed, got %v", err)
	}
}

func TestSignDocument(t *testing.T) {
	fb := &fakeBackend{}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()

	cert := []byte("cert-pem")
	key := []byte("key-pem")
	if err := o.SignDocument("file:///tmp/x.odt", cert, key); err != nil {
		t.Fatal(err)
	}
	if fb.lastSignURL != "file:///tmp/x.odt" {
		t.Errorf("lastSignURL=%q", fb.lastSignURL)
	}
}

func TestSignDocument_Closed(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	o.Close()
	if err := o.SignDocument("url", []byte("c"), []byte("k")); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestSignDocument_EmptyURL(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	err := o.SignDocument("", []byte("c"), nil)
	var lokErr *LOKError
	if !errors.As(err, &lokErr) || lokErr.Op != "SignDocument" {
		t.Errorf("want *LOKError{Op:SignDocument}, got %v", err)
	}
}

func TestSignDocument_EmptyCert(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	err := o.SignDocument("url", nil, []byte("k"))
	var lokErr *LOKError
	if !errors.As(err, &lokErr) || lokErr.Op != "SignDocument" {
		t.Errorf("want *LOKError{Op:SignDocument}, got %v", err)
	}
}

func TestSignDocument_Failed(t *testing.T) {
	fb := &fakeBackend{signErr: ErrSignFailed}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	if err := o.SignDocument("url", []byte("c"), nil); !errors.Is(err, ErrSignFailed) {
		t.Errorf("want ErrSignFailed, got %v", err)
	}
}

func TestSignatureState(t *testing.T) {
	fb := &fakeBackend{signatureStateResult: 1}
	_, doc := loadFakeDoc(t, fb)
	s, err := doc.SignatureState()
	if err != nil {
		t.Fatal(err)
	}
	if s != SignatureOK {
		t.Errorf("got %v, want SignatureOK", s)
	}
}

func TestSignatureState_Closed(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	doc.Close()
	_, err := doc.SignatureState()
	if !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestSignatureState_String(t *testing.T) {
	cases := map[SignatureState]string{
		SignatureNotSigned:    "NotSigned",
		SignatureOK:           "OK",
		SignatureNotValidated: "NotValidated",
		SignatureInvalid:      "Invalid",
		SignatureUnknown:      "Unknown",
		SignatureState(99):    "SignatureState(99)",
	}
	for s, want := range cases {
		if got := s.String(); got != want {
			t.Errorf("SignatureState(%d).String()=%q, want %q", s, got, want)
		}
	}
}

func TestInsertCertificate(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	cert := []byte("cert")
	key := []byte("key")
	if err := doc.InsertCertificate(cert, key); err != nil {
		t.Fatal(err)
	}
	if string(fb.lastInsertCert) != "cert" {
		t.Errorf("lastInsertCert=%q", fb.lastInsertCert)
	}
}

func TestInsertCertificate_Closed(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	doc.Close()
	if err := doc.InsertCertificate([]byte("c"), nil); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestInsertCertificate_EmptyCert(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	err := doc.InsertCertificate(nil, []byte("k"))
	var lokErr *LOKError
	if !errors.As(err, &lokErr) || lokErr.Op != "InsertCertificate" {
		t.Errorf("want *LOKError, got %v", err)
	}
}

func TestAddCertificate(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	if err := doc.AddCertificate([]byte("cert")); err != nil {
		t.Fatal(err)
	}
	if string(fb.lastAddCert) != "cert" {
		t.Errorf("lastAddCert=%q", fb.lastAddCert)
	}
}

func TestAddCertificate_Closed(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	doc.Close()
	if err := doc.AddCertificate([]byte("c")); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestAddCertificate_EmptyCert(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	err := doc.AddCertificate(nil)
	var lokErr *LOKError
	if !errors.As(err, &lokErr) || lokErr.Op != "AddCertificate" {
		t.Errorf("want *LOKError, got %v", err)
	}
}
```

- [ ] Create file and verify `go test ./lok -run 'TestRunMacro|TestSignDocument|TestSignatureState|TestInsertCertificate|TestAddCertificate' -v`

## Step 3: Full build + test

```bash
go build ./...
go test ./lok -race -count=1
```

- [ ] Builds and tests pass
