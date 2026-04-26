//go:build linux || darwin

package lokc

import "testing"

// All five Phase 10 command-related wrappers go through the same two
// guard rails: zero handle → ErrNilDocument; non-NULL handle whose
// pClass is NULL (calloc'd fake) → ErrUnsupported. These are
// fire-and-forget from Go's view aside from those error returns.

func TestDocumentGetCommandValues_NilSafe(t *testing.T) {
	if v, err := DocumentGetCommandValues(DocumentHandle{}, ".uno:Save"); err != ErrNilDocument || v != "" {
		t.Errorf("zero handle: v=%q err=%v, want \"\" / ErrNilDocument", v, err)
	}
	if v, err := DocumentGetCommandValues(newFakeDoc(t), ".uno:Save"); err != ErrUnsupported || v != "" {
		t.Errorf("nil pClass: v=%q err=%v, want \"\" / ErrUnsupported", v, err)
	}
}

func TestDocumentCompleteFunction_NilSafe(t *testing.T) {
	if err := DocumentCompleteFunction(DocumentHandle{}, "SUM"); err != ErrNilDocument {
		t.Errorf("zero handle: err=%v, want ErrNilDocument", err)
	}
	if err := DocumentCompleteFunction(newFakeDoc(t), "SUM"); err != ErrUnsupported {
		t.Errorf("nil pClass: err=%v, want ErrUnsupported", err)
	}
}

func TestDocumentSendDialogEvent_NilSafe(t *testing.T) {
	if err := DocumentSendDialogEvent(DocumentHandle{}, 1, "{}"); err != ErrNilDocument {
		t.Errorf("zero handle: err=%v, want ErrNilDocument", err)
	}
	if err := DocumentSendDialogEvent(newFakeDoc(t), 1, "{}"); err != ErrUnsupported {
		t.Errorf("nil pClass: err=%v, want ErrUnsupported", err)
	}
}

func TestDocumentSendContentControlEvent_NilSafe(t *testing.T) {
	if err := DocumentSendContentControlEvent(DocumentHandle{}, "{}"); err != ErrNilDocument {
		t.Errorf("zero handle: err=%v, want ErrNilDocument", err)
	}
	if err := DocumentSendContentControlEvent(newFakeDoc(t), "{}"); err != ErrUnsupported {
		t.Errorf("nil pClass: err=%v, want ErrUnsupported", err)
	}
}

func TestDocumentSendFormFieldEvent_NilSafe(t *testing.T) {
	if err := DocumentSendFormFieldEvent(DocumentHandle{}, "{}"); err != ErrNilDocument {
		t.Errorf("zero handle: err=%v, want ErrNilDocument", err)
	}
	if err := DocumentSendFormFieldEvent(newFakeDoc(t), "{}"); err != ErrUnsupported {
		t.Errorf("nil pClass: err=%v, want ErrUnsupported", err)
	}
}
