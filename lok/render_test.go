//go:build linux || darwin

package lok

import (
	"bytes"
	"errors"
	"image"
	"testing"
)

func TestInitializeForRendering_HappyPath(t *testing.T) {
	fb := &fakeBackend{tileMode: 1} // BGRA
	_, doc := loadFakeDoc(t, fb)
	if err := doc.InitializeForRendering(""); err != nil {
		t.Fatalf("InitializeForRendering: %v", err)
	}
	if fb.lastInitArgs != "" {
		t.Errorf("lastInitArgs=%q, want empty", fb.lastInitArgs)
	}
}

func TestInitializeForRendering_ForwardsArgs(t *testing.T) {
	fb := &fakeBackend{tileMode: 1}
	_, doc := loadFakeDoc(t, fb)
	args := `{".uno:HideWhitespace":{"type":"boolean","value":"true"}}`
	if err := doc.InitializeForRendering(args); err != nil {
		t.Fatal(err)
	}
	if fb.lastInitArgs != args {
		t.Errorf("lastInitArgs=%q, want %q", fb.lastInitArgs, args)
	}
}

func TestInitializeForRendering_UnsupportedTileMode(t *testing.T) {
	fb := &fakeBackend{tileMode: 0} // RGBA — unsupported
	_, doc := loadFakeDoc(t, fb)
	err := doc.InitializeForRendering("")
	var lokErr *LOKError
	if !errors.As(err, &lokErr) || lokErr.Op != "InitializeForRendering" {
		t.Errorf("want *LOKError{Op: InitializeForRendering}, got %T %v", err, err)
	}
}

func TestInitializeForRendering_UnexpectedTileMode(t *testing.T) {
	// Any non-1 mode (including future enum values) is treated as an error.
	fb := &fakeBackend{tileMode: 2}
	_, doc := loadFakeDoc(t, fb)
	err := doc.InitializeForRendering("")
	var lokErr *LOKError
	if !errors.As(err, &lokErr) {
		t.Errorf("want *LOKError, got %T %v", err, err)
	}
}

