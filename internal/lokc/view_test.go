//go:build linux || darwin

package lokc

import "testing"

func TestDocumentView_NilHandleAreNoOps(t *testing.T) {
	var d DocumentHandle
	if got := DocumentCreateView(d); got != -1 {
		t.Errorf("CreateView on nil: got %d, want -1", got)
	}
	if got := DocumentCreateViewWithOptions(d, "foo=1"); got != -1 {
		t.Errorf("CreateViewWithOptions on nil: got %d, want -1", got)
	}
	if got := DocumentGetView(d); got != -1 {
		t.Errorf("GetView on nil: got %d, want -1", got)
	}
	if got := DocumentGetViewsCount(d); got != -1 {
		t.Errorf("GetViewsCount on nil: got %d, want -1", got)
	}
	if ids, ok := DocumentGetViewIds(d); ids != nil || ok {
		t.Errorf("GetViewIds on nil: got (%v, %v), want (nil, false)", ids, ok)
	}
	DocumentDestroyView(d, 0)
	DocumentSetView(d, 0)
	DocumentSetViewLanguage(d, 0, "en-US")
	DocumentSetViewReadOnly(d, 0, true)
	DocumentSetAccessibilityState(d, 0, true)
	DocumentSetViewTimezone(d, 0, "UTC")
}

func TestDocumentView_FakeHandle_SafeNoOps(t *testing.T) {
	d := NewFakeDocumentHandle()
	t.Cleanup(func() { FreeFakeDocumentHandle(d) })

	// pClass is NULL — C guards short-circuit every call. We're only
	// verifying the Go-side CString/free/cgo-call path doesn't crash.
	DocumentCreateView(d)
	DocumentCreateViewWithOptions(d, "a=1")
	DocumentDestroyView(d, 0)
	DocumentSetView(d, 0)
	DocumentGetView(d)
	DocumentGetViewsCount(d)
	DocumentGetViewIds(d)
	DocumentSetViewLanguage(d, 0, "en-US")
	DocumentSetViewReadOnly(d, 0, true)
	DocumentSetAccessibilityState(d, 0, true)
	DocumentSetViewTimezone(d, 0, "UTC")
}
