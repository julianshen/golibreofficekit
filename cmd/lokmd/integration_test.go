//go:build lok_integration && (linux || darwin)

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestIntegration_AllFourPairs builds the lokmd binary and exercises
// every supported conversion direction against real LibreOffice via
// the LOK_PATH env. Mirrors lok/integration_test.go's gate behaviour:
// skipped when LOK_PATH is unset; runs under
// `make test-integration` (which sets GODEBUG=asyncpreemptoff=1).
//
// The test is one shared sequence so the binary's process model
// (one-shot New → Close per invocation) is exercised four times,
// and any state that leaks across invocations would surface here.
func TestIntegration_AllFourPairs(t *testing.T) {
	if os.Getenv("LOK_PATH") == "" {
		t.Skip("LOK_PATH not set")
	}

	work := t.TempDir()
	bin := filepath.Join(work, "lokmd")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		t.Fatalf("go build: %v", err)
	}

	src := filepath.Join(work, "deck.md")
	if err := os.WriteFile(src, []byte(marpFixture), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	docx := filepath.Join(work, "out.docx")
	pptx := filepath.Join(work, "out.pptx")
	mdFromDocx := filepath.Join(work, "from-docx.md")
	mdFromPptx := filepath.Join(work, "from-pptx.md")

	runOK(t, bin, "-in", src, "-out", docx)
	runOK(t, bin, "-in", src, "-out", pptx)
	runOK(t, bin, "-in", docx, "-out", mdFromDocx)
	runOK(t, bin, "-in", pptx, "-out", mdFromPptx)

	// PK\003\004 (zip header) for OOXML outputs.
	for _, p := range []string{docx, pptx} {
		assertZipMagic(t, p)
	}

	// Markdown round-trips must contain "Hello" and "World" — the
	// fixture's slide titles. Don't assert on bullet text since the
	// pptx round-trip can collapse list structure.
	assertContains(t, mdFromDocx, "Hello", "World")
	assertContains(t, mdFromPptx, "Hello", "World")

	// Negative: docx → pptx should be rejected with exit code 1.
	bad := filepath.Join(work, "bad.pptx")
	cmd := exec.Command(bin, "-in", docx, "-out", bad)
	cmd.Stderr = nil
	cmd.Stdout = nil
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Errorf("docx → pptx should fail; got success with output: %s", out)
	}
	if !strings.Contains(string(out), "unsupported conversion") {
		t.Errorf("expected 'unsupported conversion' error; got: %s", out)
	}
}

const marpFixture = `---
marp: true
theme: default
---

# Hello

First slide body.

---

# World

- bullet one
- bullet two
`

func runOK(t *testing.T, bin string, args ...string) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", bin, strings.Join(args, " "), err, out)
	}
}

func assertZipMagic(t *testing.T, path string) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Errorf("open %s: %v", path, err)
		return
	}
	defer f.Close()
	buf := make([]byte, 4)
	if _, err := f.Read(buf); err != nil {
		t.Errorf("read %s: %v", path, err)
		return
	}
	if !bytes.Equal(buf, []byte{0x50, 0x4b, 0x03, 0x04}) {
		t.Errorf("%s: missing zip magic, got % x", path, buf)
	}
}

func assertContains(t *testing.T, path string, wants ...string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("read %s: %v", path, err)
		return
	}
	s := string(body)
	for _, w := range wants {
		if !strings.Contains(s, w) {
			t.Errorf("%s missing %q\n--- contents ---\n%s", path, w, s)
		}
	}
}
