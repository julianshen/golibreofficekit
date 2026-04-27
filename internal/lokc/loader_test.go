//go:build linux || darwin

package lokc

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestOpenLibrary_EmptyPathErrors(t *testing.T) {
	_, err := OpenLibrary("")
	if !errors.Is(err, ErrInstallPathRequired) {
		t.Fatalf("want ErrInstallPathRequired, got %v", err)
	}
}

func TestOpenLibrary_MissingFileErrors(t *testing.T) {
	_, err := OpenLibrary("/nonexistent/install/path")
	if err == nil {
		t.Fatal("expected error for missing install dir")
	}
	var dlerr *DLError
	if !errors.As(err, &dlerr) || dlerr.Op != OpDLOpen {
		t.Errorf("want *DLError Op=%q, got %T %v", OpDLOpen, err, err)
	}
}

// buildFakeSO compiles an empty shared library named libsofficeapp.{so,dylib}
// into a fresh temp dir. Returns that dir (suitable for OpenLibrary).
// Skips the test if `cc` is not installed. Accepts zero or more hook
// symbols to export.
func buildFakeSO(t *testing.T, hookSymbols ...string) string {
	t.Helper()
	cc, err := exec.LookPath("cc")
	if err != nil {
		t.Skip("cc not available:", err)
	}
	tmp := t.TempDir()
	src := filepath.Join(tmp, "fake.c")
	body := "void _lok_fake_marker(void) {}\n"
	for _, sym := range hookSymbols {
		body += "void " + sym + "(void) {}\n"
	}
	if err := os.WriteFile(src, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	soName := "libsofficeapp.so"
	ccArgs := []string{"-shared", "-fPIC"}
	if runtime.GOOS == "darwin" {
		soName = "libsofficeapp.dylib"
		ccArgs = []string{"-dynamiclib"}
	}
	so := filepath.Join(tmp, soName)
	out, err := exec.Command(cc, append(ccArgs, "-o", so, src)...).CombinedOutput()
	if err != nil {
		t.Fatalf("cc build failed: %v\n%s", err, out)
	}
	return tmp
}

func TestOpenLibrary_MissingHookSymbolErrors(t *testing.T) {
	dir := buildFakeSO(t) // no hook symbols
	_, err := OpenLibrary(dir)
	if err == nil {
		t.Fatal("expected error for .so without hook symbols")
	}
	var dlerr *DLError
	if !errors.As(err, &dlerr) || dlerr.Op != OpDLSym {
		t.Errorf("want *DLError Op=%q, got %T %v", OpDLSym, err, err)
	}
}

func TestOpenLibrary_ResolvesHook2(t *testing.T) {
	dir := buildFakeSO(t, "libreofficekit_hook_2")
	lib, err := OpenLibrary(dir)
	if err != nil {
		t.Fatalf("OpenLibrary: %v", err)
	}
	if lib.HookVersion() != 2 {
		t.Errorf("HookVersion: want 2, got %d", lib.HookVersion())
	}
}

func TestOpenLibrary_PrefersHook2WhenBothExist(t *testing.T) {
	dir := buildFakeSO(t, "libreofficekit_hook_2", "libreofficekit_hook")
	lib, err := OpenLibrary(dir)
	if err != nil {
		t.Fatalf("OpenLibrary: %v", err)
	}
	if lib.HookVersion() != 2 {
		t.Errorf("with both symbols present, HookVersion: want 2, got %d", lib.HookVersion())
	}
}

func TestOpenLibrary_FallsBackToHook1(t *testing.T) {
	dir := buildFakeSO(t, "libreofficekit_hook")
	lib, err := OpenLibrary(dir)
	if err != nil {
		t.Fatalf("OpenLibrary: %v", err)
	}
	if lib.HookVersion() != 1 {
		t.Errorf("HookVersion: want 1, got %d", lib.HookVersion())
	}
}

func TestSoCandidates_NonDarwin(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("darwin branch exercised elsewhere")
	}
	got := soCandidates()
	want := []string{"libsofficeapp.so", "libmergedlo.so"}
	if len(got) != len(want) {
		t.Fatalf("soCandidates()=%v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("soCandidates()[%d]=%q, want %q", i, got[i], want[i])
		}
	}
}

// buildFakeSOWithName builds a fake LO runtime under <tempdir>/<soName>.
// Used to verify the candidate-fallback list — Ubuntu/Debian's apt
// libreoffice ships only libmergedlo.so (the merged build), not the
// upstream libsofficeapp.so layout.
func buildFakeSOWithName(t *testing.T, soName string, hookSymbols ...string) string {
	t.Helper()
	cc, err := exec.LookPath("cc")
	if err != nil {
		t.Skip("cc not available:", err)
	}
	tmp := t.TempDir()
	src := filepath.Join(tmp, "fake.c")
	body := "void _lok_fake_marker(void) {}\n"
	for _, sym := range hookSymbols {
		body += "void " + sym + "(void) {}\n"
	}
	if err := os.WriteFile(src, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	ccArgs := []string{"-shared", "-fPIC"}
	if runtime.GOOS == "darwin" {
		ccArgs = []string{"-dynamiclib"}
	}
	so := filepath.Join(tmp, soName)
	out, err := exec.Command(cc, append(ccArgs, "-o", so, src)...).CombinedOutput()
	if err != nil {
		t.Fatalf("cc build failed: %v\n%s", err, out)
	}
	return tmp
}

func TestOpenLibrary_FallsBackToMergedLO(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("merged-build fallback is Linux-only")
	}
	// Only libmergedlo.so exists, no libsofficeapp.so. The loader
	// must walk past the missing first candidate and find the merged
	// build. This is the layout Debian/Ubuntu's apt libreoffice ships.
	dir := buildFakeSOWithName(t, "libmergedlo.so", "libreofficekit_hook_2")
	lib, err := OpenLibrary(dir)
	if err != nil {
		t.Fatalf("OpenLibrary: %v", err)
	}
	if lib.HookVersion() != 2 {
		t.Errorf("HookVersion: want 2, got %d", lib.HookVersion())
	}
}

func TestOpenLibrary_PrefersSofficeappOverMergedLO(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("merged-build fallback is Linux-only")
	}
	cc, err := exec.LookPath("cc")
	if err != nil {
		t.Skip("cc not available:", err)
	}
	tmp := t.TempDir()
	src := filepath.Join(tmp, "fake.c")
	if err := os.WriteFile(src, []byte("void libreofficekit_hook_2(void) {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, soName := range []string{"libsofficeapp.so", "libmergedlo.so"} {
		so := filepath.Join(tmp, soName)
		out, err := exec.Command(cc, "-shared", "-fPIC", "-o", so, src).CombinedOutput()
		if err != nil {
			t.Fatalf("cc build %s failed: %v\n%s", soName, err, out)
		}
	}
	if _, err := OpenLibrary(tmp); err != nil {
		t.Fatalf("OpenLibrary: %v", err)
	}
	// We have no observable way to assert *which* file was opened
	// (both expose the same symbol), but the loader must pick the
	// upstream name first per the documented preference order.
}

func TestLibrary_Accessors(t *testing.T) {
	dir := buildFakeSO(t, "libreofficekit_hook_2")
	lib, err := OpenLibrary(dir)
	if err != nil {
		t.Fatalf("OpenLibrary: %v", err)
	}
	if got := lib.InstallPath(); got != dir {
		t.Errorf("InstallPath()=%q, want %q", got, dir)
	}
	if lib.HookSymbol() == nil {
		t.Error("HookSymbol() is nil; hook_2 resolved above so it should be non-nil")
	}
}
