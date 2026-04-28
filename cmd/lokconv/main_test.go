package main

import (
	"strings"
	"testing"
)

func TestOutputFormatFromPath(t *testing.T) {
	cases := map[string]outFormat{
		"out.pdf":       fmtPDF,
		"OUT.PDF":       fmtPDF,
		"a/b/c.pdf":     fmtPDF,
		"page.png":      fmtPNG,
		"page.PNG":      fmtPNG,
		"path.with.png": fmtPNG,
		"x.jpg":         fmtUnknown,
		"noext":         fmtUnknown,
		"":              fmtUnknown,
	}
	for path, want := range cases {
		if got := outputFormatFromPath(path); got != want {
			t.Errorf("outputFormatFromPath(%q) = %v, want %v", path, got, want)
		}
	}
}

// convert's unsupported-extension branch is reachable without a real
// LO install — the format check happens before lok.New. Use a
// deliberately-bogus loPath to make sure we hit the early-return.
func TestConvert_UnsupportedExtension(t *testing.T) {
	err := convert("/no/such/lo", "in.odt", "out.bmp", 0, 1.0)
	if err == nil {
		t.Fatal("expected error for unsupported extension, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported output extension") {
		t.Errorf("error %q does not mention unsupported extension", err)
	}
}
