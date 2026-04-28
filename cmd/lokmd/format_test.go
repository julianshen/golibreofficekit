package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFormatFromExt(t *testing.T) {
	cases := map[string]docFormat{
		"foo.md":         fmtMD,
		"foo.MD":         fmtMD,
		"foo.markdown":   fmtMD,
		"foo.Markdown":   fmtMD,
		"foo.docx":       fmtDOCX,
		"foo.DOCX":       fmtDOCX,
		"foo.pptx":       fmtPPTX,
		"foo.PPTX":       fmtPPTX,
		"foo.txt":        fmtUnknown,
		"":               fmtUnknown,
		"no-ext":         fmtUnknown,
		"a.b.c.markdown": fmtMD,
	}
	for path, want := range cases {
		if got := formatFromExt(path); got != want {
			t.Errorf("formatFromExt(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestSupportedConversion(t *testing.T) {
	cases := []struct {
		in, out docFormat
		want    bool
	}{
		// All four user-asked directions:
		{fmtMD, fmtDOCX, true},
		{fmtDOCX, fmtMD, true},
		{fmtMD, fmtPPTX, true},
		{fmtPPTX, fmtMD, true},

		// Unsupported pairs we expect to reject:
		{fmtDOCX, fmtPPTX, false}, // would silently lose pages or formatting
		{fmtPPTX, fmtDOCX, false}, // same
		{fmtMD, fmtMD, false},     // identity is a no-op, refuse
		{fmtDOCX, fmtDOCX, false}, // same-format copy is a no-op
		{fmtPPTX, fmtPPTX, false},
		{fmtUnknown, fmtMD, false},
		{fmtMD, fmtUnknown, false},
	}
	for _, c := range cases {
		if got := supportedConversion(c.in, c.out); got != c.want {
			t.Errorf("supportedConversion(%v, %v) = %v, want %v", c.in, c.out, got, c.want)
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

func TestResolveLOPath_RejectsExplicitMissing(t *testing.T) {
	ghost := filepath.Join(t.TempDir(), "no", "such", "dir")
	if _, err := resolveLOPath(ghost, nil); err == nil {
		t.Errorf("expected error for missing explicit path")
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

func TestResolveLOPath_AutoDetect(t *testing.T) {
	a := t.TempDir()
	b := t.TempDir()
	got, err := resolveLOPath("", []string{filepath.Join(a, "missing"), b})
	if err != nil {
		t.Fatalf("auto-detect: %v", err)
	}
	if got != b {
		t.Errorf("got %q, want %q", got, b)
	}
}

func TestResolveLOPath_NoneFound(t *testing.T) {
	if _, err := resolveLOPath("", []string{"/does/not/exist", "/nope"}); err == nil {
		t.Errorf("expected error when no candidate exists")
	}
}