func TestInitializeForRendering_AfterCloseErrors(t *testing.T) {
	_, doc := loadFakeDoc(t, &fakeBackend{tileMode: 1})
	doc.Close()
	if err := doc.InitializeForRendering(""); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestSetClientZoom_Passes(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	if err := doc.SetClientZoom(256, 256, 1440, 1440); err != nil {
		t.Fatal(err)
	}
	if fb.lastZoom != [4]int{256, 256, 1440, 1440} {
		t.Errorf("lastZoom=%v", fb.lastZoom)
	}
}

func TestSetClientZoom_WithoutInitializeOK(t *testing.T) {
	// Zoom is an optional hint; does NOT require InitializeForRendering.
	_, doc := loadFakeDoc(t, &fakeBackend{})
	if err := doc.SetClientZoom(1, 1, 1, 1); err != nil {
		t.Errorf("want no error, got %v", err)
	}
}

func TestSetClientVisibleArea_PassesAsInt(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	if err := doc.SetClientVisibleArea(TwipRect{X: 0, Y: 0, W: 14400, H: 14400}); err != nil {
		t.Fatal(err)
	}
	if fb.lastVisibleArea != [4]int{0, 0, 14400, 14400} {
		t.Errorf("lastVisibleArea=%v", fb.lastVisibleArea)
	}
}

func TestSetClientVisibleArea_RejectsOverflow(t *testing.T) {
	_, doc := loadFakeDoc(t, &fakeBackend{})
	err := doc.SetClientVisibleArea(TwipRect{X: 0, Y: 0, W: 1<<32 + 1, H: 1})
	var lokErr *LOKError
	if !errors.As(err, &lokErr) {
		t.Fatalf("want *LOKError, got %T %v", err, err)
	}
	if lokErr.Op != "SetClientVisibleArea" {
		t.Errorf("Op=%q, want SetClientVisibleArea", lokErr.Op)
	}
}

func TestPaintTileRaw_PassesTileArgs(t *testing.T) {
	fb := &fakeBackend{tileMode: 1}
	_, doc := loadFakeDoc(t, fb)
	if err := doc.InitializeForRendering(""); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4*256*256)
	if err := doc.PaintTileRaw(buf, 256, 256, TwipRect{X: 10, Y: 20, W: 3000, H: 4000}); err != nil {
		t.Fatal(err)
	}
	if len(fb.paintCalls) != 1 {
		t.Fatalf("paintCalls: %d, want 1", len(fb.paintCalls))
	}
	got := fb.paintCalls[0]
	want := fakePaint{pxW: 256, pxH: 256, x: 10, y: 20, w: 3000, h: 4000, bufLen: 4 * 256 * 256}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestPaintTileRaw_WrongBufferSizeErrors(t *testing.T) {
	fb := &fakeBackend{tileMode: 1}
	_, doc := loadFakeDoc(t, fb)
	if err := doc.InitializeForRendering(""); err != nil {
		t.Fatal(err)
	}
	// Too small.
	err := doc.PaintTileRaw(make([]byte, 10), 256, 256, TwipRect{})
	var lokErr *LOKError
	if !errors.As(err, &lokErr) {
		t.Errorf("too-small: want *LOKError, got %T %v", err, err)
	}
	// Too large — exact match required.
	err = doc.PaintTileRaw(make([]byte, 4*256*256+1), 256, 256, TwipRect{})
	if !errors.As(err, &lokErr) {
		t.Errorf("too-large: want *LOKError, got %T %v", err, err)
	}
	if len(fb.paintCalls) != 0 {
		t.Errorf("paintCalls should be empty on size-mismatch; got %d", len(fb.paintCalls))
	}
}

func TestPaintTileRaw_WithoutInitializeErrors(t *testing.T) {
	fb := &fakeBackend{tileMode: 1}
	_, doc := loadFakeDoc(t, fb)
	err := doc.PaintTileRaw(make([]byte, 4*256*256), 256, 256, TwipRect{})
	var lokErr *LOKError
	if !errors.As(err, &lokErr) || lokErr.Op != "PaintTile" {
		t.Errorf("want *LOKError{Op: PaintTile}, got %T %v", err, err)
	}
}

func TestPaintTileRaw_RangeCheck(t *testing.T) {
	fb := &fakeBackend{tileMode: 1}
	_, doc := loadFakeDoc(t, fb)
	if err := doc.InitializeForRendering(""); err != nil {
		t.Fatal(err)
	}
	err := doc.PaintTileRaw(make([]byte, 4), 1, 1, TwipRect{W: 1<<32 + 1})
	var lokErr *LOKError
	if !errors.As(err, &lokErr) {
		t.Errorf("want *LOKError, got %T %v", err, err)
	}
	if len(fb.paintCalls) != 0 {
		t.Error("paintCalls not empty after range error")
	}
}

func TestPaintTile_AllocatesAndUnpremultiplies(t *testing.T) {
	// Fake backend writes known premul BGRA into the caller's buffer;
	// PaintTile should return NRGBA with the unpremultiplied values.
	fb := &fakePaintingBackend{
		fakeBackend: fakeBackend{tileMode: 1},
		// Two pixels: opaque red, 50% red.
		paintBytes: []byte{0, 0, 255, 255, 0, 0, 128, 128},
	}
	_, doc := loadFakeDocWithBackend(t, fb)
	if err := doc.InitializeForRendering(""); err != nil {
		t.Fatal(err)
	}
	img, err := doc.PaintTile(2, 1, TwipRect{W: 100, H: 50})
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{255, 0, 0, 255, 255, 0, 0, 128}
	if !bytes.Equal(img.Pix, want) {
		t.Errorf("img.Pix=%v, want %v", img.Pix, want)
	}
	if img.Rect != image.Rect(0, 0, 2, 1) {
		t.Errorf("img.Rect=%v, want (0,0)-(2,1)", img.Rect)
	}
}

func TestPaintPartTileRaw_PassesPart(t *testing.T) {
	fb := &fakeBackend{tileMode: 1}
	_, doc := loadFakeDoc(t, fb)
	if err := doc.InitializeForRendering(""); err != nil {
		t.Fatal(err)
	}
	if err := doc.PaintPartTileRaw(make([]byte, 4*2*2), 3, 2, 2, TwipRect{W: 100, H: 100}); err != nil {
		t.Fatal(err)
	}
	if len(fb.partPaintCalls) != 1 || fb.partPaintCalls[0].part != 3 {
		t.Errorf("partPaintCalls=%+v", fb.partPaintCalls)
	}
}

func TestRenderSearchResultRaw_NoMatch(t *testing.T) {
	fb := &fakeBackend{tileMode: 1} // searchResultOK defaults to false
	_, doc := loadFakeDoc(t, fb)
	if err := doc.InitializeForRendering(""); err != nil {
		t.Fatal(err)
	}
	buf, w, h, err := doc.RenderSearchResultRaw("nope")
	if err != nil {
		t.Fatal(err)
	}
	if buf != nil || w != 0 || h != 0 {
		t.Errorf("no-match: got (%v, %d, %d), want (nil, 0, 0)", buf, w, h)
	}
	if fb.lastSearchQuery != "nope" {
		t.Errorf("query not forwarded: %q", fb.lastSearchQuery)
	}
}

func TestRenderSearchResultRaw_Match(t *testing.T) {
	bgra := []byte{0, 0, 255, 255} // opaque red pixel
	fb := &fakeBackend{
		tileMode:        1,
		searchResultBuf: bgra,
		searchResultPxW: 1,
		searchResultPxH: 1,
		searchResultOK:  true,
	}
	_, doc := loadFakeDoc(t, fb)
	if err := doc.InitializeForRendering(""); err != nil {
		t.Fatal(err)
	}
	buf, w, h, err := doc.RenderSearchResultRaw("q")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf, bgra) || w != 1 || h != 1 {
		t.Errorf("got (%v, %d, %d)", buf, w, h)
	}
}

