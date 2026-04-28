//go:build (linux && (amd64 || arm64)) || (darwin && (amd64 || arm64))

package lokc

import "testing"

// TestDocumentInput_NilHandle_ReturnsUnsupported asserts that a zero
// handle and a fake-pClass handle both surface ErrUnsupported, mirroring
// the selection.go pattern. Replaces the old "no-op safety" test —
// silent success indistinguishable from a vtable-missing build was
// the audit defect we are fixing.
func TestDocumentInput_NilHandle_ReturnsUnsupported(t *testing.T) {
	var d DocumentHandle
	if err := DocumentPostKeyEvent(d, 0, 'a', 0); err != ErrUnsupported {
		t.Errorf("PostKeyEvent on nil: err=%v, want ErrUnsupported", err)
	}
	if err := DocumentPostMouseEvent(d, 0, 100, 100, 1, 1, 0); err != ErrUnsupported {
		t.Errorf("PostMouseEvent on nil: err=%v, want ErrUnsupported", err)
	}
	if err := DocumentPostUnoCommand(d, ".uno:Bold", "", false); err != ErrUnsupported {
		t.Errorf("PostUnoCommand on nil: err=%v, want ErrUnsupported", err)
	}
}

func TestDocumentInput_FakeHandle_ReturnsUnsupported(t *testing.T) {
	d := NewFakeDocumentHandle()
	t.Cleanup(func() { FreeFakeDocumentHandle(d) })
	if err := DocumentPostKeyEvent(d, 0, 'a', 0); err != ErrUnsupported {
		t.Errorf("PostKeyEvent on fake: err=%v, want ErrUnsupported", err)
	}
	if err := DocumentPostMouseEvent(d, 0, 100, 100, 1, 1, 0); err != ErrUnsupported {
		t.Errorf("PostMouseEvent on fake: err=%v, want ErrUnsupported", err)
	}
	if err := DocumentPostUnoCommand(d, ".uno:Bold", "", false); err != ErrUnsupported {
		t.Errorf("PostUnoCommand on fake: err=%v, want ErrUnsupported", err)
	}
}
