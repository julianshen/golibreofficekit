//go:build linux || darwin

package lokc

import "testing"

// All five Phase 11 advanced wrappers share two guard rails: zero
// handle → ErrNilOffice / ErrNilDocument; calloc'd fake (pClass==NULL)
// → ErrUnsupported. Beyond that they have no observable Go behaviour
// without a real LOK.

func newFakeOffice(t *testing.T) OfficeHandle {
	t.Helper()
	h := NewFakeOfficeHandle()
	t.Cleanup(func() { FreeFakeOfficeHandle(h) })
	return h
}

func TestOfficeRunMacro_NilSafe(t *testing.T) {
	if err := OfficeRunMacro(OfficeHandle{}, "macro:///x"); err != ErrNilOffice {
		t.Errorf("zero handle: err=%v, want ErrNilOffice", err)
	}
	if err := OfficeRunMacro(newFakeOffice(t), "macro:///x"); err != ErrUnsupported {
		t.Errorf("nil pClass: err=%v, want ErrUnsupported", err)
	}
}

func TestOfficeSignDocument_NilSafe(t *testing.T) {
	if err := OfficeSignDocument(OfficeHandle{}, "url", nil, nil); err != ErrNilOffice {
		t.Errorf("zero handle: err=%v, want ErrNilOffice", err)
	}
	if err := OfficeSignDocument(newFakeOffice(t), "url", []byte("c"), []byte("k")); err != ErrUnsupported {
		t.Errorf("nil pClass: err=%v, want ErrUnsupported", err)
	}
}

func TestDocumentInsertCertificate_NilSafe(t *testing.T) {
	if err := DocumentInsertCertificate(DocumentHandle{}, nil, nil); err != ErrNilDocument {
		t.Errorf("zero handle: err=%v, want ErrNilDocument", err)
	}
	if err := DocumentInsertCertificate(newFakeDoc(t), []byte("c"), []byte("k")); err != ErrUnsupported {
		t.Errorf("nil pClass: err=%v, want ErrUnsupported", err)
	}
}

func TestDocumentAddCertificate_NilSafe(t *testing.T) {
	if err := DocumentAddCertificate(DocumentHandle{}, nil); err != ErrNilDocument {
		t.Errorf("zero handle: err=%v, want ErrNilDocument", err)
	}
	if err := DocumentAddCertificate(newFakeDoc(t), []byte("c")); err != ErrUnsupported {
		t.Errorf("nil pClass: err=%v, want ErrUnsupported", err)
	}
}

func TestDocumentGetSignatureState_NilSafe(t *testing.T) {
	if _, err := DocumentGetSignatureState(DocumentHandle{}); err != ErrNilDocument {
		t.Errorf("zero handle: err=%v, want ErrNilDocument", err)
	}
	if _, err := DocumentGetSignatureState(newFakeDoc(t)); err != ErrUnsupported {
		t.Errorf("nil pClass: err=%v, want ErrUnsupported", err)
	}
}
