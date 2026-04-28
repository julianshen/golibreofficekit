package main

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/julianshen/golibreofficekit/lok"
)

// extractSlidesAsMarkdown loads a presentation through LOK, has LO
// save it as Flat OpenDocument Presentation (.fodp), and walks the
// resulting XML to harvest each slide's title and body text.
//
// This avoids the async SelectAll/GetTextSelection dance: LO's saveAs
// is synchronous, and the .fodp produced by the "OpenDocument
// Presentation Flat XML" filter has every text run materialised
// inside <draw:page>/<text:p> elements that we can iterate
// deterministically.
func extractSlidesAsMarkdown(doc *lok.Document) (string, error) {
	tmp := filepath.Join(os.TempDir(),
		fmt.Sprintf("lokmd-extract-%d.fodp", os.Getpid()))
	defer os.Remove(tmp)
	if err := doc.SaveAs(tmp, "fodp", ""); err != nil {
		return "", fmt.Errorf("save flat-xml fodp: %w", err)
	}
	xmlBytes, err := os.ReadFile(tmp)
	if err != nil {
		return "", fmt.Errorf("read fodp: %w", err)
	}
	slides, err := parseFODPSlides(xmlBytes)
	if err != nil {
		return "", err
	}
	return slidesToMarkdown(slides), nil
}

// fodpSlide is one parsed slide from the flat XML — a name (the
// LO-assigned page label) and a list of text paragraphs harvested
// from any <text:p> descendant of the slide's <draw:page>.
type fodpSlide struct {
	name  string
	paras []string
}

// parseFODPSlides walks the FODP XML stream and returns one
// fodpSlide per <draw:page>. Text inside <text:p> elements is
// concatenated as plain text (no formatting preserved). LO sometimes
// inserts a layout-fragment <draw:page> in the master-page section;
// the parser skips any <draw:page> that lives under <office:master-
// styles> by tracking depth into that subtree.
func parseFODPSlides(xmlBytes []byte) ([]fodpSlide, error) {
	dec := xml.NewDecoder(strings.NewReader(string(xmlBytes)))
	var (
		slides   []fodpSlide
		cur      *fodpSlide
		text     strings.Builder
		inText   int
		inMaster int // depth into office:master-styles; skip pages there
	)
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "master-styles":
				inMaster++
			case "page":
				if inMaster > 0 {
					continue
				}
				slides = append(slides, fodpSlide{name: pageName(t)})
				cur = &slides[len(slides)-1]
			case "p":
				if cur != nil && t.Name.Space == "urn:oasis:names:tc:opendocument:xmlns:text:1.0" {
					inText++
					text.Reset()
				}
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "master-styles":
				if inMaster > 0 {
					inMaster--
				}
			case "p":
				if cur != nil && t.Name.Space == "urn:oasis:names:tc:opendocument:xmlns:text:1.0" && inText > 0 {
					inText--
					if s := strings.TrimSpace(text.String()); s != "" {
						cur.paras = append(cur.paras, s)
					}
					text.Reset()
				}
			case "page":
				if inMaster == 0 {
					cur = nil
				}
			}
		case xml.CharData:
			if inText > 0 {
				text.Write(t)
			}
		}
	}
	return slides, nil
}

// pageName returns the value of the <draw:page draw:name="…">
// attribute, or "" if absent.
func pageName(t xml.StartElement) string {
	for _, a := range t.Attr {
		if a.Name.Local == "name" && a.Name.Space == "urn:oasis:names:tc:opendocument:xmlns:drawing:1.0" {
			return a.Value
		}
	}
	return ""
}

// slidesToMarkdown turns the parsed slides into a Marp-shaped
// markdown document: one `# title` heading per slide, body text as
// paragraphs, and `---` separators between slides. The first
// paragraph is treated as the slide title when the LO page name is
// just the auto-generated "page1"/"Slide 1" placeholder; users
// usually want the in-frame heading to land on the H1, not the
// LO-internal page label.
func slidesToMarkdown(slides []fodpSlide) string {
	var b strings.Builder
	for i, s := range slides {
		title, body := splitTitleAndBody(s)
		b.WriteString("# ")
		if title == "" {
			fmt.Fprintf(&b, "Slide %d", i+1)
		} else {
			b.WriteString(title)
		}
		b.WriteString("\n\n")
		for _, p := range body {
			b.WriteString(p)
			b.WriteString("\n\n")
		}
		if i < len(slides)-1 {
			b.WriteString("---\n\n")
		}
	}
	return b.String()
}

// splitTitleAndBody pulls the title out of the slide. Preference
// order: a meaningful page name (anything that isn't an auto-
// generated "page1"/"Slide 1" / "page-1" placeholder) wins; else
// the first paragraph becomes the title and the rest is body.
func splitTitleAndBody(s fodpSlide) (title string, body []string) {
	if name := strings.TrimSpace(s.name); name != "" && !isAutoPageName(name) {
		return name, s.paras
	}
	if len(s.paras) > 0 {
		return s.paras[0], s.paras[1:]
	}
	return "", nil
}

// isAutoPageName reports whether name looks like LO's default
// page-N / Slide N placeholder (the user didn't set a real title).
func isAutoPageName(name string) bool {
	lower := strings.ToLower(name)
	for _, prefix := range []string{"page", "slide"} {
		if rest, ok := strings.CutPrefix(lower, prefix); ok {
			rest = strings.TrimLeft(rest, "- ")
			if _, err := strconv.ParseInt(rest, 10, 64); err == nil {
				return true
			}
		}
	}
	return false
}
