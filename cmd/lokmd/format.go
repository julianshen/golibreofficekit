package main

import (
	"path/filepath"
	"strings"
)

// docFormat is the high-level kind of file we're converting between.
// It maps from the file extension; lokmd doesn't peek at file
// contents.
type docFormat int

const (
	fmtUnknown docFormat = iota
	fmtMD
	fmtDOCX
	fmtPPTX
)

func (f docFormat) String() string {
	switch f {
	case fmtMD:
		return "md"
	case fmtDOCX:
		return "docx"
	case fmtPPTX:
		return "pptx"
	}
	return "unknown"
}

// formatFromExt maps a file extension to docFormat. .md and
// .markdown are both treated as markdown to match common conventions
// (GitHub, pandoc, etc.).
func formatFromExt(path string) docFormat {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".markdown":
		return fmtMD
	case ".docx":
		return fmtDOCX
	case ".pptx":
		return fmtPPTX
	}
	return fmtUnknown
}

// supportedConversion reports whether (in, out) is a pair lokmd
// knows how to handle. The supported directions are:
//
//	md ↔ docx
//	md ↔ pptx
//
// Same-format copies (e.g. md→md) are rejected — the user almost
// certainly meant something else and a silent no-op or pass-through
// would hide the typo. docx ↔ pptx isn't supported because LO
// can't go between document kinds and we don't want to silently
// drop slides or paragraphs.
func supportedConversion(in, out docFormat) bool {
	if in == fmtUnknown || out == fmtUnknown || in == out {
		return false
	}
	pairs := map[[2]docFormat]struct{}{
		{fmtMD, fmtDOCX}: {},
		{fmtDOCX, fmtMD}: {},
		{fmtMD, fmtPPTX}: {},
		{fmtPPTX, fmtMD}: {},
	}
	_, ok := pairs[[2]docFormat{in, out}]
	return ok
}
