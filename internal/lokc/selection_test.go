//go:build linux || darwin

package lokc

import "testing"

// newFakeDoc wraps NewFakeDocumentHandle + FreeFakeDocumentHandle in
// a t.Cleanup so each test can say `d := newFakeDoc(t)` in one line.
// Uses the existing helpers in document_test_helper.go, which yield
// a calloc'd LibreOfficeKitDocument with pClass == NULL.
func newFakeDoc(t *testing.T) DocumentHandle {
	t.Helper()
	d := NewFakeDocumentHandle()
	t.Cleanup(func() { FreeFakeDocumentHandle(d) })
	return d
}

// All four setters are fire-and-forget; the only observable behaviour
// from Go is that a zero / fake-pClass handle does not crash.

func TestDocumentSetTextSelection_NilSafe(t *testing.T) {
	DocumentSetTextSelection(DocumentHandle{}, 0, 0, 0) // zero handle
	h := newFakeDoc(t)
	DocumentSetTextSelection(h, 1, 100, 200) // pClass == NULL
}

func TestDocumentResetSelection_NilSafe(t *testing.T) {
	DocumentResetSelection(DocumentHandle{})
	DocumentResetSelection(newFakeDoc(t))
}

func TestDocumentSetGraphicSelection_NilSafe(t *testing.T) {
	DocumentSetGraphicSelection(DocumentHandle{}, 0, 0, 0)
	DocumentSetGraphicSelection(newFakeDoc(t), 1, 10, 20)
}

func TestDocumentSetBlockedCommandList_NilSafe(t *testing.T) {
	DocumentSetBlockedCommandList(DocumentHandle{}, 0, ".uno:Save")
	DocumentSetBlockedCommandList(newFakeDoc(t), 1, ".uno:Save,.uno:SaveAs")
}

func TestDocumentGetTextSelection_NilSafe(t *testing.T) {
	// Zero handle and fake-pClass both surface ErrUnsupported and
	// empty strings — callers can distinguish "no selection" (no
	// error) from "vtable slot missing" (ErrUnsupported).
	text, mime, err := DocumentGetTextSelection(DocumentHandle{}, "text/plain")
	if err != ErrUnsupported || text != "" || mime != "" {
		t.Errorf("zero handle: got (%q, %q, %v); want (\"\", \"\", ErrUnsupported)", text, mime, err)
	}
	text, mime, err = DocumentGetTextSelection(newFakeDoc(t), "text/plain")
	if err != ErrUnsupported || text != "" || mime != "" {
		t.Errorf("nil pClass: got (%q, %q, %v); want (\"\", \"\", ErrUnsupported)", text, mime, err)
	}
}

func TestDocumentGetSelectionType_NilSafe(t *testing.T) {
	if v, err := DocumentGetSelectionType(DocumentHandle{}); err != ErrUnsupported || v != 0 {
		t.Errorf("zero handle: got (%d, %v); want (0, ErrUnsupported)", v, err)
	}
	if v, err := DocumentGetSelectionType(newFakeDoc(t)); err != ErrUnsupported || v != 0 {
		t.Errorf("nil pClass: got (%d, %v); want (0, ErrUnsupported)", v, err)
	}
}

func TestDocumentGetSelectionTypeAndText_UnsupportedOnNilSlot(t *testing.T) {
	// Zero handle maps to Unsupported (the slot is effectively NULL).
	kind, text, mime, err := DocumentGetSelectionTypeAndText(DocumentHandle{}, "text/plain")
	if err != ErrUnsupported {
		t.Errorf("zero handle: err=%v, want ErrUnsupported", err)
	}
	if kind != -1 || text != "" || mime != "" {
		t.Errorf("zero handle: non-zero out values (%d, %q, %q)", kind, text, mime)
	}
	kind, text, mime, err = DocumentGetSelectionTypeAndText(newFakeDoc(t), "text/plain")
	if err != ErrUnsupported {
		t.Errorf("nil pClass: err=%v, want ErrUnsupported", err)
	}
	if kind != -1 || text != "" || mime != "" {
		t.Errorf("nil pClass: non-zero out values (%d, %q, %q)", kind, text, mime)
	}
}
