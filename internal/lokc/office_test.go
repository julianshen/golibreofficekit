//go:build linux || darwin

package lokc

import (
	"errors"
	"testing"
)

func TestInvokeHook_RejectsNilLibrary(t *testing.T) {
	_, err := InvokeHook(nil, "")
	if !errors.Is(err, ErrNilLibrary) {
		t.Fatalf("want ErrNilLibrary, got %v", err)
	}
}

func TestOfficeHandle_Nil(t *testing.T) {
	var h OfficeHandle
	if h.IsValid() {
		t.Error("zero-value OfficeHandle must be invalid")
	}
}

func TestInvokeHook_Hook2Path(t *testing.T) {
	lib := newFakeLibrary(2)
	h, err := InvokeHook(lib, "file:///tmp/profile")
	if err != nil {
		t.Fatalf("InvokeHook: %v", err)
	}
	if !h.IsValid() {
		t.Fatal("OfficeHandle invalid after successful fake hook")
	}
	freeFakeOfficeHandle(h)
}

func TestInvokeHook_Hook1Path(t *testing.T) {
	lib := newFakeLibrary(1)
	h, err := InvokeHook(lib, "")
	if err != nil {
		t.Fatalf("InvokeHook: %v", err)
	}
	if !h.IsValid() {
		t.Fatal("OfficeHandle invalid after successful fake hook")
	}
	freeFakeOfficeHandle(h)
}

func TestInvokeHook_NullReturnIsError(t *testing.T) {
	lib := newFakeLibraryNullReturn()
	_, err := InvokeHook(lib, "")
	if err == nil {
		t.Fatal("expected error when hook returns NULL")
	}
	var lokErr *LOKError
	if !errors.As(err, &lokErr) {
		t.Fatalf("want *LOKError, got %T %v", err, err)
	}
}

func TestLOKError_Error(t *testing.T) {
	err := &LOKError{Detail: "sample"}
	if got := err.Error(); got != "lokc: sample" {
		t.Errorf("Error()=%q, want %q", got, "lokc: sample")
	}
}

// TestOfficeWrappers_FakeHandle exercises every Office* wrapper on a
// fake LibreOfficeKit (pClass == NULL). The C-side guards make each
// call a safe no-op while every Go statement (nil-check, CString,
// free, cgo call, copyAndFree) runs.
func TestOfficeWrappers_FakeHandle(t *testing.T) {
	h := newFakeOfficeHandle()
	t.Cleanup(func() { freeFakeOfficeHandle(h) })

	if got := OfficeGetError(h); got != "" {
		t.Errorf("OfficeGetError on fake: %q, want empty", got)
	}
	if got := OfficeGetVersionInfo(h); got != "" {
		t.Errorf("OfficeGetVersionInfo on fake: %q, want empty", got)
	}
	if got := OfficeDumpState(h); got != "" {
		t.Errorf("OfficeDumpState on fake: %q, want empty", got)
	}

	// The setters/void wrappers have no return; we only verify they
	// don't crash on a fake handle.
	OfficeSetOptionalFeatures(h, 0x1)
	OfficeSetAuthor(h, "Jane")
	OfficeSetDocumentPassword(h, "file:///x", "pw")
	OfficeSetDocumentPassword(h, "file:///x", "") // empty-password branch
	OfficeTrimMemory(h, 100)
}

func TestOfficeWrappers_NilHandleAreNoOps(t *testing.T) {
	var h OfficeHandle // zero value, invalid
	if got := OfficeGetError(h); got != "" {
		t.Errorf("OfficeGetError: %q, want empty on nil", got)
	}
	if got := OfficeGetVersionInfo(h); got != "" {
		t.Errorf("OfficeGetVersionInfo: %q, want empty on nil", got)
	}
	if got := OfficeDumpState(h); got != "" {
		t.Errorf("OfficeDumpState: %q, want empty on nil", got)
	}
	OfficeSetOptionalFeatures(h, 0)
	OfficeSetAuthor(h, "x")
	OfficeSetDocumentPassword(h, "", "")
	OfficeTrimMemory(h, 0)
	OfficeDestroy(h) // must not panic
}
