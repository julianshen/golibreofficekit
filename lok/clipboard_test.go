//go:build linux || darwin

package lok

import (
	"bytes"
	"testing"
)

func TestClipboardItem_ShapeCompiles(t *testing.T) {
	// Compile-time assertion: ClipboardItem has MimeType and Data
	// fields with the documented types.
	it := ClipboardItem{MimeType: "text/plain", Data: []byte("hi")}
	if it.MimeType != "text/plain" || !bytes.Equal(it.Data, []byte("hi")) {
		t.Errorf("ClipboardItem round-trip failed: %+v", it)
	}
}
