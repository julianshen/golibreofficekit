package main

import (
	"strings"
	"testing"
)

func TestParseMarkdownSlides_SplitsOnHorizontalRule(t *testing.T) {
	input := `# Slide 1
First slide body.

---

# Slide 2
Second slide body.
`
	got := parseMarkdownSlides(input)
	if len(got) != 2 {
		t.Fatalf("got %d slides, want 2", len(got))
	}
	if got[0].title != "Slide 1" {
		t.Errorf("slide 0 title = %q, want %q", got[0].title, "Slide 1")
	}
	if got[1].title != "Slide 2" {
		t.Errorf("slide 1 title = %q, want %q", got[1].title, "Slide 2")
	}
	if !strings.Contains(got[0].body, "First slide body.") {
		t.Errorf("slide 0 body missing content: %q", got[0].body)
	}
	if !strings.Contains(got[1].body, "Second slide body.") {
		t.Errorf("slide 1 body missing content: %q", got[1].body)
	}
}

func TestParseMarkdownSlides_EmptyInputYieldsOneEmptySlide(t *testing.T) {
	got := parseMarkdownSlides("")
	if len(got) != 1 {
		t.Fatalf("got %d slides, want 1 (empty placeholder)", len(got))
	}
	if got[0].title != "" || got[0].body != "" {
		t.Errorf("empty input should yield empty slide, got title=%q body=%q", got[0].title, got[0].body)
	}
}

func TestParseMarkdownSlides_NoTitleHeading(t *testing.T) {
	got := parseMarkdownSlides("just a body, no heading")
	if len(got) != 1 {
		t.Fatalf("got %d slides, want 1", len(got))
	}
	if got[0].title != "" {
		t.Errorf("title without # should be empty, got %q", got[0].title)
	}
	if got[0].body != "just a body, no heading" {
		t.Errorf("body = %q, want full input", got[0].body)
	}
}

func TestParseMarkdownSlides_TrimsWhitespaceAroundSeparator(t *testing.T) {
	input := "# A\nfirst\n\n   ---   \n\n# B\nsecond"
	got := parseMarkdownSlides(input)
	if len(got) != 2 {
		t.Fatalf("got %d slides, want 2", len(got))
	}
	if got[0].title != "A" || got[1].title != "B" {
		t.Errorf("titles = %q, %q; want A, B", got[0].title, got[1].title)
	}
}

func TestParseMarkdownSlides_StripsMarpFrontMatter(t *testing.T) {
	input := `---
marp: true
theme: gaia
---

# Slide 1
body

---

# Slide 2
more
`
	got := parseMarkdownSlides(input)
	if len(got) != 2 {
		t.Fatalf("got %d slides, want 2", len(got))
	}
	if got[0].title != "Slide 1" {
		t.Errorf("front-matter not stripped: slide 0 title = %q", got[0].title)
	}
}

func TestParseMarkdownSlides_NoSeparatorYieldsSingleSlide(t *testing.T) {
	input := "# Only Slide\nWith body."
	got := parseMarkdownSlides(input)
	if len(got) != 1 {
		t.Fatalf("got %d slides, want 1", len(got))
	}
	if got[0].title != "Only Slide" {
		t.Errorf("title = %q", got[0].title)
	}
	if got[0].body != "With body." {
		t.Errorf("body = %q", got[0].body)
	}
}

func TestSlidesToFODP_ProducesValidXML(t *testing.T) {
	slides := []slide{
		{title: "Hello", body: "first body"},
		{title: "World", body: "second\nline"},
	}
	xml := slidesToFODP(slides)
	if !strings.HasPrefix(xml, "<?xml") {
		t.Errorf("output should start with XML declaration: %q", xml[:50])
	}
	for _, want := range []string{"Hello", "World", "first body", "second", "line"} {
		if !strings.Contains(xml, want) {
			t.Errorf("FODP missing expected text %q", want)
		}
	}
	// Per-slide draw:page; expect 2 occurrences.
	if got := strings.Count(xml, "<draw:page"); got != 2 {
		t.Errorf("got %d draw:page elements, want 2", got)
	}
}

func TestSplitBodyParagraphs_BulletsBecomeSeparateParagraphs(t *testing.T) {
	in := "- bullet one\n- bullet two"
	got := splitBodyParagraphs(in)
	if len(got) != 2 {
		t.Fatalf("got %d paragraphs, want 2", len(got))
	}
	if got[0] != "- bullet one" || got[1] != "- bullet two" {
		t.Errorf("got %q", got)
	}
}

func TestSplitBodyParagraphs_ProseStaysOneParagraph(t *testing.T) {
	in := "this is a paragraph\nwith a soft line break"
	got := splitBodyParagraphs(in)
	if len(got) != 1 {
		t.Errorf("prose with soft breaks should be one paragraph, got %d: %q", len(got), got)
	}
}

func TestSplitBodyParagraphs_BlankLineSplitsParagraphs(t *testing.T) {
	in := "first paragraph\n\nsecond paragraph"
	got := splitBodyParagraphs(in)
	if len(got) != 2 || got[0] != "first paragraph" || got[1] != "second paragraph" {
		t.Errorf("got %q", got)
	}
}

// FODP must escape XML metachars in slide titles/bodies so a doc
// like `<script>` doesn't break the output.
func TestSlidesToFODP_EscapesXMLMetachars(t *testing.T) {
	slides := []slide{{title: "<a&b>", body: `quote: "x" 'y'`}}
	xml := slidesToFODP(slides)
	if strings.Contains(xml, "<a&b>") {
		t.Errorf("title not escaped — raw <a&b> appears in output")
	}
	for _, want := range []string{"&lt;a&amp;b&gt;", "&#34;", "&#39;"} {
		// xml.EscapeString uses &#34; for " and &#39; for '.
		if !strings.Contains(xml, want) {
			t.Errorf("expected escaped sequence %q not found", want)
		}
	}
}
