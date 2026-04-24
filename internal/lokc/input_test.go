//go:build (linux && (amd64 || arm64)) || (darwin && (amd64 || arm64))

package lokc

import "testing"

func TestDocumentInput_NilHandleAreNoOps(t *testing.T) {
	var d DocumentHandle
	DocumentPostKeyEvent(d, 0, 'a', 0)
	DocumentPostMouseEvent(d, 0, 100, 100, 1, 1, 0)
	DocumentPostUnoCommand(d, ".uno:Bold", "", false)
}

func TestDocumentInput_FakeHandle_SafeNoOps(t *testing.T) {
	d := NewFakeDocumentHandle()
	t.Cleanup(func() { FreeFakeDocumentHandle(d) })
	DocumentPostKeyEvent(d, 0, 'a', 0)
	DocumentPostMouseEvent(d, 0, 100, 100, 1, 1, 0)
	DocumentPostUnoCommand(d, ".uno:Bold", "", false)
}
