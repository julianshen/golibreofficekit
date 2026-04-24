//go:build (linux && (amd64 || arm64)) || (darwin && (amd64 || arm64))

package lokc

import "testing"

func TestDocumentRender_NilHandleAreNoOps(t *testing.T) {
	var d DocumentHandle

	DocumentInitializeForRendering(d, "")
	DocumentSetClientZoom(d, 1, 1, 1, 1)
	DocumentSetClientVisibleArea(d, 0, 0, 0, 0)
	DocumentPaintTile(d, make([]byte, 16), 2, 2, 0, 0, 100, 100)
	DocumentPaintPartTile(d, make([]byte, 16), 0, 0, 2, 2, 0, 0, 100, 100)

	if got := DocumentGetTileMode(d); got != 0 {
		t.Errorf("GetTileMode on nil: got %d, want 0", got)
	}
	if buf, w, h, ok := DocumentRenderSearchResult(d, "q"); buf != nil || w != 0 || h != 0 || ok {
		t.Errorf("RenderSearchResult on nil: got (%v, %d, %d, %v)", buf, w, h, ok)
	}
	if got := DocumentRenderShapeSelection(d); got != nil {
		t.Errorf("RenderShapeSelection on nil: got %v, want nil", got)
	}
}

func TestDocumentRender_FakeHandle_SafeNoOps(t *testing.T) {
	d := NewFakeDocumentHandle()
	t.Cleanup(func() { FreeFakeDocumentHandle(d) })

	DocumentInitializeForRendering(d, "{}")
	DocumentSetClientZoom(d, 256, 256, 1440, 1440)
	DocumentSetClientVisibleArea(d, 0, 0, 14400, 14400)
	DocumentPaintTile(d, make([]byte, 16), 2, 2, 0, 0, 100, 100)
	DocumentPaintPartTile(d, make([]byte, 16), 0, 0, 2, 2, 0, 0, 100, 100)

	if got := DocumentGetTileMode(d); got != 0 {
		t.Errorf("GetTileMode on fake: got %d, want 0", got)
	}
	if buf, w, h, ok := DocumentRenderSearchResult(d, "q"); buf != nil || w != 0 || h != 0 || ok {
		t.Errorf("RenderSearchResult on fake: got (%v, %d, %d, %v)", buf, w, h, ok)
	}
	if got := DocumentRenderShapeSelection(d); got != nil {
		t.Errorf("RenderShapeSelection on fake: got %v, want nil", got)
	}
}
