//go:build linux || darwin

package lokc

import (
	"errors"
	"testing"
)

func TestDocumentHandle_Nil(t *testing.T) {
	var d DocumentHandle
	if d.IsValid() {
		t.Error("zero-value DocumentHandle must be invalid")
	}
}

func TestDocumentWrappers_NilAreNoOps(t *testing.T) {
	var d DocumentHandle
	if got := DocumentGetType(d); got != -1 {
		t.Errorf("DocumentGetType on nil: got %d, want -1", got)
	}
	if err := DocumentSaveAs(d, "file:///tmp/x.odt", "", ""); !errors.Is(err, ErrNilDocument) {
		t.Errorf("DocumentSaveAs on nil: want ErrNilDocument, got %v", err)
	}
	DocumentDestroy(d) // must not panic
}

func TestDocumentWrappers_FakeHandle(t *testing.T) {
	d := NewFakeDocumentHandle()
	t.Cleanup(func() { FreeFakeDocumentHandle(d) })

	// pClass == NULL in the fake, C wrappers short-circuit. Exercises
	// the Go-side CString/free/cgo-call path on every wrapper.
	_ = DocumentGetType(d)
	_ = DocumentSaveAs(d, "file:///tmp/x.odt", "odt", "")
	DocumentDestroy(d)
}

func TestDocumentLoad_NilOfficeHandle(t *testing.T) {
	// Zero OfficeHandle must return an invalid DocumentHandle.
	d := DocumentLoad(OfficeHandle{}, "file:///tmp/x.odt")
	if d.IsValid() {
		t.Error("DocumentLoad on zero OfficeHandle: expected invalid DocumentHandle")
	}
}

func TestDocumentLoad_FakeOfficeHandle(t *testing.T) {
	// Fake handle has pClass==NULL; go_document_load returns NULL.
	h := NewFakeOfficeHandle()
	t.Cleanup(func() { FreeFakeOfficeHandle(h) })
	d := DocumentLoad(h, "file:///tmp/x.odt")
	// Result is always invalid (pClass==NULL → C returns NULL).
	if d.IsValid() {
		FreeFakeDocumentHandle(d) // clean up if somehow valid
		t.Error("DocumentLoad with pClass==NULL: expected invalid DocumentHandle")
	}
}

func TestDocumentLoadWithOptions_NilOfficeHandle(t *testing.T) {
	// Zero OfficeHandle must return an invalid DocumentHandle.
	d := DocumentLoadWithOptions(OfficeHandle{}, "file:///tmp/x.odt", "")
	if d.IsValid() {
		t.Error("DocumentLoadWithOptions on zero OfficeHandle: expected invalid DocumentHandle")
	}
}

func TestDocumentLoadWithOptions_FakeOfficeHandle(t *testing.T) {
	// Fake handle has pClass==NULL; go_document_load_with_options returns NULL.
	h := NewFakeOfficeHandle()
	t.Cleanup(func() { FreeFakeOfficeHandle(h) })
	// Test with empty options (copts stays nil).
	d := DocumentLoadWithOptions(h, "file:///tmp/x.odt", "")
	if d.IsValid() {
		FreeFakeDocumentHandle(d)
		t.Error("DocumentLoadWithOptions (no opts) with pClass==NULL: expected invalid DocumentHandle")
	}
	// Test with non-empty options (copts allocated and freed).
	d2 := DocumentLoadWithOptions(h, "file:///tmp/x.odt", "Hidden=true")
	if d2.IsValid() {
		FreeFakeDocumentHandle(d2)
		t.Error("DocumentLoadWithOptions (with opts) with pClass==NULL: expected invalid DocumentHandle")
	}
}
