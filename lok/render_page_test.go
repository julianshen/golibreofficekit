//go:build linux || darwin

package lok

import (
	"bytes"
	"errors"
	"image/png"
	"testing"
)

// Writer-style: pages are sub-rectangles within a single part.
// PartPageRectangles fixture: two synthetic test rects stacked
// vertically. The 1000-twip Y offset of page 1 leaves a 250-twip gap
// below page 0, exercising the parser's tolerance for non-contiguous
// rects. At dpiScale=1.0 each page is 100×50 px (1500/15 × 750/15)
// which the painting tests rely on. Twip dims are deliberately small
// to keep the fake paint buffer cheap; not an A4 representation.
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

// TestRenderPage_MultiPart_RestoresActivePartOnPaintError asserts the
// defer SetPart(active) runs even when paintToNRGBA returns an error
// after the SetPart(page) has already happened. The scenario: zero
// DocumentSize after the SetPart, which triggers paintToNRGBA's
// "page has zero dimensions" branch.
func TestRenderPage_MultiPart_RestoresActivePartOnPaintError(t *testing.T) {
	fb := &fakeBackend{
		tileMode:       1,
		partsCount:     2,
		partActive:     0,
		docWidthTwips:  0, // forces the zero-dims error after SetPart(page)
		docHeightTwips: 0,
	}
	_, doc := loadFakeDoc(t, fb)
	if err := doc.InitializeForRendering(""); err != nil {
		t.Fatal(err)
	}
	if _, err := doc.RenderPage(1, 1.0); err == nil {
		t.Fatal("RenderPage(1) on zero-DocumentSize: expected error, got nil")
	}
	if got, _ := doc.Part(); got != 0 {
		t.Errorf("active part after errored RenderPage = %d, want 0 (defer restore must run on error path)", got)
	}
}

// TestRenderPage_MultiPart_NoRestoreWhenActiveInvalid covers the
// guard that skips restoration when DocumentGetPart returned -1
// (no active view). Restoring to -1 would leave LO in an
// undocumented state.
func TestRenderPage_MultiPart_NoRestoreWhenActiveInvalid(t *testing.T) {
	pxW, pxH := 100, 50
	bgra := make([]byte, 4*pxW*pxH)
	fb := &fakePaintingBackend{
		fakeBackend: fakeBackend{
			tileMode:       1,
			partsCount:     2,
			partActive:     -1, // simulates "no active view"
			docWidthTwips:  1500,
			docHeightTwips: 750,
		},
		paintBytes: bgra,
	}
	_, doc := loadFakeDocWithBackend(t, fb)
	if err := doc.InitializeForRendering(""); err != nil {
		t.Fatal(err)
	}
	if _, err := doc.RenderPage(1, 1.0); err != nil {
		t.Fatalf("RenderPage(1): %v", err)
	}
	// Active part stays at the rendered page (1) — no restore was
	// attempted because active was -1. Restoring to -1 would have
	// failed silently in real LO.
	if got, _ := doc.Part(); got != 1 {
		t.Errorf("active part = %d, want 1 (no restore attempted from -1)", got)
	}
}

// TestRenderPage_MultiPart_PropagatesSetPartUnsupported guards
// against a silent-failure regression: when DocumentSetPart returns
// ErrUnsupported (NULL vtable slot on a stripped LO build), the
// multi-part path must surface it instead of paginating against the
// wrong (still-active) part. Without the propagation a stripped LO
// would return the active part's pixels under whatever index the
// caller asked for, which is exactly the silent-failure pattern PR
// #38 set out to fix.
func TestRenderPage_MultiPart_PropagatesSetPartUnsupported(t *testing.T) {
	fb := &fakeBackend{
		tileMode:       1,
		partsCount:     2,
		partActive:     0,
		docWidthTwips:  1500,
		docHeightTwips: 750,
		setPartErr:     ErrUnsupported,
	}
	_, doc := loadFakeDoc(t, fb)
	if err := doc.InitializeForRendering(""); err != nil {
		t.Fatal(err)
	}
	_, err := doc.RenderPage(1, 1.0)
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("RenderPage with NULL setPart vtable: err=%v, want ErrUnsupported", err)
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
