package main

import (
	"os"
	"path/filepath"
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

func TestResolveLOPath_PrefersExplicit(t *testing.T) {
	dir := t.TempDir()
	got, err := resolveLOPath(dir, []string{"/should/not/be/checked"})
	if err != nil {
		t.Fatalf("explicit path: %v", err)
	}
	if got != dir {
		t.Errorf("got %q, want %q", got, dir)
	}
}

func TestResolveLOPath_RejectsExplicitNonDir(t *testing.T) {
	f := filepath.Join(t.TempDir(), "notadir")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveLOPath(f, nil); err == nil {
		t.Errorf("expected error for non-directory explicit path")
	}
}

// A typo'd -lo-path is the realistic failure mode; assert the missing
// path is reported (don't bother matching the wrap text — different
// platforms phrase ENOENT differently).
func TestResolveLOPath_RejectsExplicitMissing(t *testing.T) {
	ghost := filepath.Join(t.TempDir(), "no", "such", "dir")
	if _, err := resolveLOPath(ghost, nil); err == nil {
		t.Errorf("expected error for missing explicit path")
	}
}

func TestResolveLOPath_AutoDetect(t *testing.T) {
	a := t.TempDir()
	b := t.TempDir()
	got, err := resolveLOPath("", []string{
		filepath.Join(a, "missing"),
		b,
	})
	if err != nil {
		t.Fatalf("auto-detect: %v", err)
	}
	if got != b {
		t.Errorf("got %q, want %q (first existing candidate)", got, b)
	}
}

func TestResolveLOPath_NoneFound(t *testing.T) {
	if _, err := resolveLOPath("", []string{"/does/not/exist", "/nope"}); err == nil {
		t.Errorf("expected error when no candidate exists")
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
