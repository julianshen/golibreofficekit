//go:build linux || darwin

package lok

import (
	"errors"
	"math"
	"testing"
)

func TestSetTextSelectionType_String(t *testing.T) {
	cases := []struct {
		typ  SetTextSelectionType
		want string
	}{
		{SetTextSelectionStart, "SetTextSelectionStart"},
		{SetTextSelectionEnd, "SetTextSelectionEnd"},
		{SetTextSelectionReset, "SetTextSelectionReset"},
		{SetTextSelectionType(99), "SetTextSelectionType(99)"},
	}
	for _, tc := range cases {
		if got := tc.typ.String(); got != tc.want {
			t.Errorf("%d: got %q, want %q", tc.typ, got, tc.want)
		}
	}
}

func TestSetGraphicSelectionType_String(t *testing.T) {
	cases := []struct {
		typ  SetGraphicSelectionType
		want string
	}{
		{SetGraphicSelectionStart, "SetGraphicSelectionStart"},
		{SetGraphicSelectionEnd, "SetGraphicSelectionEnd"},
		{SetGraphicSelectionType(99), "SetGraphicSelectionType(99)"},
	}
	for _, tc := range cases {
		if got := tc.typ.String(); got != tc.want {
			t.Errorf("%d: got %q, want %q", tc.typ, got, tc.want)
		}
	}
}

func TestSelectionKind_String(t *testing.T) {
	cases := []struct {
		k    SelectionKind
		want string
	}{
		{SelectionKindNone, "SelectionKindNone"},
		{SelectionKindText, "SelectionKindText"},
		{SelectionKindComplex, "SelectionKindComplex"},
		{SelectionKind(99), "SelectionKind(99)"},
	}
	for _, tc := range cases {
		if got := tc.k.String(); got != tc.want {
			t.Errorf("%d: got %q, want %q", tc.k, got, tc.want)
		}
	}
}

func TestGetTextSelection_ForwardsArgsAndStrings(t *testing.T) {
	fb := &fakeBackend{selectionText: "hello", selectionUsedMime: "text/plain;charset=utf-8"}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()

	text, usedMime, err := doc.GetTextSelection("text/plain")
	if err != nil {
		t.Fatalf("GetTextSelection: %v", err)
	}
	if text != "hello" || usedMime != "text/plain;charset=utf-8" {
		t.Errorf("got (%q, %q), want (hello, text/plain;charset=utf-8)", text, usedMime)
	}
	if fb.lastGetTextSelectionMime != "text/plain" {
		t.Errorf("mime forwarded = %q, want text/plain", fb.lastGetTextSelectionMime)
	}
}

func TestGetTextSelection_ClosedDoc(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	doc.Close()

	if _, _, err := doc.GetTextSelection("text/plain"); !errors.Is(err, ErrClosed) {
		t.Errorf("closed: want ErrClosed, got %v", err)
	}
}

func TestGetTextSelection_InvalidMime(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()

	cases := []struct {
		name string
		mime string
	}{
		{"empty", ""},
		{"nul", "text/plain\x00"},
		{"too-long", string(make([]byte, 257))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := doc.GetTextSelection(tc.mime); !errors.Is(err, ErrInvalidOption) {
				t.Errorf("mime=%q: want ErrInvalidOption, got %v", tc.mime, err)
			}
		})
	}
}

func TestGetSelectionKind_ReturnsKinds(t *testing.T) {
	cases := []struct {
		raw  int
		want SelectionKind
	}{
		{0, SelectionKindNone},
		{1, SelectionKindText},
		{2, SelectionKindComplex}, // LARGE_TEXT folds to Complex.
		{3, SelectionKindComplex},
	}
	for _, tc := range cases {
		fb := &fakeBackend{selectionKind: tc.raw}
		withFakeBackend(t, fb)
		o, _ := New("/install")
		doc, _ := o.Load("/tmp/x.odt")
		got, err := doc.GetSelectionKind()
		if err != nil {
			t.Fatalf("raw=%d: %v", tc.raw, err)
		}
		if got != tc.want {
			t.Errorf("raw=%d: got %v, want %v", tc.raw, got, tc.want)
		}
		doc.Close()
		o.Close()
	}
}

