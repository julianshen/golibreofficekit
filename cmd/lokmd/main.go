// lokmd converts between Markdown and Office formats using
// LibreOffice's built-in filters where they exist, and a small
// Marp-compatible markdown→ODP pipeline where they don't.
//
// Supported pairs (any direction):
//
//	.md/.markdown ↔ .docx   via LO's Markdown filter
//	.md/.markdown ↔ .pptx   via Marp-style "---" slide separators
//
// The markdown side follows Marp/Marpit conventions
// (https://marpit.marp.app): `---` on its own line separates slides,
// a leading YAML front-matter block (between two `---`) is stripped,
// and the first `# ` heading per slide becomes the slide title.
//
// Examples:
//
//	# Round-trip notes through Word
//	lokmd -in notes.md   -out notes.docx
//	lokmd -in notes.docx -out notes.md
//
//	# Marp-style deck → PowerPoint
//	lokmd -in deck.md  -out deck.pptx
//	lokmd -in deck.pptx -out deck.md
//
// LibreOffice install path comes from -lo-path, then $LOK_PATH,
// then auto-detect (Ubuntu/Debian, Fedora, macOS .app).
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/julianshen/golibreofficekit/internal/cli"
	"github.com/julianshen/golibreofficekit/lok"
)

func main() {
	in := flag.String("in", "", "input document path (required)")
	out := flag.String("out", "", "output path; format inferred from extension (required)")
	loPath := flag.String("lo-path", os.Getenv("LOK_PATH"), "LibreOffice install path; defaults to $LOK_PATH then auto-detect")
	flag.Parse()

	if *in == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "lokmd: -in and -out are required")
		flag.Usage()
		os.Exit(2)
	}

	resolved, err := cli.ResolveLOPath(*loPath, cli.DefaultLOPathCandidates)
	if err != nil {
		cli.Die("lokmd", "%v", err)
	}

	if err := convert(resolved, *in, *out); err != nil {
		cli.Die("lokmd", "%v", err)
	}
}

// convert routes (in, out) to one of the four supported pipelines.
// All paths share the same LO Office handle, opened once per
// invocation; lok.New is the dominant cost so multi-step pipelines
// (md → fodp → pptx) reuse a single Office.
func convert(loPath, inPath, outPath string) error {
	inFmt := formatFromExt(inPath)
	outFmt := formatFromExt(outPath)
	if !supportedConversion(inFmt, outFmt) {
		return fmt.Errorf("unsupported conversion %s → %s; supported pairs are md↔docx and md↔pptx",
			displayFormat(inPath, inFmt), displayFormat(outPath, outFmt))
	}

	o, err := lok.New(loPath)
	if err != nil {
		return fmt.Errorf("init LibreOffice at %s: %w", loPath, err)
	}
	defer o.Close()

	switch {
	case inFmt == fmtMD && outFmt == fmtDOCX:
		return mdToOfficeViaFilter(o, inPath, outPath, "docx")
	case inFmt == fmtDOCX && outFmt == fmtMD:
		return officeToMd(o, inPath, outPath)
	case inFmt == fmtMD && outFmt == fmtPPTX:
		return mdToPPTX(o, inPath, outPath)
	case inFmt == fmtPPTX && outFmt == fmtMD:
		return pptxToMd(o, inPath, outPath)
	}
	return fmt.Errorf("unreachable: %v → %v", inFmt, outFmt)
}

// mdToOfficeViaFilter is the trivial path: LO's Markdown import +
// any Writer-side export filter. format is the SaveAs filter name
// ("docx", "odt", etc.).
func mdToOfficeViaFilter(o *lok.Office, inPath, outPath, format string) error {
	doc, err := o.Load(inPath)
	if err != nil {
		return fmt.Errorf("load %s: %w", inPath, err)
	}
	defer doc.Close()
	if err := doc.SaveAs(outPath, format, ""); err != nil {
		_ = os.Remove(outPath)
		return fmt.Errorf("save %s: %w", outPath, err)
	}
	return nil
}

// officeToMd loads a Writer-readable file (e.g. .docx) and exports
// markdown via LO's Markdown filter. The filter is Writer-only —
// pptx and other Impress sources need pptxToMd instead.
func officeToMd(o *lok.Office, inPath, outPath string) error {
	doc, err := o.Load(inPath)
	if err != nil {
		return fmt.Errorf("load %s: %w", inPath, err)
	}
	defer doc.Close()
	if err := doc.SaveAs(outPath, "md", ""); err != nil {
		_ = os.Remove(outPath)
		return fmt.Errorf("save %s: %w", outPath, err)
	}
	return nil
}

// mdToPPTX uses a two-step pipeline: parse the markdown into
// Marp-style slides (Go-side), render an OpenDocument flat-XML
// presentation (.fodp) into a temp file, then have LO load that and
// SaveAs the .pptx. LO's Impress filter handles .fodp natively, so
// this works without any direct .pptx XML emission.
func mdToPPTX(o *lok.Office, inPath, outPath string) error {
	md, err := os.ReadFile(inPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", inPath, err)
	}
	slides := parseMarkdownSlides(string(md))
	fodpPath := filepath.Join(os.TempDir(),
		fmt.Sprintf("lokmd-%d.fodp", os.Getpid()))
	if err := os.WriteFile(fodpPath, []byte(slidesToFODP(slides)), 0o644); err != nil {
		return fmt.Errorf("write fodp temp: %w", err)
	}
	defer os.Remove(fodpPath)

	doc, err := o.Load(fodpPath)
	if err != nil {
		return fmt.Errorf("load fodp: %w", err)
	}
	defer doc.Close()
	if err := doc.SaveAs(outPath, "pptx", ""); err != nil {
		_ = os.Remove(outPath)
		return fmt.Errorf("save pptx: %w", err)
	}
	return nil
}

// pptxToMd loads a presentation and walks each slide via the binding,
// producing Marp-style markdown.
func pptxToMd(o *lok.Office, inPath, outPath string) error {
	doc, err := o.Load(inPath)
	if err != nil {
		return fmt.Errorf("load %s: %w", inPath, err)
	}
	defer doc.Close()
	md, err := extractSlidesAsMarkdown(doc)
	if err != nil {
		return err
	}
	if err := os.WriteFile(outPath, []byte(md), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}
	return nil
}

// displayFormat is a small helper for the "unsupported pair" error
// message — names the format string when known, otherwise echoes the
// extension so the user can spot a typo at a glance.
func displayFormat(path string, f docFormat) string {
	if f != fmtUnknown {
		return f.String()
	}
	if ext := filepath.Ext(path); ext != "" {
		return ext
	}
	return "(no extension)"
}
