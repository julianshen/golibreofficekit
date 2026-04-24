//go:build (linux && (amd64 || arm64)) || (darwin && (amd64 || arm64))

package lokc

import (
	"bytes"
	"testing"
)

func TestCopyAndFreeBytes_CopiesAndFrees(t *testing.T) {
	payload := []byte{0xDE, 0xAD, 0xBE, 0xEF, 0x00}
	p := cmallocCopy(payload)
	got := copyAndFreeBytesTest(p, len(payload))
	if !bytes.Equal(got, payload) {
		t.Errorf("got %v, want %v", got, payload)
	}
}

func TestCopyAndFreeBytes_NilIsNil(t *testing.T) {
	if got := copyAndFreeBytesTest(nil, 0); got != nil {
		t.Errorf("nil input: got %v, want nil", got)
	}
}

func TestCopyAndFreeBytes_ZeroLengthFreesAndReturnsNil(t *testing.T) {
	p := cmallocRaw(1) // non-nil but irrelevant contents
	if got := copyAndFreeBytesTest(p, 0); got != nil {
		t.Errorf("0-length: got %v, want nil", got)
	}
	// p must be freed even though n=0 — no way to observe directly;
	// the helper's contract says copyAndFreeBytes always frees non-nil p.
}
