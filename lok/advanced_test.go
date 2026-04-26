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

	url := "macro:///Standard.Module1.Main()"
	if err := o.RunMacro(url); err != nil {
		t.Fatal(err)
	}
	if fb.lastMacroURL != url {
		t.Errorf("lastMacroURL=%q, want %q", fb.lastMacroURL, url)
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

func TestRunMacro_BackendError(t *testing.T) {
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
	if string(fb.lastSignCert) != "cert-pem" {
		t.Errorf("lastSignCert=%q", fb.lastSignCert)
	}
	if string(fb.lastSignKey) != "key-pem" {
		t.Errorf("lastSignKey=%q", fb.lastSignKey)
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

func TestSignDocument_BackendError(t *testing.T) {
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
	if _, err := doc.SignatureState(); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestSignatureState_BackendError(t *testing.T) {
	fb := &fakeBackend{signatureStateErr: ErrUnsupported}
	_, doc := loadFakeDoc(t, fb)
	if _, err := doc.SignatureState(); !errors.Is(err, ErrUnsupported) {
		t.Errorf("want ErrUnsupported, got %v", err)
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
	if string(fb.lastInsertKey) != "key" {
		t.Errorf("lastInsertKey=%q", fb.lastInsertKey)
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
		t.Errorf("want *LOKError{Op:InsertCertificate}, got %v", err)
	}
}

func TestInsertCertificate_BackendError(t *testing.T) {
	fb := &fakeBackend{insertCertErr: ErrSignFailed}
	_, doc := loadFakeDoc(t, fb)
	if err := doc.InsertCertificate([]byte("c"), nil); !errors.Is(err, ErrSignFailed) {
		t.Errorf("want ErrSignFailed, got %v", err)
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
		t.Errorf("want *LOKError{Op:AddCertificate}, got %v", err)
	}
}

func TestAddCertificate_BackendError(t *testing.T) {
	fb := &fakeBackend{addCertErr: ErrSignFailed}
	_, doc := loadFakeDoc(t, fb)
	if err := doc.AddCertificate([]byte("c")); !errors.Is(err, ErrSignFailed) {
		t.Errorf("want ErrSignFailed, got %v", err)
	}
}
