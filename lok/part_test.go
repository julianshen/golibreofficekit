//go:build linux || darwin

package lok

import (
	"errors"
	"slices"
	"testing"
)

func TestParts_ReturnsBackendCount(t *testing.T) {
	fb := &fakeBackend{partsCount: 3}
	_, doc := loadFakeDoc(t, fb)
	n, err := doc.Parts()
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("Parts=%d, want 3", n)
	}
}

func TestParts_BackendFailureErrors(t *testing.T) {
	fb := &fakeBackend{partsCount: -1}
	_, doc := loadFakeDoc(t, fb)
	_, err := doc.Parts()
	var lokErr *LOKError
	if !errors.As(err, &lokErr) {
		t.Errorf("want *LOKError, got %T %v", err, err)
	}
}

func TestPart_ReadsActive(t *testing.T) {
	_, doc := loadFakeDoc(t, &fakeBackend{partsCount: 4, partActive: 2})
	got, err := doc.Part()
	if err != nil {
		t.Fatal(err)
	}
	if got != 2 {
		t.Errorf("Part=%d, want 2", got)
	}
}

func TestPart_BackendFailureErrors(t *testing.T) {
	_, doc := loadFakeDoc(t, &fakeBackend{partActive: -1})
	_, err := doc.Part()
	var lokErr *LOKError
	if !errors.As(err, &lokErr) {
		t.Errorf("want *LOKError, got %T %v", err, err)
	}
}

func TestSetPart_UpdatesActive(t *testing.T) {
	fb := &fakeBackend{partsCount: 4}
	_, doc := loadFakeDoc(t, fb)
	if err := doc.SetPart(2); err != nil {
		t.Fatal(err)
	}
	if fb.partActive != 2 {
		t.Errorf("partActive=%d, want 2", fb.partActive)
	}
}

func TestSetPartMode_Records(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	if err := doc.SetPartMode(42); err != nil {
		t.Fatal(err)
	}
	if fb.lastPartMode != 42 {
		t.Errorf("lastPartMode=%d, want 42", fb.lastPartMode)
	}
}

func TestPartName_ReadsMap(t *testing.T) {
	fb := &fakeBackend{partNames: map[int]string{1: "Sheet2"}}
	_, doc := loadFakeDoc(t, fb)
	name, err := doc.PartName(1)
	if err != nil {
		t.Fatal(err)
	}
	if name != "Sheet2" {
		t.Errorf("PartName(1)=%q, want Sheet2", name)
	}
}

func TestPartHash_ReadsMap(t *testing.T) {
	fb := &fakeBackend{partHashes: map[int]string{0: "abc123"}}
	_, doc := loadFakeDoc(t, fb)
	hash, err := doc.PartHash(0)
	if err != nil {
		t.Fatal(err)
	}
	if hash != "abc123" {
		t.Errorf("PartHash(0)=%q", hash)
	}
}

