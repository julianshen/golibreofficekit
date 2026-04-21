//go:build lok_integration && (linux || darwin)

package lok

import (
	"os"
	"strings"
	"testing"
)

// TestIntegration_FullLifecycle exercises New → VersionInfo →
// SetAuthor → SetOptionalFeatures → TrimMemory → DumpState → Close
// in one process. LibreOffice's lok_init cannot be re-invoked within
// a single process even after destroy, so every integration check in
// this package has to share a single New/Close pair.
func TestIntegration_FullLifecycle(t *testing.T) {
	path := os.Getenv("LOK_PATH")
	if path == "" {
		t.Skip("LOK_PATH not set")
	}
	// Give LO its own scratch profile so parallel test binaries (this
	// package + internal/lokc running concurrently under go test ./...)
	// don't fight over ~/.config/libreoffice lock files.
	profile := "file://" + t.TempDir()
	o, err := New(path, WithUserProfile(profile))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer o.Close()

	vi, err := o.VersionInfo()
	if err != nil {
		t.Fatalf("VersionInfo: %v", err)
	}
	if vi.ProductName == "" {
		t.Error("ProductName is empty")
	}
	if !strings.HasPrefix(vi.ProductVersion, "24.8") && !strings.HasPrefix(vi.ProductVersion, "25.") {
		t.Logf("ProductVersion=%q (not a hard failure, but unexpected)", vi.ProductVersion)
	}

	if err := o.SetAuthor("CI Runner"); err != nil {
		t.Errorf("SetAuthor: %v", err)
	}
	if err := o.SetOptionalFeatures(FeatureDocumentPassword); err != nil {
		t.Errorf("SetOptionalFeatures: %v", err)
	}
	if err := o.TrimMemory(1); err != nil {
		t.Errorf("TrimMemory: %v", err)
	}
	if _, err := o.DumpState(); err != nil {
		t.Errorf("DumpState: %v", err)
	}
}
