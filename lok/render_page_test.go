//go:build linux || darwin

package lok

import (
	"bytes"
	"errors"
	"image/png"
	"testing"
)

// Writer-style: pages are sub-rectangles within a single part.
// PartPageRectangles fixture: two A4-ish pages stacked vertically,
// "0,0,1500,750;0,1000,1500,750". Page 0 is the first half, page 1
// the second.
const writerTwoPagesRects = "0,0,1500,750;0,1000,1500,750"

func TestRenderPage_Writer(t *testing.T) {
	pxW, pxH := 100, 50 // 1500/15, 750/15 at dpiScale=1.0
	bgra := make([]byte, 4*pxW*pxH)
	fb := &fakePaintingBackend{
		fakeBackend: fakeBackend{
			tileMode:  1,
			partRects: writerTwoPagesRects,
			// partsCount = 0 by default → Writer-style branch
		},
		paintBytes: bgra,
	}
	_, doc := loadFakeDocWithBackend(t, fb)
	if err := doc.InitializeForRendering(""); err != nil {
		t.Fatalf("InitializeForRendering: %v", err)
	}

	img, err := doc.RenderPage(1, 1.0)
	if err != nil {
		t.Fatalf("RenderPage(1): %v", err)
	}
	if img.Bounds().Dx() != pxW || img.Bounds().Dy() != pxH {
		t.Errorf("got %dx%d, want %dx%d", img.Bounds().Dx(), img.Bounds().Dy(), pxW, pxH)
	}
	// Painted via PaintTile (Writer path), with the second rect's twips.
	if len(fb.paintCalls) != 1 {
		t.Fatalf("paintCalls=%d, want 1", len(fb.paintCalls))
	}
	pc := fb.paintCalls[0]
	if pc.x != 0 || pc.y != 1000 || pc.w != 1500 || pc.h != 750 {
		t.Errorf("paint twip rect = (%d,%d,%d,%d), want (0,1000,1500,750)", pc.x, pc.y, pc.w, pc.h)
	}
}

func TestRenderPage_Writer_OutOfRange(t *testing.T) {
	fb := &fakeBackend{tileMode: 1, partRects: writerTwoPagesRects}
	_, doc := loadFakeDoc(t, fb)
	if err := doc.InitializeForRendering(""); err != nil {
		t.Fatal(err)
	}
	_, err := doc.RenderPage(5, 1.0)
	var lokErr *LOKError
	if !errors.As(err, &lokErr) || lokErr.Op != "RenderPage" {
		t.Errorf("want *LOKError{Op:RenderPage}, got %v", err)
	}
}

func TestRenderPage_Writer_NoPagesReported(t *testing.T) {
	fb := &fakeBackend{tileMode: 1, partRects: ""}
	_, doc := loadFakeDoc(t, fb)
	if err := doc.InitializeForRendering(""); err != nil {
		t.Fatal(err)
	}
	_, err := doc.RenderPage(0, 1.0)
	var lokErr *LOKError
	if !errors.As(err, &lokErr) || lokErr.Op != "RenderPage" {
		t.Errorf("want *LOKError{Op:RenderPage}, got %v", err)
	}
}

func TestRenderPage_MultiPart(t *testing.T) {
	// Two-sheet Calc: parts=2, active=0, each sheet 1500x750 twips.
	pxW, pxH := 100, 50
	bgra := make([]byte, 4*pxW*pxH)
	fb := &fakePaintingBackend{
		fakeBackend: fakeBackend{
			tileMode:       1,
			partsCount:     2,
			partActive:     0,
			docWidthTwips:  1500,
			docHeightTwips: 750,
		},
		paintBytes: bgra,
	}
	_, doc := loadFakeDocWithBackend(t, fb)
	if err := doc.InitializeForRendering(""); err != nil {
		t.Fatal(err)
	}

	img, err := doc.RenderPage(1, 1.0)
	if err != nil {
		t.Fatalf("RenderPage(1): %v", err)
	}
	if img.Bounds().Dx() != pxW || img.Bounds().Dy() != pxH {
		t.Errorf("got %dx%d, want %dx%d", img.Bounds().Dx(), img.Bounds().Dy(), pxW, pxH)
	}
	// PaintPartTile must have run with part=1.
	if len(fb.partPaintCalls) != 1 {
		t.Fatalf("partPaintCalls=%d, want 1", len(fb.partPaintCalls))
	}
	if got := fb.partPaintCalls[0].part; got != 1 {
		t.Errorf("paint part=%d, want 1", got)
	}
	// Active part must be restored to 0 after RenderPage.
	if got, _ := doc.Part(); got != 0 {
		t.Errorf("active part after RenderPage=%d, want 0 (restored)", got)
	}
}

