//go:build linux || darwin

package lok

import (
	"errors"
	"testing"
)

func TestPasteData(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	if err := doc.PasteData("text/plain", []byte("hello")); err != nil {
		t.Fatal(err)
	}
	if fb.lastPasteMime != "text/plain" {
		t.Errorf("lastPasteMime=%q", fb.lastPasteMime)
	}
	if string(fb.lastPasteData) != "hello" {
		t.Errorf("lastPasteData=%q", fb.lastPasteData)
	}
}

func TestPasteData_Closed(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	doc.Close()
	if err := doc.PasteData("text/plain", nil); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestPasteData_EmptyMime(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	err := doc.PasteData("", []byte("x"))
	var lokErr *LOKError
	if !errors.As(err, &lokErr) || lokErr.Op != "PasteData" {
		t.Errorf("want *LOKError{Op:PasteData}, got %v", err)
	}
}

func TestPasteData_BackendError(t *testing.T) {
	fb := &fakeBackend{pasteErr: ErrUnsupported}
	_, doc := loadFakeDoc(t, fb)
	if err := doc.PasteData("text/plain", nil); !errors.Is(err, ErrUnsupported) {
		t.Errorf("want ErrUnsupported, got %v", err)
	}
}

// PasteData must not shadow the existing Phase-6 Document.Paste
// convenience wrapper for .uno:Paste.
func TestPaste_NotShadowed(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	if err := doc.Paste(); err != nil {
		t.Errorf("Paste(): %v", err)
	}
	if fb.lastUnoCmd != ".uno:Paste" {
		t.Errorf("lastUnoCmd=%q, want .uno:Paste", fb.lastUnoCmd)
	}
}

func TestSelectPart(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	if err := doc.SelectPart(2, true); err != nil {
		t.Fatal(err)
	}
	if fb.lastSelectPart != 2 || !fb.lastSelectSelected {
		t.Errorf("part=%d selected=%v", fb.lastSelectPart, fb.lastSelectSelected)
	}
}

func TestSelectPart_Closed(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	doc.Close()
	if err := doc.SelectPart(0, true); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestMoveSelectedParts(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	if err := doc.MoveSelectedParts(3, true); err != nil {
		t.Fatal(err)
	}
	if fb.lastMovePos != 3 || !fb.lastMoveDup {
		t.Errorf("pos=%d dup=%v", fb.lastMovePos, fb.lastMoveDup)
	}
}

func TestMoveSelectedParts_Closed(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	doc.Close()
	if err := doc.MoveSelectedParts(0, false); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestRenderFont(t *testing.T) {
	fb := &fakeBackend{
		renderFontBuf: []byte{0, 0, 0, 255},
		renderFontW:   1,
		renderFontH:   1,
	}
	_, doc := loadFakeDoc(t, fb)
	buf, w, h, err := doc.RenderFont("Arial", "A")
	if err != nil {
		t.Fatal(err)
	}
	if w != 1 || h != 1 {
		t.Errorf("w=%d h=%d, want 1/1", w, h)
	}
	if len(buf) != 4 {
		t.Errorf("len(buf)=%d, want 4", len(buf))
	}
	if fb.lastRenderFontName != "Arial" || fb.lastRenderFontChar != "A" {
		t.Errorf("font=%q char=%q", fb.lastRenderFontName, fb.lastRenderFontChar)
	}
}

func TestRenderFont_Closed(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	doc.Close()
	if _, _, _, err := doc.RenderFont("Arial", "A"); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestRenderFont_BackendError(t *testing.T) {
	fb := &fakeBackend{renderFontErr: ErrUnsupported}
	_, doc := loadFakeDoc(t, fb)
	if _, _, _, err := doc.RenderFont("Arial", "A"); !errors.Is(err, ErrUnsupported) {
		t.Errorf("want ErrUnsupported, got %v", err)
	}
}
