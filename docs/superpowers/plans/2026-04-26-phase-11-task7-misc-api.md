# Phase 11 — Task 7: Public API — misc.go + tests

**Parent plan:** `docs/superpowers/plans/2026-04-26-phase-11-advanced.md`

**Prerequisites:** Task 4 completed (backend interface extended).

---

## Files created

- `lok/misc.go`
- `lok/misc_test.go`

---

## Step 1: Create `lok/misc.go`

```go
//go:build linux || darwin

package lok

// Paste inserts data from the clipboard into the document.
// mimeType describes the data format; data is the raw payload.
func (d *Document) Paste(mimeType string, data []byte) error {
	d.office.mu.Lock()
	defer d.office.mu.Unlock()
	if d.closed {
		return ErrClosed
	}
	if mimeType == "" {
		return &LOKError{Op: "Paste", Detail: "mimeType is required"}
	}
	return d.office.be.DocumentPaste(d.h, mimeType, data)
}

// SelectPart selects or deselects a part (slide/sheet).
func (d *Document) SelectPart(part int, select_ bool) error {
	d.office.mu.Lock()
	defer d.office.mu.Unlock()
	if d.closed {
		return ErrClosed
	}
	return d.office.be.DocumentSelectPart(d.h, part, select_)
}

// MoveSelectedParts moves the selected parts to a new position.
// If duplicate is true, the parts are copied instead of moved.
func (d *Document) MoveSelectedParts(position int, duplicate bool) error {
	d.office.mu.Lock()
	defer d.office.mu.Unlock()
	if d.closed {
		return ErrClosed
	}
	return d.office.be.DocumentMoveSelectedParts(d.h, position, duplicate)
}

// RenderFont renders a glyph of the named font and returns the bitmap
// in premultiplied BGRA, along with the pixel dimensions.
// If fontName is empty, the default font is rendered.
func (d *Document) RenderFont(fontName, char string) ([]byte, int, int, error) {
	d.office.mu.Lock()
	defer d.office.mu.Unlock()
	if d.closed {
		return nil, 0, 0, ErrClosed
	}
	return d.office.be.DocumentRenderFont(d.h, fontName, char)
}
```

- [ ] Create file and verify `go build ./lok`

## Step 2: Create `lok/misc_test.go`

```go
//go:build linux || darwin

package lok

import (
	"errors"
	"testing"
)

func TestPaste(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	if err := doc.Paste("text/plain", []byte("hello")); err != nil {
		t.Fatal(err)
	}
	if fb.lastPasteMime != "text/plain" {
		t.Errorf("lastPasteMime=%q", fb.lastPasteMime)
	}
	if string(fb.lastPasteData) != "hello" {
		t.Errorf("lastPasteData=%q", fb.lastPasteData)
	}
}

func TestPaste_Closed(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	doc.Close()
	if err := doc.Paste("text/plain", nil); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestPaste_EmptyMime(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	err := doc.Paste("", []byte("x"))
	var lokErr *LOKError
	if !errors.As(err, &lokErr) || lokErr.Op != "Paste" {
		t.Errorf("want *LOKError{Op:Paste}, got %v", err)
	}
}

func TestPaste_BackendError(t *testing.T) {
	fb := &fakeBackend{pasteErr: ErrUnsupported}
	_, doc := loadFakeDoc(t, fb)
	if err := doc.Paste("text/plain", nil); !errors.Is(err, ErrUnsupported) {
		t.Errorf("want ErrUnsupported, got %v", err)
	}
}

func TestSelectPart(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	if err := doc.SelectPart(2, true); err != nil {
		t.Fatal(err)
	}
	if fb.lastSelectPart != 2 || !fb.lastSelectSelect {
		t.Errorf("part=%d select=%v", fb.lastSelectPart, fb.lastSelectSelect)
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
	fb := &fakeBackend{renderFontBuf: []byte{0, 0, 0, 255}, renderFontW: 1, renderFontH: 1}
	_, doc := loadFakeDoc(t, fb)
	buf, w, h, err := doc.RenderFont("Arial", "A")
	if err != nil {
		t.Fatal(err)
	}
	if w != 1 || h != 1 {
		t.Errorf("w=%d h=%d", w, h)
	}
	if len(buf) != 4 {
		t.Errorf("len(buf)=%d", len(buf))
	}
	if fb.lastRenderFontName != "Arial" || fb.lastRenderFontChar != "A" {
		t.Errorf("font=%q char=%q", fb.lastRenderFontName, fb.lastRenderFontChar)
	}
}

func TestRenderFont_Closed(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	doc.Close()
	_, _, _, err := doc.RenderFont("Arial", "A")
	if !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestRenderFont_BackendError(t *testing.T) {
	fb := &fakeBackend{renderFontErr: ErrUnsupported}
	_, doc := loadFakeDoc(t, fb)
	_, _, _, err := doc.RenderFont("Arial", "A")
	if !errors.Is(err, ErrUnsupported) {
		t.Errorf("want ErrUnsupported, got %v", err)
	}
}
```

- [ ] Create file and verify `go test ./lok -run 'TestPaste|TestSelectPart|TestMoveSelectedParts|TestRenderFont' -v`

## Step 3: Full build + test

```bash
go build ./...
go test ./lok -race -count=1
```

- [ ] Builds and tests pass
