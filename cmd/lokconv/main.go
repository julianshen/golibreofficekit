// lokconv converts an office document (Writer/Calc/Impress/Draw)
// to PDF or PNG using the golibreofficekit binding. Output format
// is inferred from the -out file extension.
//
// Examples:
//
//	# Whole document as PDF (uses LO's PDF export filter)
//	lokconv -in report.docx -out report.pdf
//
//	# A specific page rendered as PNG at 1.5× DPI (144 DPI)
//	lokconv -in deck.pptx -out slide-2.png -page 1 -dpi 1.5
//
// LibreOffice install path is taken from -lo-path, then $LOK_PATH,
// then a small list of platform-default candidates.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/julianshen/golibreofficekit/lok"
)

type outFormat int

const (
	fmtUnknown outFormat = iota
	fmtPDF
	fmtPNG
)

// defaultLOPathCandidates is the auto-detect list when neither
// -lo-path nor $LOK_PATH is set. Order matters — the first existing
// directory wins.
var defaultLOPathCandidates = []string{
	"/usr/lib/libreoffice/program",                      // Debian/Ubuntu (apt)
	"/usr/lib64/libreoffice/program",                    // Fedora/RHEL
	"/Applications/LibreOffice.app/Contents/Frameworks", // macOS .app bundle
}

func main() {
	in := flag.String("in", "", "input document path (required)")
	out := flag.String("out", "", "output path; format inferred from .pdf or .png extension (required)")
	page := flag.Int("page", 0, "page index for PNG output (0-based; ignored for PDF)")
	dpi := flag.Float64("dpi", 1.0, "DPI scale for PNG (1.0 = 96 DPI)")
	loPath := flag.String("lo-path", os.Getenv("LOK_PATH"), "LibreOffice install path; defaults to $LOK_PATH then auto-detect")
	flag.Parse()

	if *in == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "lokconv: -in and -out are required")
		flag.Usage()
		os.Exit(2)
	}

	resolved, err := resolveLOPath(*loPath, defaultLOPathCandidates)
	if err != nil {
		die("%v", err)
	}

	if err := convert(resolved, *in, *out, *page, *dpi); err != nil {
		die("%v", err)
	}
}

// outputFormatFromPath maps a file extension to outFormat.
func outputFormatFromPath(path string) outFormat {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".pdf":
		return fmtPDF
	case ".png":
		return fmtPNG
	}
	return fmtUnknown
}

// resolveLOPath returns explicit if non-empty (verifying it is a
// directory), otherwise the first existing directory in candidates.
// Returns an error if explicit is set but isn't a directory, or if
// no candidate exists.
func resolveLOPath(explicit string, candidates []string) (string, error) {
	if explicit != "" {
		st, err := os.Stat(explicit)
		if err != nil {
			return "", fmt.Errorf("lo-path %q: %w", explicit, err)
		}
		if !st.IsDir() {
			return "", fmt.Errorf("lo-path %q is not a directory", explicit)
		}
		return explicit, nil
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			return c, nil
		}
	}
	return "", errors.New("LibreOffice install not found; pass -lo-path or set $LOK_PATH")
}

// convert opens LO at loPath, loads inPath, and writes PDF or PNG to
// outPath based on the output extension. page and dpiScale apply to
// PNG output only.
func convert(loPath, inPath, outPath string, page int, dpiScale float64) error {
	format := outputFormatFromPath(outPath)
	if format == fmtUnknown {
		return fmt.Errorf("unsupported output extension on %q (want .pdf or .png)", outPath)
	}

	o, err := lok.New(loPath)
	if err != nil {
		return fmt.Errorf("init LibreOffice at %s: %w", loPath, err)
	}
	defer o.Close()

	absIn, err := filepath.Abs(inPath)
	if err != nil {
		return fmt.Errorf("resolve input path: %w", err)
	}
	doc, err := o.Load(absIn)
	if err != nil {
		return fmt.Errorf("load %s: %w", absIn, err)
	}
	defer doc.Close()

	switch format {
	case fmtPDF:
		absOut, err := filepath.Abs(outPath)
		if err != nil {
			return fmt.Errorf("resolve output path: %w", err)
		}
		if err := doc.SaveAs(absOut, "pdf", ""); err != nil {
			return fmt.Errorf("export PDF: %w", err)
		}
		return nil
	case fmtPNG:
		if err := doc.InitializeForRendering(""); err != nil {
			return fmt.Errorf("initialize for rendering: %w", err)
		}
		pngBytes, err := doc.RenderPagePNG(page, dpiScale)
		if err != nil {
			return fmt.Errorf("render PNG (page=%d, dpi=%.2f): %w", page, dpiScale, err)
		}
		if err := os.WriteFile(outPath, pngBytes, 0o644); err != nil {
			return fmt.Errorf("write PNG: %w", err)
		}
		return nil
	}
	return fmt.Errorf("unreachable: format=%d", format)
}

func die(format string, args ...any) {
	fmt.Fprintln(os.Stderr, "lokconv: "+fmt.Sprintf(format, args...))
	os.Exit(1)
}
