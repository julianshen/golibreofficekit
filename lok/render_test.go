//go:build linux || darwin

package lok

import (
	"errors"
	"testing"
)

func TestInitializeForRendering_HappyPath(t *testing.T) {
	fb := &fakeBackend{tileMode: 1} // BGRA
	_, doc := loadFakeDoc(t, fb)
	if err := doc.InitializeForRendering(""); err != nil {
		t.Fatalf("InitializeForRendering: %v", err)
	}
	if fb.lastInitArgs != "" {
		t.Errorf("lastInitArgs=%q, want empty", fb.lastInitArgs)
	}
}

func TestInitializeForRendering_ForwardsArgs(t *testing.T) {
	fb := &fakeBackend{tileMode: 1}
	_, doc := loadFakeDoc(t, fb)
	args := `{".uno:HideWhitespace":{"type":"boolean","value":"true"}}`
	if err := doc.InitializeForRendering(args); err != nil {
		t.Fatal(err)
	}
	if fb.lastInitArgs != args {
		t.Errorf("lastInitArgs=%q, want %q", fb.lastInitArgs, args)
	}
}

func TestInitializeForRendering_UnsupportedTileMode(t *testing.T) {
	fb := &fakeBackend{tileMode: 0} // RGBA — unsupported
	_, doc := loadFakeDoc(t, fb)
	err := doc.InitializeForRendering("")
	var lokErr *LOKError
	if !errors.As(err, &lokErr) || lokErr.Op != "InitializeForRendering" {
		t.Errorf("want *LOKError{Op: InitializeForRendering}, got %T %v", err, err)
	}
}

func TestInitializeForRendering_UnexpectedTileMode(t *testing.T) {
	// Any non-1 mode (including future enum values) is treated as an error.
	fb := &fakeBackend{tileMode: 2}
	_, doc := loadFakeDoc(t, fb)
	err := doc.InitializeForRendering("")
	var lokErr *LOKError
	if !errors.As(err, &lokErr) {
		t.Errorf("want *LOKError, got %T %v", err, err)
	}
}

func TestInitializeForRendering_AfterCloseErrors(t *testing.T) {
	_, doc := loadFakeDoc(t, &fakeBackend{tileMode: 1})
	doc.Close()
	if err := doc.InitializeForRendering(""); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestSetClientZoom_Passes(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	if err := doc.SetClientZoom(256, 256, 1440, 1440); err != nil {
		t.Fatal(err)
	}
	if fb.lastZoom != [4]int{256, 256, 1440, 1440} {
		t.Errorf("lastZoom=%v", fb.lastZoom)
	}
}

func TestSetClientZoom_WithoutInitializeOK(t *testing.T) {
	// Zoom is an optional hint; does NOT require InitializeForRendering.
	_, doc := loadFakeDoc(t, &fakeBackend{})
	if err := doc.SetClientZoom(1, 1, 1, 1); err != nil {
		t.Errorf("want no error, got %v", err)
	}
}

func TestSetClientVisibleArea_PassesAsInt(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	if err := doc.SetClientVisibleArea(TwipRect{X: 0, Y: 0, W: 14400, H: 14400}); err != nil {
		t.Fatal(err)
	}
	if fb.lastVisibleArea != [4]int{0, 0, 14400, 14400} {
		t.Errorf("lastVisibleArea=%v", fb.lastVisibleArea)
	}
}

func TestSetClientVisibleArea_RejectsOverflow(t *testing.T) {
	_, doc := loadFakeDoc(t, &fakeBackend{})
	err := doc.SetClientVisibleArea(TwipRect{X: 0, Y: 0, W: 1<<32 + 1, H: 1})
	var lokErr *LOKError
	if !errors.As(err, &lokErr) {
		t.Fatalf("want *LOKError, got %T %v", err, err)
	}
	if lokErr.Op != "SetClientVisibleArea" {
		t.Errorf("Op=%q, want SetClientVisibleArea", lokErr.Op)
	}
}
