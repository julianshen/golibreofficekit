//go:build linux || darwin

package lokc

import (
	"errors"
	"testing"
)

// ErrClipboardFailed must exist as a package-level sentinel so callers
// outside lokc can errors.Is() against it. Before promotion, the
// failure errors were inline errors.New() values that could only be
// matched on string content.
func TestErrClipboardFailed_IsExported(t *testing.T) {
	if ErrClipboardFailed == nil {
		t.Fatal("ErrClipboardFailed is nil; want a package-level sentinel")
	}
	wrapped := errors.Join(ErrClipboardFailed, errors.New("context"))
	if !errors.Is(wrapped, ErrClipboardFailed) {
		t.Errorf("errors.Is failed to traverse to ErrClipboardFailed")
	}
}

func TestDocumentGetClipboard_NilSafe(t *testing.T) {
	items, err := DocumentGetClipboard(DocumentHandle{}, nil)
	if err != ErrUnsupported {
		t.Errorf("zero handle: err=%v, want ErrUnsupported", err)
	}
	if items != nil {
		t.Errorf("zero handle: items=%v, want nil", items)
	}

	items, err = DocumentGetClipboard(newFakeDoc(t), []string{"text/plain"})
	if err != ErrUnsupported {
		t.Errorf("nil pClass: err=%v, want ErrUnsupported", err)
	}
	if items != nil {
		t.Errorf("nil pClass: items=%v, want nil", items)
	}
}

func TestDocumentSetClipboard_NilSafe(t *testing.T) {
	if err := DocumentSetClipboard(DocumentHandle{}, nil); err != ErrUnsupported {
		t.Errorf("zero handle: err=%v, want ErrUnsupported", err)
	}
	items := []ClipboardItem{{MimeType: "text/plain", Data: []byte("hi")}}
	if err := DocumentSetClipboard(newFakeDoc(t), items); err != ErrUnsupported {
		t.Errorf("nil pClass: err=%v, want ErrUnsupported", err)
	}
}
