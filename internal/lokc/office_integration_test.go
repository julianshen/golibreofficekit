//go:build lok_integration && (linux || darwin)

package lokc

import (
	"os"
	"strings"
	"testing"
)

func TestIntegration_Hook_RoundTrip(t *testing.T) {
	path := os.Getenv("LOK_PATH")
	if path == "" {
		t.Skip("LOK_PATH not set")
	}
	lib, err := OpenLibrary(path)
	if err != nil {
		t.Fatalf("OpenLibrary: %v", err)
	}
	h, err := InvokeHook(lib, "")
	if err != nil {
		t.Fatalf("InvokeHook: %v", err)
	}
	if !h.IsValid() {
		t.Fatal("handle invalid after successful InvokeHook")
	}
	defer OfficeDestroy(h)

	ver := OfficeGetVersionInfo(h)
	if !strings.Contains(ver, "ProductVersion") {
		t.Errorf("version info missing ProductVersion: %q", ver)
	}

	if errStr := OfficeGetError(h); errStr != "" {
		t.Errorf("unexpected pending error: %q", errStr)
	}

	if state := OfficeDumpState(h); !strings.Contains(strings.ToLower(state), "libreoffice") {
		// dumpState content varies; require a non-empty result only.
		if state == "" {
			t.Error("dumpState returned empty string")
		}
	}
}