func TestRenderPage_MultiPart_OutOfRange(t *testing.T) {
	fb := &fakeBackend{tileMode: 1, partsCount: 3}
	_, doc := loadFakeDoc(t, fb)
	if err := doc.InitializeForRendering(""); err != nil {
		t.Fatal(err)
	}
	_, err := doc.RenderPage(10, 1.0)
	var lokErr *LOKError
	if !errors.As(err, &lokErr) || lokErr.Op != "RenderPage" {
		t.Errorf("want *LOKError{Op:RenderPage}, got %v", err)
	}
}

func TestRenderPage_InvalidDpi(t *testing.T) {
	fb := &fakeBackend{tileMode: 1, partRects: writerTwoPagesRects}
	_, doc := loadFakeDoc(t, fb)
	if err := doc.InitializeForRendering(""); err != nil {
		t.Fatal(err)
	}
	for _, dpi := range []float64{0, -1} {
		_, err := doc.RenderPage(0, dpi)
		var lokErr *LOKError
		if !errors.As(err, &lokErr) || lokErr.Op != "RenderPage" {
			t.Errorf("dpi=%v: want *LOKError{Op:RenderPage}, got %v", dpi, err)
		}
	}
}

func TestRenderPage_NotInitialized(t *testing.T) {
	fb := &fakeBackend{tileMode: 1, partRects: writerTwoPagesRects}
	_, doc := loadFakeDoc(t, fb)
	// Skip InitializeForRendering — RenderPage must reject the call.
	_, err := doc.RenderPage(0, 1.0)
	var lokErr *LOKError
	if !errors.As(err, &lokErr) {
		t.Errorf("want *LOKError, got %v", err)
	}
}

func TestRenderPage_Closed(t *testing.T) {
	fb := &fakeBackend{tileMode: 1, partRects: writerTwoPagesRects}
	_, doc := loadFakeDoc(t, fb)
	doc.Close()
	if _, err := doc.RenderPage(0, 1.0); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestRenderPagePNG_Writer(t *testing.T) {
	pxW, pxH := 100, 50
	bgra := make([]byte, 4*pxW*pxH)
	fb := &fakePaintingBackend{
		fakeBackend: fakeBackend{
			tileMode:  1,
			partRects: writerTwoPagesRects,
		},
		paintBytes: bgra,
	}
	_, doc := loadFakeDocWithBackend(t, fb)
	if err := doc.InitializeForRendering(""); err != nil {
		t.Fatal(err)
	}
	pngBytes, err := doc.RenderPagePNG(0, 1.0)
	if err != nil {
		t.Fatalf("RenderPagePNG: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("png.Decode: %v", err)
	}
	if img.Bounds().Dx() != pxW || img.Bounds().Dy() != pxH {
		t.Errorf("decoded PNG: got %dx%d, want %dx%d", img.Bounds().Dx(), img.Bounds().Dy(), pxW, pxH)
	}
}

func TestRenderPagePNG_Closed(t *testing.T) {
	fb := &fakeBackend{tileMode: 1, partRects: writerTwoPagesRects}
	_, doc := loadFakeDoc(t, fb)
	doc.Close()
	if _, err := doc.RenderPagePNG(0, 1.0); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}
