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
	// PR B: setters now report ErrUnsupported instead of silently
	// no-opping when the vtable slot is NULL (or the handle is zero).
	if err := DocumentSetPart(d, 0); err != ErrUnsupported {
		t.Errorf("SetPart on nil: err=%v, want ErrUnsupported", err)
	}
	if err := DocumentSetPartMode(d, 0); err != ErrUnsupported {
		t.Errorf("SetPartMode on nil: err=%v, want ErrUnsupported", err)
	}
	if err := DocumentSetOutlineState(d, false, 0, 0, false); err != ErrUnsupported {
		t.Errorf("SetOutlineState on nil: err=%v, want ErrUnsupported", err)
	}
}

func TestDocumentPart_FakeHandle_SafeNoOps(t *testing.T) {
	d := NewFakeDocumentHandle()
	t.Cleanup(func() { FreeFakeDocumentHandle(d) })

	if got := DocumentGetParts(d); got != -1 {
		t.Errorf("GetParts on fake: got %d, want -1", got)
	}
	if got := DocumentGetPart(d); got != -1 {
		t.Errorf("GetPart on fake: got %d, want -1", got)
	}
	if got := DocumentGetPartName(d, 0); got != "" {
		t.Errorf("GetPartName on fake: got %q, want empty", got)
	}
	if got := DocumentGetPartHash(d, 0); got != "" {
		t.Errorf("GetPartHash on fake: got %q, want empty", got)
	}
	if got := DocumentGetPartInfo(d, 0); got != "" {
		t.Errorf("GetPartInfo on fake: got %q, want empty", got)
	}
	if w, h := DocumentGetDocumentSize(d); w != 0 || h != 0 {
		t.Errorf("GetDocumentSize on fake: got (%d, %d), want (0, 0)", w, h)
	}
	if got := DocumentGetPartPageRectangles(d); got != "" {
		t.Errorf("GetPartPageRectangles on fake: got %q, want empty", got)
	}
	if err := DocumentSetPart(d, 0); err != ErrUnsupported {
		t.Errorf("SetPart on fake: err=%v, want ErrUnsupported", err)
	}
	if err := DocumentSetPartMode(d, 0); err != ErrUnsupported {
		t.Errorf("SetPartMode on fake: err=%v, want ErrUnsupported", err)
	}
	if err := DocumentSetOutlineState(d, false, 0, 0, false); err != ErrUnsupported {
		t.Errorf("SetOutlineState on fake: err=%v, want ErrUnsupported", err)
	}
}
