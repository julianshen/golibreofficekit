//go:build linux || darwin

package lok

import (
	"errors"
	"testing"
)

func TestSendDialogEvent(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)

	args := `{"type":"dialog","action":"execute"}`
	const wid uint64 = 1 << 33 // verify uint64 round-trips end-to-end
	if err := doc.SendDialogEvent(wid, args); err != nil {
		t.Fatal(err)
	}
	if fb.lastDialogWindowID != wid {
		t.Errorf("lastDialogWindowID=%d, want %d", fb.lastDialogWindowID, wid)
	}
	if fb.lastDialogArgs != args {
		t.Errorf("lastDialogArgs=%q, want %q", fb.lastDialogArgs, args)
	}
}

func TestSendDialogEvent_BackendError(t *testing.T) {
	fb := &fakeBackend{sendDialogEventErr: ErrUnsupported}
	_, doc := loadFakeDoc(t, fb)

	if err := doc.SendDialogEvent(42, "{}"); !errors.Is(err, ErrUnsupported) {
		t.Errorf("want ErrUnsupported, got %v", err)
	}
}

func TestSendDialogEvent_Closed(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	doc.Close()

	if err := doc.SendDialogEvent(42, "{}"); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestSendContentControlEvent(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)

	args := `{"control":"checkbox","action":"check"}`
	if err := doc.SendContentControlEvent(args); err != nil {
		t.Fatal(err)
	}
	if fb.lastContentControlArgs != args {
		t.Errorf("lastContentControlArgs=%q, want %q", fb.lastContentControlArgs, args)
	}
}

func TestSendContentControlEvent_BackendError(t *testing.T) {
	fb := &fakeBackend{sendContentControlEventErr: ErrUnsupported}
	_, doc := loadFakeDoc(t, fb)

	if err := doc.SendContentControlEvent("{}"); !errors.Is(err, ErrUnsupported) {
		t.Errorf("want ErrUnsupported, got %v", err)
	}
}

func TestSendContentControlEvent_Closed(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	doc.Close()

	if err := doc.SendContentControlEvent("{}"); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestSendFormFieldEvent(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)

	args := `{"field":"name","action":"changed"}`
	if err := doc.SendFormFieldEvent(args); err != nil {
		t.Fatal(err)
	}
	if fb.lastFormFieldArgs != args {
		t.Errorf("lastFormFieldArgs=%q, want %q", fb.lastFormFieldArgs, args)
	}
}

func TestSendFormFieldEvent_BackendError(t *testing.T) {
	fb := &fakeBackend{sendFormFieldEventErr: ErrUnsupported}
	_, doc := loadFakeDoc(t, fb)

	if err := doc.SendFormFieldEvent("{}"); !errors.Is(err, ErrUnsupported) {
		t.Errorf("want ErrUnsupported, got %v", err)
	}
}

func TestSendFormFieldEvent_Closed(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	doc.Close()

	if err := doc.SendFormFieldEvent("{}"); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}
