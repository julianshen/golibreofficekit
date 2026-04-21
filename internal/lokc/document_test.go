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
