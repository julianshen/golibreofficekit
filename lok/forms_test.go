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
	if err := doc.SendDialogEvent(42, args); err != nil {
		t.Fatal(err)
	}
	if fb.lastWindowID != 42 {
		t.Errorf("lastWindowID=%d", fb.lastWindowID)
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