func TestGetSelectionKind_ClosedDoc(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	doc.Close()

	if _, err := doc.GetSelectionKind(); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestGetSelectionTypeAndText_HappyPath(t *testing.T) {
	fb := &fakeBackend{
		selectionKind:     1,
		selectionText:     "hi",
		selectionUsedMime: "text/plain;charset=utf-8",
	}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()

	kind, text, mime, err := doc.GetSelectionTypeAndText("text/plain")
	if err != nil {
		t.Fatalf("GetSelectionTypeAndText: %v", err)
	}
	if kind != SelectionKindText || text != "hi" || mime != "text/plain;charset=utf-8" {
		t.Errorf("got (%v, %q, %q)", kind, text, mime)
	}
}

func TestGetSelectionTypeAndText_UnsupportedBubbles(t *testing.T) {
	fb := &fakeBackend{selectionTypeTextErr: ErrUnsupported}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()

	_, _, _, err := doc.GetSelectionTypeAndText("text/plain")
	if !errors.Is(err, ErrUnsupported) {
		t.Errorf("want ErrUnsupported, got %v", err)
	}
}

func TestSetTextSelection_ForwardsArgs(t *testing.T) {
	fb := &fakeBackend{}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()

	if err := doc.SetTextSelection(SetTextSelectionEnd, 1000, 2000); err != nil {
		t.Fatal(err)
	}
	if fb.lastSetTextSelectionTyp != 1 || fb.lastSetTextSelectionX != 1000 || fb.lastSetTextSelectionY != 2000 {
		t.Errorf("recorded (%d, %d, %d)", fb.lastSetTextSelectionTyp, fb.lastSetTextSelectionX, fb.lastSetTextSelectionY)
	}
}

func TestSetTextSelection_InvalidType(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()

	if err := doc.SetTextSelection(SetTextSelectionType(99), 0, 0); !errors.Is(err, ErrInvalidOption) {
		t.Errorf("want ErrInvalidOption, got %v", err)
	}
}

func TestSetTextSelection_Closed(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	doc.Close()

	if err := doc.SetTextSelection(SetTextSelectionStart, 0, 0); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestResetSelection_Forwards(t *testing.T) {
	fb := &fakeBackend{}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()

	for range 3 {
		if err := doc.ResetSelection(); err != nil {
			t.Fatal(err)
		}
	}
	if fb.resetSelectionCalls != 3 {
		t.Errorf("resetSelectionCalls=%d, want 3", fb.resetSelectionCalls)
	}
}

func TestResetSelection_Closed(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	doc.Close()
	if err := doc.ResetSelection(); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestSetGraphicSelection_ForwardsArgs(t *testing.T) {
	fb := &fakeBackend{}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()

	if err := doc.SetGraphicSelection(SetGraphicSelectionEnd, 10, 20); err != nil {
		t.Fatal(err)
	}
	if fb.lastSetGraphicTyp != 1 || fb.lastSetGraphicX != 10 || fb.lastSetGraphicY != 20 {
		t.Errorf("recorded (%d, %d, %d)", fb.lastSetGraphicTyp, fb.lastSetGraphicX, fb.lastSetGraphicY)
	}
}

func TestSetGraphicSelection_InvalidType(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()

	if err := doc.SetGraphicSelection(SetGraphicSelectionType(99), 0, 0); !errors.Is(err, ErrInvalidOption) {
		t.Errorf("want ErrInvalidOption, got %v", err)
	}
}

func TestSetGraphicSelection_Closed(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	doc.Close()
	if err := doc.SetGraphicSelection(SetGraphicSelectionStart, 0, 0); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestSetTextSelection_XYRangeRejection(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()

	cases := []struct {
		name string
		x, y int64
	}{
		{"x-over", math.MaxInt32 + 1, 0},
		{"x-under", math.MinInt32 - 1, 0},
		{"y-over", 0, math.MaxInt32 + 1},
		{"y-under", 0, math.MinInt32 - 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := doc.SetTextSelection(SetTextSelectionStart, tc.x, tc.y)
			var lokErr *LOKError
			if !errors.As(err, &lokErr) || lokErr.Op != "SetTextSelection" {
				t.Errorf("want *LOKError Op=SetTextSelection, got %T %v", err, err)
			}
		})
	}
}

func TestSetGraphicSelection_XYRangeRejection(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()

	cases := []struct {
		name string
		x, y int64
	}{
		{"x-over", math.MaxInt32 + 1, 0},
		{"x-under", math.MinInt32 - 1, 0},
		{"y-over", 0, math.MaxInt32 + 1},
		{"y-under", 0, math.MinInt32 - 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := doc.SetGraphicSelection(SetGraphicSelectionStart, tc.x, tc.y)
			var lokErr *LOKError
			if !errors.As(err, &lokErr) || lokErr.Op != "SetGraphicSelection" {
				t.Errorf("want *LOKError Op=SetGraphicSelection, got %T %v", err, err)
			}
		})
	}
}

func TestSetBlockedCommandList_ForwardsArgs(t *testing.T) {
	fb := &fakeBackend{}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()

	if err := doc.SetBlockedCommandList(2, ".uno:Save,.uno:SaveAs"); err != nil {
		t.Fatal(err)
	}
	if fb.lastBlockedViewID != 2 || fb.lastBlockedCSV != ".uno:Save,.uno:SaveAs" {
		t.Errorf("recorded (%d, %q)", fb.lastBlockedViewID, fb.lastBlockedCSV)
	}
}

func TestSetBlockedCommandList_Closed(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	doc.Close()
	if err := doc.SetBlockedCommandList(0, ""); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}
