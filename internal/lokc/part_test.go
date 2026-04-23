//go:build (linux && (amd64 || arm64)) || (darwin && (amd64 || arm64))

package lokc

import "testing"

func TestDocumentPart_NilHandleAreNoOps(t *testing.T) {
	var d DocumentHandle
	if got := DocumentGetParts(d); got != -1 {
		t.Errorf("GetParts on nil: got %d, want -1", got)
	}
	if got := DocumentGetPart(d); got != -1 {
		t.Errorf("GetPart on nil: got %d, want -1", got)
	}
	if got := DocumentGetPartName(d, 0); got != "" {
		t.Errorf("GetPartName on nil: got %q, want empty", got)
	}
	if got := DocumentGetPartHash(d, 0); got != "" {
		t.Errorf("GetPartHash on nil: got %q, want empty", got)
	}
	if got := DocumentGetPartInfo(d, 0); got != "" {
		t.Errorf("GetPartInfo on nil: got %q, want empty", got)
	}
	if w, h := DocumentGetDocumentSize(d); w != 0 || h != 0 {
		t.Errorf("GetDocumentSize on nil: got (%d, %d), want (0, 0)", w, h)
	}
	if got := DocumentGetPartPageRectangles(d); got != "" {
		t.Errorf("GetPartPageRectangles on nil: got %q, want empty", got)
	}
	DocumentSetPart(d, 0)
	DocumentSetPartMode(d, 0)
	DocumentSetOutlineState(d, false, 0, 0, false)
}

func TestDocumentPart_FakeHandle_SafeNoOps(t *testing.T) {
	d := NewFakeDocumentHandle()
	t.Cleanup(func() { FreeFakeDocumentHandle(d) })

	DocumentGetParts(d)
	DocumentGetPart(d)
	DocumentGetPartName(d, 0)
	DocumentGetPartHash(d, 0)
	DocumentGetPartInfo(d, 0)
	DocumentGetDocumentSize(d)
	DocumentGetPartPageRectangles(d)
	DocumentSetPart(d, 0)
	DocumentSetPartMode(d, 0)
	DocumentSetOutlineState(d, false, 0, 0, false)
}
