//go:build linux || darwin

package lok

import (
	"bytes"
	"errors"
	"image/png"
	"testing"
)

// twipsPerInch is the conversion factor LOK uses (1 inch = 1440 twips).
// Render* methods convert (twipW * dpiScale * 96) / 1440 to pixels —
// at dpiScale=1.0 that's twipW/15.
const twipsPerInch = 1440

func TestRenderImage_HappyPath(t *testing.T) {
	// 1500 twips wide × 750 twips tall at dpiScale=1.0 → 100×50 px.
	// Provide that exact-size BGRA buffer so the fake paints all pixels.
	pxW, pxH := 100, 50
	bgra := make([]byte, 4*pxW*pxH)
	for i := 0; i < pxW*pxH; i++ {
		// Premultiplied red: B=0, G=0, R=255, A=255 → after unpremul:
		// R=255 unchanged.
		bgra[4*i+0] = 0
		bgra[4*i+1] = 0
		bgra[4*i+2] = 255
		bgra[4*i+3] = 255
	}
	fb := &fakePaintingBackend{
		fakeBackend: fakeBackend{
			tileMode:       1,
			docWidthTwips:  1500,
			docHeightTwips: 750,
		},
		paintBytes: bgra,
	}
	_, doc := loadFakeDocWithBackend(t, fb)
	if err := doc.InitializeForRendering(""); err != nil {
		t.Fatalf("InitializeForRendering: %v", err)
	}

	img, err := doc.RenderImage(1.0)
	if err != nil {
		t.Fatalf("RenderImage: %v", err)
	}
	if got := img.Bounds().Dx(); got != pxW {
		t.Errorf("width=%d, want %d", got, pxW)
	}
	if got := img.Bounds().Dy(); got != pxH {
		t.Errorf("height=%d, want %d", got, pxH)
	}
	// Top-left pixel after unpremultiply should be (R=255, G=0, B=0, A=255).
	r, g, b, a := img.At(0, 0).RGBA()
	if r>>8 != 255 || g>>8 != 0 || b>>8 != 0 || a>>8 != 255 {
		t.Errorf("pixel(0,0) RGBA = (%d,%d,%d,%d), want (255,0,0,255)", r>>8, g>>8, b>>8, a>>8)
	}
}

func TestRenderImage_DpiScaling(t *testing.T) {
	// 1500×750 twips at dpiScale=2.0 → 200×100 px.
	pxW, pxH := 200, 100
	bgra := make([]byte, 4*pxW*pxH)
	fb := &fakePaintingBackend{
		fakeBackend: fakeBackend{
			tileMode:       1,
			docWidthTwips:  1500,
			docHeightTwips: 750,
		},
		paintBytes: bgra,
	}
	_, doc := loadFakeDocWithBackend(t, fb)
	if err := doc.InitializeForRendering(""); err != nil {
		t.Fatal(err)
	}
	img, err := doc.RenderImage(2.0)
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != pxW || img.Bounds().Dy() != pxH {
		t.Errorf("got %dx%d, want %dx%d", img.Bounds().Dx(), img.Bounds().Dy(), pxW, pxH)
	}
}

func TestRenderImage_InvalidDpi(t *testing.T) {
	fb := &fakeBackend{tileMode: 1, docWidthTwips: 1500, docHeightTwips: 750}
	_, doc := loadFakeDoc(t, fb)
	if err := doc.InitializeForRendering(""); err != nil {
		t.Fatal(err)
	}
	for _, dpi := range []float64{0, -1, 0.0} {
		_, err := doc.RenderImage(dpi)
		var lokErr *LOKError
		if !errors.As(err, &lokErr) || lokErr.Op != "RenderImage" {
			t.Errorf("dpi=%v: want *LOKError{Op:RenderImage}, got %v", dpi, err)
		}
	}
}

func TestRenderImage_ZeroDocumentSize(t *testing.T) {
	fb := &fakeBackend{tileMode: 1, docWidthTwips: 0, docHeightTwips: 0}
	_, doc := loadFakeDoc(t, fb)
	if err := doc.InitializeForRendering(""); err != nil {
		t.Fatal(err)
	}
	_, err := doc.RenderImage(1.0)
	var lokErr *LOKError
	if !errors.As(err, &lokErr) || lokErr.Op != "RenderImage" {
		t.Errorf("want *LOKError{Op:RenderImage}, got %v", err)
	}
}

func TestRenderImage_NotInitialized(t *testing.T) {
	fb := &fakeBackend{tileMode: 1, docWidthTwips: 1500, docHeightTwips: 750}
	_, doc := loadFakeDoc(t, fb)
	// Skip InitializeForRendering — PaintTile must reject the call.
	_, err := doc.RenderImage(1.0)
	var lokErr *LOKError
	if !errors.As(err, &lokErr) {
		t.Errorf("want *LOKError, got %v", err)
	}
}

func TestRenderImage_Closed(t *testing.T) {
	fb := &fakeBackend{tileMode: 1, docWidthTwips: 1500, docHeightTwips: 750}
	_, doc := loadFakeDoc(t, fb)
	doc.Close()
	if _, err := doc.RenderImage(1.0); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestRenderPNG_ValidPNG(t *testing.T) {
	pxW, pxH := 100, 50
	bgra := make([]byte, 4*pxW*pxH)
	fb := &fakePaintingBackend{
		fakeBackend: fakeBackend{
			tileMode:       1,
			docWidthTwips:  1500,
			docHeightTwips: 750,
		},
		paintBytes: bgra,
	}
	_, doc := loadFakeDocWithBackend(t, fb)
	if err := doc.InitializeForRendering(""); err != nil {
		t.Fatal(err)
	}
	pngBytes, err := doc.RenderPNG(1.0)
	if err != nil {
		t.Fatalf("RenderPNG: %v", err)
	}
	if len(pngBytes) == 0 {
		t.Fatal("RenderPNG returned empty bytes")
	}
	// Decode and check dimensions match.
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("png.Decode: %v", err)
	}
	if img.Bounds().Dx() != pxW || img.Bounds().Dy() != pxH {
		t.Errorf("decoded PNG: got %dx%d, want %dx%d", img.Bounds().Dx(), img.Bounds().Dy(), pxW, pxH)
	}
}

func TestRenderPNG_Closed(t *testing.T) {
	fb := &fakeBackend{tileMode: 1, docWidthTwips: 1500, docHeightTwips: 750}
	_, doc := loadFakeDoc(t, fb)
	doc.Close()
	if _, err := doc.RenderPNG(1.0); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}
