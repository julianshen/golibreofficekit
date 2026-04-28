package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveLOPath_PrefersExplicit(t *testing.T) {
	dir := t.TempDir()
	got, err := ResolveLOPath(dir, []string{"/should/not/be/checked"})
	if err != nil {
		t.Fatalf("explicit path: %v", err)
	}
	if got != dir {
		t.Errorf("got %q, want %q", got, dir)
	}
}

func TestResolveLOPath_RejectsExplicitMissing(t *testing.T) {
	ghost := filepath.Join(t.TempDir(), "no", "such", "dir")
	if _, err := ResolveLOPath(ghost, nil); err == nil {
		t.Errorf("expected error for missing explicit path")
	}
}

func TestResolveLOPath_RejectsExplicitNonDir(t *testing.T) {
	f := filepath.Join(t.TempDir(), "notadir")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveLOPath(f, nil); err == nil {
		t.Errorf("expected error for non-directory explicit path")
	}
}

func TestResolveLOPath_AutoDetect(t *testing.T) {
	a := t.TempDir()
	b := t.TempDir()
	got, err := ResolveLOPath("", []string{filepath.Join(a, "missing"), b})
	if err != nil {
		t.Fatalf("auto-detect: %v", err)
	}
	if got != b {
		t.Errorf("got %q, want %q", got, b)
	}
}

func TestResolveLOPath_NoneFound(t *testing.T) {
	if _, err := ResolveLOPath("", []string{"/does/not/exist", "/nope"}); err == nil {
		t.Errorf("expected error when no candidate exists")
	}
}

// DefaultLOPathCandidates must be non-empty and start with the most
// common Linux path so distros that match it short-circuit fast.
func TestDefaultLOPathCandidates_ShapeStable(t *testing.T) {
	if len(DefaultLOPathCandidates) == 0 {
		t.Fatal("DefaultLOPathCandidates is empty")
	}
	if DefaultLOPathCandidates[0] != "/usr/lib/libreoffice/program" {
		t.Errorf("first candidate=%q, want /usr/lib/libreoffice/program (Debian/Ubuntu)",
			DefaultLOPathCandidates[0])
	}
}
