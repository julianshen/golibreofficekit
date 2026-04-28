package main

import (
	"encoding/xml"
	"strconv"
	"strings"
)

// slide is one parsed markdown slide. title is the text after the
// first leading "# " heading; body is everything else from that
// slide's chunk (paragraphs and bullets, kept verbatim).
type slide struct {
	title string
	body  string
}

// parseMarkdownSlides splits markdown into slides on horizontal-rule
// separators (`---` on its own line, with optional surrounding
// whitespace) — the Marp/Marpit convention. For each slide chunk,
// the first leading "# " heading becomes the title; the remainder
// becomes the body. A YAML-style Marp front-matter block (a `---`
// at the very top with content and a closing `---`) is stripped
// before splitting. Empty input yields a single empty slide so
// callers always have something to render.
//
// This is a deliberately minimal markdown reader — no full CommonMark
// support, no nested formatting parsing. The goal is "give LO
// something it can present" not "perfect markdown rendering."
func parseMarkdownSlides(md string) []slide {
	md = stripMarpFrontMatter(md)
	chunks := splitOnHR(md)
	if len(chunks) == 0 {
		return []slide{{}}
	}
	out := make([]slide, 0, len(chunks))
	for _, c := range chunks {
		out = append(out, parseOneSlide(c))
	}
	return out
}

// stripMarpFrontMatter removes a leading `---\n…\n---` block. The
// directives inside (e.g. `marp: true`, `theme: gaia`) aren't
// honoured — they exist only to keep the source file Marp-compatible.
// If the document doesn't start with `---`, nothing is stripped.
func stripMarpFrontMatter(md string) string {
	lines := strings.Split(md, "\n")
	if len(lines) < 2 || strings.TrimSpace(lines[0]) != "---" {
		return md
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[i+1:], "\n")
		}
	}
	// Unterminated front-matter — leave the source untouched so the
	// user can see something rather than mysteriously losing content.
	return md
}

// splitOnHR splits md on lines that are exactly "---" after trimming
// surrounding whitespace. A leading or trailing empty chunk produced
// by a separator at the start/end is dropped — callers expect one
// slide per actual content block.
func splitOnHR(md string) []string {
	lines := strings.Split(md, "\n")
	var out []string
	var cur []string
	flush := func() {
		out = append(out, strings.Join(cur, "\n"))
		cur = cur[:0]
	}
	for _, ln := range lines {
		if strings.TrimSpace(ln) == "---" {
			flush()
			continue
		}
		cur = append(cur, ln)
	}
	flush()
	// Drop chunks that are entirely whitespace (empty leading/
	// trailing slides from separators on the boundary).
	pruned := out[:0]
	for _, c := range out {
		if strings.TrimSpace(c) != "" {
			pruned = append(pruned, c)
		}
	}
	return pruned
}

// parseOneSlide pulls a leading "# " heading out as the title and
// keeps the rest as body. Both title and body have surrounding
// whitespace trimmed.
func parseOneSlide(chunk string) slide {
	chunk = strings.Trim(chunk, "\n")
	lines := strings.Split(chunk, "\n")
	if len(lines) == 0 {
		return slide{}
	}
	first := strings.TrimSpace(lines[0])
	if rest, ok := strings.CutPrefix(first, "# "); ok {
		body := strings.Trim(strings.Join(lines[1:], "\n"), "\n")
		return slide{title: strings.TrimSpace(rest), body: body}
	}
	return slide{body: strings.Trim(chunk, "\n")}
}

// slidesToFODP renders slides as a Flat OpenDocument Presentation
// (.fodp) XML document. The result can be saved to disk and loaded
// by LibreOffice, which then can SaveAs("pptx") via the standard
// Impress export filter — the round-trip is the path md → pptx
// uses since LO has no direct markdown→Impress filter.
//
// Each slide becomes a draw:page with two text frames: a title
// (above) and a body (below). Body text is split on blank lines
// into paragraphs.
func slidesToFODP(slides []slide) string {
	var b strings.Builder
	b.WriteString(fodpPrologue)
	for i, s := range slides {
		writeSlidePage(&b, i, s)
	}
	b.WriteString(fodpEpilogue)
	return b.String()
}

const fodpPrologue = `<?xml version="1.0" encoding="UTF-8"?>
<office:document xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:draw="urn:oasis:names:tc:opendocument:xmlns:drawing:1.0" xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0" xmlns:svg="urn:oasis:names:tc:opendocument:xmlns:svg-compatible:1.0" office:version="1.2" office:mimetype="application/vnd.oasis.opendocument.presentation"><office:body><office:presentation>`

const fodpEpilogue = `</office:presentation></office:body></office:document>`

func writeSlidePage(b *strings.Builder, idx int, s slide) {
	b.WriteString(`<draw:page draw:name="`)
	xml.EscapeText(b, []byte(slidePageName(idx, s)))
	b.WriteString(`">`)
	if s.title != "" {
		b.WriteString(`<draw:frame draw:name="title" svg:width="20cm" svg:height="2cm" svg:x="2cm" svg:y="1cm"><draw:text-box><text:p>`)
		xml.EscapeText(b, []byte(s.title))
		b.WriteString(`</text:p></draw:text-box></draw:frame>`)
	}
	if s.body != "" {
		b.WriteString(`<draw:frame draw:name="body" svg:width="20cm" svg:height="14cm" svg:x="2cm" svg:y="4cm"><draw:text-box>`)
		// Split the body on blank lines into paragraphs; each
		// paragraph wraps in its own text:p so LO renders multi-
		// line content as separate lines instead of one long run.
		for _, para := range splitBodyParagraphs(s.body) {
			b.WriteString(`<text:p>`)
			xml.EscapeText(b, []byte(para))
			b.WriteString(`</text:p>`)
		}
		b.WriteString(`</draw:text-box></draw:frame>`)
	}
	b.WriteString(`</draw:page>`)
}

// slidePageName returns the title for use as the draw:name (LO uses
// this in slide-panel labels), falling back to "Slide N".
func slidePageName(idx int, s slide) string {
	if s.title != "" {
		return s.title
	}
	return "Slide " + strconv.Itoa(idx+1)
}

// splitBodyParagraphs turns a slide body into the paragraphs we want
// LO to render as separate text:p elements. Splits run from coarsest
// to finest:
//
//  1. Blank lines (\n\n) — true Markdown paragraph boundaries.
//  2. Within each paragraph, any line that starts with "- " or "* "
//     is treated as its own paragraph so bullet lists don't collapse
//     into one run when round-tripping md → pptx → md. (LO's Impress
//     filter doesn't preserve markdown's <ul> structure; one-bullet-
//     per-paragraph is the lossless thing we can do without a full
//     CommonMark renderer.)
func splitBodyParagraphs(body string) []string {
	var out []string
	for _, block := range strings.Split(body, "\n\n") {
		block = strings.Trim(block, "\n")
		if block == "" {
			continue
		}
		out = append(out, splitBulletLines(block)...)
	}
	if len(out) == 0 && strings.TrimSpace(body) != "" {
		out = []string{body}
	}
	return out
}

// splitBulletLines breaks a block into per-line paragraphs when the
// block looks like a bullet list. A non-bullet block is returned as
// a single-element slice so prose paragraphs stay intact.
func splitBulletLines(block string) []string {
	lines := strings.Split(block, "\n")
	hasBullet := false
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* ") {
			hasBullet = true
			break
		}
	}
	if !hasBullet {
		return []string{block}
	}
	out := make([]string, 0, len(lines))
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}
