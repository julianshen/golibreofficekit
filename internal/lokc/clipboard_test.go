//go:build linux || darwin

package lokc

import "testing"

func TestDocumentGetClipboard_NilSafe(t *testing.T) {
	items, err := DocumentGetClipboard(DocumentHandle{}, nil)
	if err == nil || err != ErrUnsupported {
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
