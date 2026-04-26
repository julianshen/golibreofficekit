//go:build linux || darwin

package lok

import (
	"errors"
	"testing"
)

func TestFilterTypes(t *testing.T) {
	fb := &fakeBackend{filterTypesResult: `{"writer":"writer8"}`}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()

	got, err := o.FilterTypes()
	if err != nil {
		t.Fatal(err)
	}
	if got != `{"writer":"writer8"}` {
		t.Errorf("got %q", got)
	}
}

func TestFilterTypes_Closed(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	o.Close()
	if _, err := o.FilterTypes(); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestFilterTypes_BackendError(t *testing.T) {
	fb := &fakeBackend{filterTypesErr: ErrUnsupported}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	if _, err := o.FilterTypes(); !errors.Is(err, ErrUnsupported) {
		t.Errorf("want ErrUnsupported, got %v", err)
	}
}
