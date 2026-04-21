//go:build lok_integration && (linux || darwin)

package lokc

import (
	"os"
	"strings"
	"testing"
)

// Known flake: when running the full integration suite
// (`LOK_PATH=… go test -tags=lok_integration ./...`), a fraction of
// runs crash with `fatal error: non-Go code set up signal handler
// without SA_ONSTACK flag`. LibreOffice installs SIGWINCH/SIGPIPE
// handlers that lack SA_ONSTACK; when either signal fires during or
// shortly after the hook call, the Go runtime aborts. No Go-side
// workaround is fully reliable. Running this test in isolation
// (`-run TestIntegration_Hook_RoundTrip`) is deterministic; re-run
// the suite if you hit the crash.

func TestIntegration_Hook_RoundTrip(t *testing.T) {
	path := os.Getenv("LOK_PATH")
	if path == "" {
		t.Skip("LOK_PATH not set")
	}
	lib, err := OpenLibrary(path)
	if err != nil {
		t.Fatalf("OpenLibrary: %v", err)
	}
	profile := "file://" + t.TempDir()
	h, err := InvokeHook(lib, profile)
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
