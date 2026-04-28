package main

import "testing"

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
		{fmtMD, fmtDOCX, true},
		{fmtDOCX, fmtMD, true},
		{fmtMD, fmtPPTX, true},
		{fmtPPTX, fmtMD, true},

		{fmtDOCX, fmtPPTX, false},
		{fmtPPTX, fmtDOCX, false},
		{fmtMD, fmtMD, false},
		{fmtDOCX, fmtDOCX, false},
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
