//go:build linux || darwin

package lok

import (
	"errors"
	"testing"

	"github.com/julianshen/golibreofficekit/internal/lokc"
)

// TestRealBackend_ForwardsThroughFakeLOK exercises every realBackend
// method against a lokc fake whose pClass is NULL, so the underlying
// C guards turn each call into a safe no-op. The purpose is purely to
// cover the Go-side forwarding statements in real_backend.go —
// integration tests cover the happy path against real LibreOffice.
func TestRealBackend_ForwardsThroughFakeLOK(t *testing.T) {
	rb := realBackend{}

	lib, err := rb.OpenLibrary("/nonexistent/install/path/should/not/exist")
	if err == nil {
		t.Fatal("expected OpenLibrary to fail on missing path")
	}
	if lib != nil {
		t.Error("lib non-nil despite error")
	}

	// Build a fake library handle and run InvokeHook through realBackend.
	fakeLib := realLibraryHandle{lib: lokc.NewFakeLibrary(2)}
	h, err := rb.InvokeHook(fakeLib, "file:///tmp/profile")
	if err != nil {
		t.Fatalf("InvokeHook: %v", err)
	}
	defer func() {
		// Release the fake LibreOfficeKit allocated by NewFakeLibrary's hook.
		rh := h.(realOfficeHandle)
		lokc.FreeFakeOfficeHandle(rh.h)
	}()

	// Every wrapper must be callable without panicking and return the
	// zero-state reply (empty string or void).
	if got := rb.OfficeGetError(h); got != "" {
		t.Errorf("OfficeGetError: %q, want empty", got)
	}
	if got := rb.OfficeGetVersionInfo(h); got != "" {
		t.Errorf("OfficeGetVersionInfo: %q, want empty", got)
	}
	if got := rb.OfficeDumpState(h); got != "" {
		t.Errorf("OfficeDumpState: %q, want empty", got)
	}
	rb.OfficeSetOptionalFeatures(h, 0x3)
	rb.OfficeSetDocumentPassword(h, "file:///x", "pw")
	rb.OfficeSetAuthor(h, "Author")
	rb.OfficeTrimMemory(h, 1)
}

func TestRealBackend_InvokeHookPanicsOnFakeLib(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on mismatched library handle")
		}
	}()
	realBackend{}.InvokeHook(fakeLib{}, "")
}

func TestRealBackend_MustPanicsOnFakeOffice(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on mismatched office handle")
		}
	}()
	realBackend{}.OfficeGetError(&fakeOffice{})
}

func TestRealBackend_BrandMethods(t *testing.T) {
	// Branded interface methods are nominal; calling them exercises the
	// statement for coverage and confirms they don't panic.
	realLibraryHandle{}.libraryBrand()
	realOfficeHandle{}.officeBrand()
}

// TestLOKError_Error covers both branches of LOKError.Error (with and
// without Op).
func TestLOKError_Error(t *testing.T) {
	cases := []struct {
		err  *LOKError
		want string
	}{
		{&LOKError{Detail: "bare"}, "lok: bare"},
		{&LOKError{Op: "Save", Detail: "permission denied"}, "lok: Save: permission denied"},
	}
	for _, tc := range cases {
		if got := tc.err.Error(); got != tc.want {
			t.Errorf("got %q, want %q", got, tc.want)
		}
	}
}

func TestRealBackend_DocumentForwarding(t *testing.T) {
	rb := realBackend{}

	// Fake DocumentHandle from lokc directly.
	fakeDocHandle := lokc.NewFakeDocumentHandle()
	defer lokc.FreeFakeDocumentHandle(fakeDocHandle)
	rdoc := realDocumentHandle{d: fakeDocHandle}

	if got := rb.DocumentGetType(rdoc); got != -1 {
		t.Errorf("DocumentGetType on fake pClass=NULL: got %d, want -1", got)
	}
	if err := rb.DocumentSaveAs(rdoc, "file:///tmp/x.odt", "odt", ""); err == nil {
		t.Error("DocumentSaveAs on fake with NULL vtable must error")
	}
	rb.DocumentDestroy(rdoc) // must not panic
}

func TestRealBackend_DocumentLoadForwarding(t *testing.T) {
	rb := realBackend{}

	// Build a fake office handle via NewFakeLibrary (pClass is NULL
	// so documentLoad returns NULL — exercises the error path in
	// realBackend.DocumentLoad and calls into lokc.DocumentLoad).
	fakeLib := realLibraryHandle{lib: lokc.NewFakeLibrary(2)}
	h, err := rb.InvokeHook(fakeLib, "file:///tmp/profile")
	if err != nil {
		t.Fatalf("InvokeHook: %v", err)
	}
	defer func() {
		rh := h.(realOfficeHandle)
		lokc.FreeFakeOfficeHandle(rh.h)
	}()

	// DocumentLoad: pClass.documentLoad is NULL → returns invalid handle → error.
	doc, err := rb.DocumentLoad(h, "file:///tmp/test.odt")
	if err == nil {
		t.Error("DocumentLoad with NULL vtable must error")
	}
	if doc != nil {
		t.Error("DocumentLoad must return nil on error")
	}

	// DocumentLoadWithOptions: same expectation.
	doc2, err := rb.DocumentLoadWithOptions(h, "file:///tmp/test.odt", "Hidden=1")
	if err == nil {
		t.Error("DocumentLoadWithOptions with NULL vtable must error")
	}
	if doc2 != nil {
		t.Error("DocumentLoadWithOptions must return nil on error")
	}

	// OfficeDestroy: also covered here to avoid a separate fake setup.
	rb.OfficeDestroy(h)
	// FreeFakeOfficeHandle deferred above must not double-free — it's safe
	// to call on a destroyed handle (lokc guards are NULL-checked).
}

func TestRealBackend_DocumentHandleMismatchPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on mismatched document handle")
		}
	}()
	realBackend{}.DocumentGetType(&fakeDoc{})
}

func TestRealBackend_DocumentBrand(t *testing.T) {
	realDocumentHandle{}.documentBrand()
}

// TestWithUserProfile_SetsField covers the WithUserProfile option.
func TestWithUserProfile_SetsField(t *testing.T) {
	opts := buildOptions([]Option{WithUserProfile("file:///home/x/.libreoffice")})
	if opts.userProfileURL != "file:///home/x/.libreoffice" {
		t.Errorf("userProfileURL=%q, want file:///home/x/.libreoffice", opts.userProfileURL)
	}
}

// Sanity: errors.Is still discriminates.
func TestErrorSentinelsAreDistinct(t *testing.T) {
	if errors.Is(ErrClosed, ErrAlreadyInitialised) {
		t.Error("ErrClosed should not match ErrAlreadyInitialised")
	}
}