func TestRenderSearchResult_UnpremultipliesToNRGBA(t *testing.T) {
	fb := &fakeBackend{
		tileMode:        1,
		searchResultBuf: []byte{0, 0, 255, 255}, // red
		searchResultPxW: 1,
		searchResultPxH: 1,
		searchResultOK:  true,
	}
	_, doc := loadFakeDoc(t, fb)
	if err := doc.InitializeForRendering(""); err != nil {
		t.Fatal(err)
	}
	img, err := doc.RenderSearchResult("q")
	if err != nil {
		t.Fatal(err)
	}
	if img == nil {
		t.Fatal("img is nil on match")
	}
	want := []byte{255, 0, 0, 255}
	if !bytes.Equal(img.Pix, want) {
		t.Errorf("got %v, want %v", img.Pix, want)
	}
}

func TestRenderSearchResult_NoMatch(t *testing.T) {
	_, doc := loadFakeDoc(t, &fakeBackend{tileMode: 1})
	if err := doc.InitializeForRendering(""); err != nil {
		t.Fatal(err)
	}
	img, err := doc.RenderSearchResult("nope")
	if err != nil {
		t.Fatal(err)
	}
	if img != nil {
		t.Errorf("no-match: img=%v, want nil", img)
	}
}

func TestRenderSearchResult_RequiresInitialize(t *testing.T) {
	_, doc := loadFakeDoc(t, &fakeBackend{tileMode: 1, searchResultOK: true})
	_, err := doc.RenderSearchResult("q")
	var lokErr *LOKError
	if !errors.As(err, &lokErr) {
		t.Errorf("want *LOKError, got %T %v", err, err)
	}
}

