//go:build lok_integration && (linux || darwin)

package lok

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// LibreOffice installs SIGWINCH/SIGPIPE signal handlers that lack
// SA_ONSTACK. Go 1.14+ async preemption (which uses SIGURG to force
// goroutine preemption) interacts with that and aborts the runtime
// with "non-Go code set up signal handler without SA_ONSTACK flag".
// The Makefile's test-integration target sets GODEBUG=asyncpreemptoff=1
// to disable async preemption for the duration of the test run, which
// avoids the crash deterministically. Run this test via
//   make test-integration
// NOT
//   go test -tags=lok_integration ./...
// unless you set GODEBUG=asyncpreemptoff=1 yourself.

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

	fixture, err := filepath.Abs("../testdata/hello.odt")
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	doc, err := o.Load(fixture)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	t.Cleanup(func() { doc.Close() })

	if got := doc.Type(); got != TypeText {
		t.Errorf("Type()=%v, want Text", got)
	}

	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "roundtrip.odt")
	if err := doc.SaveAs(outPath, "odt", ""); err != nil {
		t.Errorf("SaveAs: %v", err)
	}
	if st, err := os.Stat(outPath); err != nil {
		t.Errorf("SaveAs output missing: %v", err)
	} else if st.Size() == 0 {
		t.Error("SaveAs output is zero bytes")
	}

	pdfPath := filepath.Join(outDir, "out.pdf")
	if err := doc.SaveAs(pdfPath, "pdf", ""); err != nil {
		t.Errorf("SaveAs pdf: %v", err)
	}

	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	doc2, err := o.LoadFromReader(bytes.NewReader(data), "odt")
	if err != nil {
		t.Fatalf("LoadFromReader: %v", err)
	}
	defer doc2.Close()
	if got := doc2.Type(); got != TypeText {
		t.Errorf("reader-loaded Type()=%v, want Text", got)
	}

	// View round-trip on doc.

	initialView, err := doc.View()
	if err != nil {
		t.Fatalf("View (initial): %v", err)
	}
	initialViews, err := doc.Views()
	if err != nil {
		t.Fatalf("Views (initial): %v", err)
	}
	if len(initialViews) == 0 {
		t.Logf("initial Views() empty; LO may have set this up lazily")
	}

	newView, err := doc.CreateView()
	if err != nil {
		t.Fatalf("CreateView: %v", err)
	}
	if newView == initialView {
		t.Errorf("CreateView returned same ID as initial view")
	}

	views, err := doc.Views()
	if err != nil {
		t.Fatalf("Views: %v", err)
	}
	var found bool
	for _, v := range views {
		if v == newView {
			found = true
		}
	}
	if !found {
		t.Errorf("new view %d not in Views() list %v", newView, views)
	}

	if err := doc.SetView(newView); err != nil {
		t.Errorf("SetView: %v", err)
	}
	if got, err := doc.View(); err != nil {
		t.Errorf("View after SetView: %v", err)
	} else if got != newView {
		t.Errorf("View()=%d after SetView(%d)", got, newView)
	}

	if err := doc.SetViewReadOnly(newView, true); err != nil {
		t.Errorf("SetViewReadOnly: %v", err)
	}
	if err := doc.SetViewLanguage(newView, "en-US"); err != nil {
		t.Errorf("SetViewLanguage: %v", err)
	}
	if err := doc.SetAccessibilityState(newView, false); err != nil {
		t.Errorf("SetAccessibilityState: %v", err)
	}
	if err := doc.SetViewTimezone(newView, "UTC"); err != nil {
		t.Errorf("SetViewTimezone: %v", err)
	}

	if err := doc.DestroyView(newView); err != nil {
		t.Errorf("DestroyView: %v", err)
	}

	// Restore the initial view as active so any subsequent subtest
	// starts from a deterministic state rather than whatever
	// fallback LO picked after DestroyView.
	if err := doc.SetView(initialView); err != nil {
		t.Errorf("SetView restore: %v", err)
	}

	// Part + size round-trip on doc. Writer legitimately returns 0
	// parts — "parts" means Calc sheets / Impress slides. We
	// verify the per-part methods tolerate the zero-part case and
	// only cross-check part-indexed reads when nParts > 0.

	nParts, err := doc.Parts()
	if err != nil {
		t.Fatalf("Parts: %v", err)
	}
	if nParts < 0 {
		t.Fatalf("Parts returned %d; want >=0", nParts)
	}

	if nParts > 0 {
		activePart, err := doc.Part()
		if err != nil {
			t.Fatalf("Part: %v", err)
		}
		if activePart < 0 || activePart >= nParts {
			t.Errorf("Part out of range: got %d, want [0, %d)", activePart, nParts)
		}

		if _, err := doc.PartName(activePart); err != nil {
			t.Errorf("PartName(%d): %v", activePart, err)
		}

		partHash, err := doc.PartHash(activePart)
		if err != nil {
			t.Errorf("PartHash(%d): %v", activePart, err)
		}
		if partHash == "" {
			t.Log("PartHash empty; LO may not compute it for this doc type")
		}

		info, err := doc.PartInfo(activePart)
		if err != nil {
			t.Errorf("PartInfo(%d): %v (want nil err; empty payload is OK)", activePart, err)
		}
		if info == nil {
			t.Logf("PartInfo(%d) empty (expected for non-Impress docs)", activePart)
		}

		if err := doc.SetPart(0); err != nil {
			t.Errorf("SetPart(0): %v", err)
		}
	} else {
		t.Logf("Parts=0 (Writer documents don't enumerate parts); skipping per-part subtests")
	}

	// DocumentSize() and PartPageRectangles() are intentionally
	// omitted: both deterministically trigger the SA_ONSTACK
	// signal-handler crash on a Writer doc loaded without
	// initializeForRendering. Add them here once the doc is
	// initialised for rendering. Unit tests cover them today.
}
