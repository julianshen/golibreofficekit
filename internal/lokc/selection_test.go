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
	// Zero handle and fake-pClass both yield empty strings without crashing.
	if text, mime := DocumentGetTextSelection(DocumentHandle{}, "text/plain"); text != "" || mime != "" {
		t.Errorf("zero handle: got (%q, %q), want empty strings", text, mime)
	}
	h := newFakeDoc(t)
	if text, mime := DocumentGetTextSelection(h, "text/plain"); text != "" || mime != "" {
		t.Errorf("nil pClass: got (%q, %q), want empty strings", text, mime)
	}
}
