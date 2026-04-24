//go:build linux || darwin

package lok

import (
	"bytes"
	"errors"
	"testing"
)

func TestClipboardItem_ShapeCompiles(t *testing.T) {
	// Compile-time assertion: ClipboardItem has MimeType and Data
	// fields with the documented types.
	it := ClipboardItem{MimeType: "text/plain", Data: []byte("hi")}
	if it.MimeType != "text/plain" || !bytes.Equal(it.Data, []byte("hi")) {
		t.Errorf("ClipboardItem round-trip failed: %+v", it)
	}
}

func TestGetClipboard_NilMimesForwardedAsNil(t *testing.T) {
	fb := &fakeBackend{getClipboardResult: []clipboardItemInternal{
		{MimeType: "text/plain;charset=utf-8", Data: []byte("hi")},
	}}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()

	items, err := doc.GetClipboard(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].MimeType != "text/plain;charset=utf-8" || string(items[0].Data) != "hi" {
		t.Errorf("items=%+v", items)
	}
	if fb.lastGetClipboardMimes != nil {
		t.Errorf("nil mimes forwarded as %v, want nil", fb.lastGetClipboardMimes)
	}
}

func TestGetClipboard_EmptyMimesForwardedAsNil(t *testing.T) {
	fb := &fakeBackend{}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()

	if _, err := doc.GetClipboard([]string{}); err != nil {
		t.Fatal(err)
	}
	if fb.lastGetClipboardMimes != nil {
		t.Errorf("empty mimes forwarded as %v, want nil", fb.lastGetClipboardMimes)
	}
}

func TestGetClipboard_PreservesRequestOrderWithNilData(t *testing.T) {
	fb := &fakeBackend{
		getClipboardResult: []clipboardItemInternal{
			{MimeType: "text/plain", Data: []byte("hi")},
			{MimeType: "application/x-nothing", Data: nil},
		},
	}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()

	items, err := doc.GetClipboard([]string{"text/plain", "application/x-nothing"})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("len=%d, want 2", len(items))
	}
	if items[1].Data != nil {
		t.Errorf("items[1].Data=%v, want nil", items[1].Data)
	}
	if fb.lastGetClipboardMimes[1] != "application/x-nothing" {
		t.Errorf("mime[1] forwarded=%q", fb.lastGetClipboardMimes[1])
	}
}

func TestGetClipboard_InvalidMime(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()

	if _, err := doc.GetClipboard([]string{""}); !errors.Is(err, ErrInvalidOption) {
		t.Errorf("want ErrInvalidOption, got %v", err)
	}
}

func TestGetClipboard_Closed(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	doc.Close()
	if _, err := doc.GetClipboard(nil); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestGetClipboard_BackendErrorSurfaces(t *testing.T) {
	fb := &fakeBackend{getClipboardErr: ErrUnsupported}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()
	if _, err := doc.GetClipboard(nil); !errors.Is(err, ErrUnsupported) {
		t.Errorf("want ErrUnsupported, got %v", err)
	}
}
