package main

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"
)

func TestReserveTempPath_PropagatesCloseError(t *testing.T) {
	// Pre-close the file so reserveTempPath's own Close call returns
	// fs.ErrClosed. This drives the surface-the-Close-error path with
	// only stdlib primitives — no fake interface needed.
	tmp, err := os.CreateTemp("", "lokmd-test-*.fodp")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Close(); err != nil {
		t.Fatalf("pre-close: %v", err)
	}
	_, gotErr := reserveTempPath(tmp, nil)
	if !errors.Is(gotErr, fs.ErrClosed) {
		t.Fatalf("got %v, want chain containing fs.ErrClosed", gotErr)
	}
}

func TestReserveTempPath_PropagatesCreateError(t *testing.T) {
	want := errors.New("no space")
	_, err := reserveTempPath(nil, want)
	if !errors.Is(err, want) {
		t.Fatalf("got %v, want chain containing %v", err, want)
	}
}

func TestReserveTempPath_ReturnsName(t *testing.T) {
	tmp, err := os.CreateTemp("", "lokmd-test-*.fodp")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer os.Remove(tmp.Name())
	want := tmp.Name()
	got, err := reserveTempPath(tmp, nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != want {
		t.Errorf("path=%q, want %q", got, want)
	}
}

func TestParseFODPSlides_TwoSlides(t *testing.T) {
	xml := `<?xml version="1.0"?>
<office:document
  xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0"
  xmlns:draw="urn:oasis:names:tc:opendocument:xmlns:drawing:1.0"
  xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0">
<office:body><office:presentation>
  <draw:page draw:name="Hello">
    <draw:frame><draw:text-box>
      <text:p>title placeholder</text:p>
      <text:p>first body</text:p>
    </draw:text-box></draw:frame>
  </draw:page>
  <draw:page draw:name="World">
    <draw:frame><draw:text-box><text:p>second body</text:p></draw:text-box></draw:frame>
  </draw:page>
</office:presentation></office:body></office:document>`
	slides, err := parseFODPSlides([]byte(xml))
	if err != nil {
		t.Fatal(err)
	}
	if len(slides) != 2 {
		t.Fatalf("got %d slides, want 2", len(slides))
	}
	if slides[0].name != "Hello" || slides[1].name != "World" {
		t.Errorf("names = %q, %q", slides[0].name, slides[1].name)
	}
	if len(slides[0].paras) != 2 {
		t.Errorf("slide 0 paras = %v", slides[0].paras)
	}
}

// Master-page slides (LO's layout templates) live under
// office:master-styles and would otherwise leak into the slide list.
func TestParseFODPSlides_SkipsMasterPages(t *testing.T) {
	xml := `<?xml version="1.0"?>
<office:document
  xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0"
  xmlns:draw="urn:oasis:names:tc:opendocument:xmlns:drawing:1.0"
  xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0">
<office:master-styles>
  <draw:page draw:name="Default-Master">
    <draw:frame><draw:text-box><text:p>master frame</text:p></draw:text-box></draw:frame>
  </draw:page>
</office:master-styles>
<office:body><office:presentation>
  <draw:page draw:name="Real">
    <draw:frame><draw:text-box><text:p>real body</text:p></draw:text-box></draw:frame>
  </draw:page>
</office:presentation></office:body></office:document>`
	slides, err := parseFODPSlides([]byte(xml))
	if err != nil {
		t.Fatal(err)
	}
	if len(slides) != 1 {
		t.Fatalf("got %d slides, want 1 (master page should be skipped)", len(slides))
	}
	if slides[0].name != "Real" {
		t.Errorf("kept slide = %q", slides[0].name)
	}
}

func TestSplitTitleAndBody_PrefersMeaningfulPageName(t *testing.T) {
	s := fodpSlide{name: "Intro", paras: []string{"line 1", "line 2"}}
	title, body := splitTitleAndBody(s)
	if title != "Intro" {
		t.Errorf("title = %q, want Intro", title)
	}
	if len(body) != 2 || body[0] != "line 1" {
		t.Errorf("body = %v", body)
	}
}

// LO assigns "page1" / "Slide 1" placeholder names when the user
// didn't set one. Those should fall through to "use the first
// paragraph as the title" so the markdown looks user-meaningful.
func TestSplitTitleAndBody_FallsThroughAutoPageName(t *testing.T) {
	for _, name := range []string{"page1", "Page 2", "Slide 3", "slide-4"} {
		s := fodpSlide{name: name, paras: []string{"Real Title", "body"}}
		title, body := splitTitleAndBody(s)
		if title != "Real Title" {
			t.Errorf("name=%q: title=%q, want Real Title (auto-name should be skipped)", name, title)
		}
		if len(body) != 1 || body[0] != "body" {
			t.Errorf("name=%q: body=%v", name, body)
		}
	}
}

func TestSlidesToMarkdown_FormatsAsMarp(t *testing.T) {
	slides := []fodpSlide{
		{name: "First", paras: []string{"a", "b"}},
		{name: "Second", paras: []string{"c"}},
	}
	got := slidesToMarkdown(slides)
	if !strings.Contains(got, "# First") || !strings.Contains(got, "# Second") {
		t.Errorf("missing headings: %q", got)
	}
	if strings.Count(got, "---") != 1 {
		t.Errorf("want exactly 1 separator, got %q", got)
	}
}