func TestRenderShapeSelection_Empty(t *testing.T) {
	_, doc := loadFakeDoc(t, &fakeBackend{tileMode: 1})
	if err := doc.InitializeForRendering(""); err != nil {
		t.Fatal(err)
	}
	got, err := doc.RenderShapeSelection()
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("empty: got %v, want nil", got)
	}
}

func TestRenderShapeSelection_ReturnsBytes(t *testing.T) {
	payload := []byte("<svg/>")
	fb := &fakeBackend{tileMode: 1, shapeSelection: payload}
	_, doc := loadFakeDoc(t, fb)
	if err := doc.InitializeForRendering(""); err != nil {
		t.Fatal(err)
	}
	got, err := doc.RenderShapeSelection()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("got %q, want %q", got, payload)
	}
}

func TestRenderMethods_AfterCloseErrors(t *testing.T) {
	cases := []struct {
		name string
		call func(*Document) error
	}{
		{"InitializeForRendering", func(d *Document) error { return d.InitializeForRendering("") }},
		{"SetClientZoom", func(d *Document) error { return d.SetClientZoom(1, 1, 1, 1) }},
		{"SetClientVisibleArea", func(d *Document) error { return d.SetClientVisibleArea(TwipRect{}) }},
		{"PaintTileRaw", func(d *Document) error { return d.PaintTileRaw(make([]byte, 4), 1, 1, TwipRect{}) }},
		{"PaintTile", func(d *Document) error { _, err := d.PaintTile(1, 1, TwipRect{}); return err }},
		{"PaintPartTileRaw", func(d *Document) error { return d.PaintPartTileRaw(make([]byte, 4), 0, 1, 1, TwipRect{}) }},
		{"PaintPartTile", func(d *Document) error { _, err := d.PaintPartTile(0, 1, 1, TwipRect{}); return err }},
		{"RenderSearchResultRaw", func(d *Document) error { _, _, _, err := d.RenderSearchResultRaw("q"); return err }},
		{"RenderSearchResult", func(d *Document) error { _, err := d.RenderSearchResult("q"); return err }},
		{"RenderShapeSelection", func(d *Document) error { _, err := d.RenderShapeSelection(); return err }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, doc := loadFakeDoc(t, &fakeBackend{tileMode: 1})
			doc.Close()
			if err := tc.call(doc); !errors.Is(err, ErrClosed) {
				t.Errorf("want ErrClosed, got %v", err)
			}
		})
	}
}

// fakePaintingBackend extends fakeBackend with a programmable tile
// payload that PaintTile writes into the caller's buffer, so the
// unpremultiply path has something deterministic to decode.
type fakePaintingBackend struct {
	fakeBackend
	paintBytes []byte
}

func (f *fakePaintingBackend) DocumentPaintTile(_ documentHandle, buf []byte, pxW, pxH, x, y, w, h int) {
	copy(buf, f.paintBytes)
	f.paintCalls = append(f.paintCalls, fakePaint{pxW: pxW, pxH: pxH, x: x, y: y, w: w, h: h, bufLen: len(buf)})
}

func (f *fakePaintingBackend) DocumentPaintPartTile(_ documentHandle, buf []byte, part, mode, pxW, pxH, x, y, w, h int) {
	copy(buf, f.paintBytes)
	f.partPaintCalls = append(f.partPaintCalls, fakePartPaint{part: part, mode: mode, pxW: pxW, pxH: pxH, x: x, y: y, w: w, h: h, bufLen: len(buf)})
}

// loadFakeDocWithBackend is loadFakeDoc for callers that need to
// install a backend other than *fakeBackend (e.g. *fakePaintingBackend
// which embeds it).
func loadFakeDocWithBackend(t *testing.T, be backend) (*Office, *Document) {
	t.Helper()
	orig := currentBackend
	t.Cleanup(func() { setBackend(orig); resetSingleton() })
	setBackend(be)
	resetSingleton()
	o, err := New("/install")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := o.Load("/tmp/x.odt")
	if err != nil {
		o.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { doc.Close(); o.Close() })
	return o, doc
}
