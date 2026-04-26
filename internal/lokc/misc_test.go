//go:build linux || darwin

package lokc

import "testing"

func TestOfficeGetFilterTypes_NilSafe(t *testing.T) {
	if v, err := OfficeGetFilterTypes(OfficeHandle{}); err != ErrNilOffice || v != "" {
		t.Errorf("zero handle: v=%q err=%v, want \"\" / ErrNilOffice", v, err)
	}
	if v, err := OfficeGetFilterTypes(newFakeOffice(t)); err != ErrUnsupported || v != "" {
		t.Errorf("nil pClass: v=%q err=%v, want \"\" / ErrUnsupported", v, err)
	}
}

func TestDocumentPaste_NilSafe(t *testing.T) {
	if err := DocumentPaste(DocumentHandle{}, "text/plain", nil); err != ErrNilDocument {
		t.Errorf("zero handle: err=%v, want ErrNilDocument", err)
	}
	if err := DocumentPaste(newFakeDoc(t), "text/plain", []byte("hi")); err != ErrUnsupported {
		t.Errorf("nil pClass: err=%v, want ErrUnsupported", err)
	}
}

func TestDocumentSelectPart_NilSafe(t *testing.T) {
	// LOK's selectPart returns void. The shim still distinguishes
	// "vtable slot missing" from "operation succeeded" so callers on
	// old LO builds aren't fooled by a silent no-op.
	if err := DocumentSelectPart(DocumentHandle{}, 0, true); err != ErrNilDocument {
		t.Errorf("zero handle: err=%v, want ErrNilDocument", err)
	}
	if err := DocumentSelectPart(newFakeDoc(t), 0, true); err != ErrUnsupported {
		t.Errorf("nil pClass: err=%v, want ErrUnsupported", err)
	}
}

func TestDocumentMoveSelectedParts_NilSafe(t *testing.T) {
	if err := DocumentMoveSelectedParts(DocumentHandle{}, 0, false); err != ErrNilDocument {
		t.Errorf("zero handle: err=%v, want ErrNilDocument", err)
	}
	if err := DocumentMoveSelectedParts(newFakeDoc(t), 1, true); err != ErrUnsupported {
		t.Errorf("nil pClass: err=%v, want ErrUnsupported", err)
	}
}

func TestDocumentRenderFont_NilSafe(t *testing.T) {
	if _, _, _, err := DocumentRenderFont(DocumentHandle{}, "Arial", "A"); err != ErrNilDocument {
		t.Errorf("zero handle: err=%v, want ErrNilDocument", err)
	}
	if buf, w, h, err := DocumentRenderFont(newFakeDoc(t), "Arial", "A"); err != ErrUnsupported || buf != nil || w != 0 || h != 0 {
		t.Errorf("nil pClass: buf=%v w=%d h=%d err=%v, want nil / 0 / 0 / ErrUnsupported", buf, w, h, err)
	}
}
