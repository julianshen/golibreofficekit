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
	if !errors.As(err, &dlerr) || dlerr.Op != "dlopen" {
		t.Errorf("want *DLError Op=dlopen, got %T %v", err, err)
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
	if !errors.As(err, &dlerr) || dlerr.Op != "dlsym" {
		t.Errorf("want *DLError Op=dlsym, got %T %v", err, err)
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

func TestSoFilename_NonDarwin(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("darwin branch exercised elsewhere")
	}
	if got := soFilename(); got != "libsofficeapp.so" {
		t.Errorf("soFilename()=%q, want libsofficeapp.so", got)
	}
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
