package lok

import (
	"errors"
	"testing"
)

func TestErrUnsupported_Sentinel(t *testing.T) {
	// A fresh wrap of ErrUnsupported must still compare equal with
	// errors.Is, and ErrUnsupported must be distinct from the other
	// known sentinels.
	wrapped := errors.Join(ErrUnsupported, errors.New("ctx"))
	if !errors.Is(wrapped, ErrUnsupported) {
		t.Errorf("errors.Is(wrapped, ErrUnsupported) = false, want true")
	}
	if errors.Is(ErrUnsupported, ErrClosed) {
		t.Errorf("ErrUnsupported must not alias ErrClosed")
	}
	if ErrUnsupported.Error() == "" {
		t.Error("ErrUnsupported.Error() must not be empty")
	}
}
