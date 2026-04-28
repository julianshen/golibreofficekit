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
	// PR B: setters now report ErrUnsupported instead of silently
	// no-opping when the vtable slot is NULL (or the handle is zero).
	if err := DocumentDestroyView(d, 0); err != ErrUnsupported {
		t.Errorf("DestroyView on nil: err=%v, want ErrUnsupported", err)
	}
	if err := DocumentSetView(d, 0); err != ErrUnsupported {
		t.Errorf("SetView on nil: err=%v, want ErrUnsupported", err)
	}
	if err := DocumentSetViewLanguage(d, 0, "en-US"); err != ErrUnsupported {
		t.Errorf("SetViewLanguage on nil: err=%v, want ErrUnsupported", err)
	}
	if err := DocumentSetViewReadOnly(d, 0, true); err != ErrUnsupported {
		t.Errorf("SetViewReadOnly on nil: err=%v, want ErrUnsupported", err)
	}
	if err := DocumentSetAccessibilityState(d, 0, true); err != ErrUnsupported {
		t.Errorf("SetAccessibilityState on nil: err=%v, want ErrUnsupported", err)
	}
	if err := DocumentSetViewTimezone(d, 0, "UTC"); err != ErrUnsupported {
		t.Errorf("SetViewTimezone on nil: err=%v, want ErrUnsupported", err)
	}
}

func TestDocumentView_FakeHandle_SafeNoOps(t *testing.T) {
	d := NewFakeDocumentHandle()
	t.Cleanup(func() { FreeFakeDocumentHandle(d) })

	// pClass is NULL — C guards short-circuit every call. We're only
	// verifying the Go-side CString/free/cgo-call path doesn't crash
	// for the getters; the setters now surface ErrUnsupported.
	DocumentCreateView(d)
	DocumentCreateViewWithOptions(d, "a=1")
	DocumentGetView(d)
	DocumentGetViewsCount(d)
	DocumentGetViewIds(d)
	if err := DocumentDestroyView(d, 0); err != ErrUnsupported {
		t.Errorf("DestroyView on fake: err=%v, want ErrUnsupported", err)
	}
	if err := DocumentSetView(d, 0); err != ErrUnsupported {
		t.Errorf("SetView on fake: err=%v, want ErrUnsupported", err)
	}
	if err := DocumentSetViewLanguage(d, 0, "en-US"); err != ErrUnsupported {
		t.Errorf("SetViewLanguage on fake: err=%v, want ErrUnsupported", err)
	}
	if err := DocumentSetViewReadOnly(d, 0, true); err != ErrUnsupported {
		t.Errorf("SetViewReadOnly on fake: err=%v, want ErrUnsupported", err)
	}
	if err := DocumentSetAccessibilityState(d, 0, true); err != ErrUnsupported {
		t.Errorf("SetAccessibilityState on fake: err=%v, want ErrUnsupported", err)
	}
	if err := DocumentSetViewTimezone(d, 0, "UTC"); err != ErrUnsupported {
		t.Errorf("SetViewTimezone on fake: err=%v, want ErrUnsupported", err)
	}
}
