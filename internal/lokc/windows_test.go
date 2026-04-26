//go:build linux || darwin

package lokc

import (
	"errors"
	"testing"
)

// Window-event wrappers: zero handle → ErrNilDocument; calloc'd fake
// (pClass == NULL) → ErrUnsupported.

func TestDocumentPostWindowKeyEvent_NilSafe(t *testing.T) {
	if err := DocumentPostWindowKeyEvent(DocumentHandle{}, 1, 0, 'A', 65); err != ErrNilDocument {
		t.Errorf("zero handle: err=%v, want ErrNilDocument", err)
	}
	if err := DocumentPostWindowKeyEvent(newFakeDoc(t), 1, 0, 'A', 65); err != ErrUnsupported {
		t.Errorf("nil pClass: err=%v, want ErrUnsupported", err)
	}
}

func TestDocumentPostWindowMouseEvent_NilSafe(t *testing.T) {
	if err := DocumentPostWindowMouseEvent(DocumentHandle{}, 1, 0, 10, 20, 1, 0, 0); err != ErrNilDocument {
		t.Errorf("zero handle: err=%v, want ErrNilDocument", err)
	}
	if err := DocumentPostWindowMouseEvent(newFakeDoc(t), 1, 0, 10, 20, 1, 0, 0); err != ErrUnsupported {
		t.Errorf("nil pClass: err=%v, want ErrUnsupported", err)
	}
}

func TestDocumentPostWindowGestureEvent_NilSafe(t *testing.T) {
	if err := DocumentPostWindowGestureEvent(DocumentHandle{}, 1, "pan", 0, 0, 0); err != ErrNilDocument {
		t.Errorf("zero handle: err=%v, want ErrNilDocument", err)
	}
	if err := DocumentPostWindowGestureEvent(newFakeDoc(t), 1, "pan", 0, 0, 0); err != ErrUnsupported {
		t.Errorf("nil pClass: err=%v, want ErrUnsupported", err)
	}
}

func TestDocumentPostWindowExtTextInputEvent_NilSafe(t *testing.T) {
	if err := DocumentPostWindowExtTextInputEvent(DocumentHandle{}, 1, 1, "hi"); err != ErrNilDocument {
		t.Errorf("zero handle: err=%v, want ErrNilDocument", err)
	}
	if err := DocumentPostWindowExtTextInputEvent(newFakeDoc(t), 1, 1, "hi"); err != ErrUnsupported {
		t.Errorf("nil pClass: err=%v, want ErrUnsupported", err)
	}
}

func TestDocumentResizeWindow_NilSafe(t *testing.T) {
	if err := DocumentResizeWindow(DocumentHandle{}, 1, 100, 100); err != ErrNilDocument {
		t.Errorf("zero handle: err=%v, want ErrNilDocument", err)
	}
	if err := DocumentResizeWindow(newFakeDoc(t), 1, 100, 100); err != ErrUnsupported {
		t.Errorf("nil pClass: err=%v, want ErrUnsupported", err)
	}
}

func TestDocumentPaintWindow_NilSafe(t *testing.T) {
	buf := make([]byte, 4*10*10)
	if err := DocumentPaintWindow(DocumentHandle{}, 1, buf, 0, 0, 10, 10); err != ErrNilDocument {
		t.Errorf("zero handle: err=%v, want ErrNilDocument", err)
	}
	if err := DocumentPaintWindow(newFakeDoc(t), 1, buf, 0, 0, 10, 10); err != ErrUnsupported {
		t.Errorf("nil pClass: err=%v, want ErrUnsupported", err)
	}
}

func TestDocumentPaintWindowDPI_NilSafe(t *testing.T) {
	buf := make([]byte, 4*10*10)
	if err := DocumentPaintWindowDPI(DocumentHandle{}, 1, buf, 0, 0, 10, 10, 1.0); err != ErrNilDocument {
		t.Errorf("zero handle: err=%v, want ErrNilDocument", err)
	}
	if err := DocumentPaintWindowDPI(newFakeDoc(t), 1, buf, 0, 0, 10, 10, 1.0); err != ErrUnsupported {
		t.Errorf("nil pClass: err=%v, want ErrUnsupported", err)
	}
}

func TestDocumentPaintWindowForView_NilSafe(t *testing.T) {
	buf := make([]byte, 4*10*10)
	if err := DocumentPaintWindowForView(DocumentHandle{}, 1, 0, buf, 0, 0, 10, 10, 1.0); err != ErrNilDocument {
		t.Errorf("zero handle: err=%v, want ErrNilDocument", err)
	}
	if err := DocumentPaintWindowForView(newFakeDoc(t), 1, 0, buf, 0, 0, 10, 10, 1.0); err != ErrUnsupported {
		t.Errorf("nil pClass: err=%v, want ErrUnsupported", err)
	}
}

// checkBGRABuf is exercised before the cgo call; mismatch returns the
// new sentinel regardless of handle validity.
func TestDocumentPaintWindow_BufferSizeMismatch(t *testing.T) {
	tooSmall := make([]byte, 7)
	err := DocumentPaintWindow(newFakeDoc(t), 1, tooSmall, 0, 0, 10, 10)
	if !errors.Is(err, ErrBufferSizeMismatch) {
		t.Errorf("got %v, want ErrBufferSizeMismatch", err)
	}
	err = DocumentPaintWindowDPI(newFakeDoc(t), 1, tooSmall, 0, 0, 10, 10, 1.0)
	if !errors.Is(err, ErrBufferSizeMismatch) {
		t.Errorf("DPI: got %v, want ErrBufferSizeMismatch", err)
	}
	err = DocumentPaintWindowForView(newFakeDoc(t), 1, 0, tooSmall, 0, 0, 10, 10, 1.0)
	if !errors.Is(err, ErrBufferSizeMismatch) {
		t.Errorf("ForView: got %v, want ErrBufferSizeMismatch", err)
	}
}