func TestPartInfo_UnwrapsJSON(t *testing.T) {
	fb := &fakeBackend{partInfos: map[int]string{0: `{"visible":true}`}}
	_, doc := loadFakeDoc(t, fb)
	raw, err := doc.PartInfo(0)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"visible":true}` {
		t.Errorf("PartInfo=%q", string(raw))
	}
}

func TestPartInfo_EmptyIsNil(t *testing.T) {
	_, doc := loadFakeDoc(t, &fakeBackend{})
	raw, err := doc.PartInfo(0)
	if err != nil {
		t.Fatalf("empty PartInfo: err=%v, want nil", err)
	}
	if raw != nil {
		t.Errorf("empty PartInfo: raw=%q, want nil", string(raw))
	}
}

func TestDocumentSize_Reads(t *testing.T) {
	fb := &fakeBackend{docWidthTwips: 12240, docHeightTwips: 15840}
	_, doc := loadFakeDoc(t, fb)
	w, h, err := doc.DocumentSize()
	if err != nil {
		t.Fatal(err)
	}
	if w != 12240 || h != 15840 {
		t.Errorf("DocumentSize=(%d, %d), want (12240, 15840)", w, h)
	}
}

func TestPartPageRectangles_Parses(t *testing.T) {
	fb := &fakeBackend{
		partRects: "0, 0, 12240, 15840; 0, 15840, 12240, 15840",
	}
	_, doc := loadFakeDoc(t, fb)
	rects, err := doc.PartPageRectangles()
	if err != nil {
		t.Fatal(err)
	}
	want := []TwipRect{
		{X: 0, Y: 0, W: 12240, H: 15840},
		{X: 0, Y: 15840, W: 12240, H: 15840},
	}
	if len(rects) != len(want) {
		t.Fatalf("got %d rects, want %d", len(rects), len(want))
	}
	for i := range want {
		if rects[i] != want[i] {
			t.Errorf("rect %d: got %+v, want %+v", i, rects[i], want[i])
		}
	}
}

func TestPartPageRectangles_EmptyString(t *testing.T) {
	_, doc := loadFakeDoc(t, &fakeBackend{partRects: ""})
	rects, err := doc.PartPageRectangles()
	if err != nil {
		t.Fatal(err)
	}
	if rects != nil {
		t.Errorf("PartPageRectangles on empty: got %v, want nil", rects)
	}
}

func TestPartPageRectangles_MalformedErrors(t *testing.T) {
	_, doc := loadFakeDoc(t, &fakeBackend{partRects: "abc, def, ghi, jkl"})
	_, err := doc.PartPageRectangles()
	var lokErr *LOKError
	if !errors.As(err, &lokErr) {
		t.Errorf("want *LOKError, got %T %v", err, err)
	}
}

func TestParsePartPageRectangles_Direct(t *testing.T) {
	cases := []struct {
		in   string
		want []TwipRect
		err  bool
	}{
		{"", nil, false},
		{";", nil, false},
		{"; ;", nil, false},
		{"0, 0, 100, 200", []TwipRect{{0, 0, 100, 200}}, false},
		{"0,0,100,200", []TwipRect{{0, 0, 100, 200}}, false},
		{"0, 0, 100, 200; 100, 0, 50, 200", []TwipRect{{0, 0, 100, 200}, {100, 0, 50, 200}}, false},
		{"0, 0, 100, 200;", []TwipRect{{0, 0, 100, 200}}, false},
		{"-10, -20, 100, 200", []TwipRect{{-10, -20, 100, 200}}, false},
		{"garbage", nil, true},
		{"1, 2, 3", nil, true},
	}
	for _, tc := range cases {
		got, err := parsePartPageRectangles(tc.in)
		if (err != nil) != tc.err {
			t.Errorf("input %q: err=%v, wantErr=%v", tc.in, err, tc.err)
			continue
		}
		if tc.err {
			continue
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("input %q: got %+v, want %+v", tc.in, got, tc.want)
		}
	}
}

func TestSetOutlineState_PassesParams(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	if err := doc.SetOutlineState(true, 2, 5, true); err != nil {
		t.Fatal(err)
	}
	if !fb.lastOutlineCol || fb.lastOutlineLevel != 2 || fb.lastOutlineIndex != 5 || !fb.lastOutlineHidden {
		t.Errorf("outline state: got (col=%v, lvl=%d, idx=%d, hidden=%v), want (true, 2, 5, true)",
			fb.lastOutlineCol, fb.lastOutlineLevel, fb.lastOutlineIndex, fb.lastOutlineHidden)
	}
}

func TestPartMethods_AfterCloseErrors(t *testing.T) {
	cases := []struct {
		name string
		call func(*Document) error
	}{
		{"Parts", func(d *Document) error { _, err := d.Parts(); return err }},
		{"Part", func(d *Document) error { _, err := d.Part(); return err }},
		{"SetPart", func(d *Document) error { return d.SetPart(0) }},
		{"SetPartMode", func(d *Document) error { return d.SetPartMode(0) }},
		{"PartName", func(d *Document) error { _, err := d.PartName(0); return err }},
		{"PartHash", func(d *Document) error { _, err := d.PartHash(0); return err }},
		{"PartInfo", func(d *Document) error { _, err := d.PartInfo(0); return err }},
		{"DocumentSize", func(d *Document) error { _, _, err := d.DocumentSize(); return err }},
		{"PartPageRectangles", func(d *Document) error { _, err := d.PartPageRectangles(); return err }},
		{"SetOutlineState", func(d *Document) error { return d.SetOutlineState(false, 0, 0, false) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, doc := loadFakeDoc(t, &fakeBackend{})
			doc.Close()
			if err := tc.call(doc); !errors.Is(err, ErrClosed) {
				t.Errorf("want ErrClosed, got %v", err)
			}
		})
	}
}
