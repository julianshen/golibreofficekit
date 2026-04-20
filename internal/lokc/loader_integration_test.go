//go:build lok_integration

package lokc

import (
	"os"
	"testing"
)

func TestIntegration_OpenRealLOK(t *testing.T) {
	path := os.Getenv("LOK_PATH")
	if path == "" {
		t.Skip("LOK_PATH not set")
	}
	lib, err := OpenLibrary(path)
	if err != nil {
		t.Fatalf("OpenLibrary(%q): %v", path, err)
	}
	if lib.HookSymbol() == nil {
		t.Fatal("hook symbol is nil")
	}
	if v := lib.HookVersion(); v != 1 && v != 2 {
		t.Errorf("HookVersion: want 1 or 2, got %d", v)
	}
	if lib.InstallPath() != path {
		t.Errorf("InstallPath: want %q, got %q", path, lib.InstallPath())
	}
}
