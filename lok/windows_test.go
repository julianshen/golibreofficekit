//go:build linux || darwin

package lok

import (
	"errors"
	"math"
	"testing"
)

func TestPostWindowKeyEvent(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)

	if err := doc.PostWindowKeyEvent(123, KeyEventInput, 'A', 65); err != nil {
		t.Fatal(err)
	}
	if fb.lastWindowID != 123 {
		t.Errorf("lastWindowID=%d", fb.lastWindowID)
	}
}

func TestPostWindowKeyEvent_Closed(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	doc.Close()

	if err := doc.PostWindowKeyEvent(1, KeyEventInput, 'A', 65); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestPostWindowMouseEvent(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)

	if err := doc.PostWindowMouseEvent(456, MouseButtonDown, 100, 200, 1, MouseLeft, ModShift); err != nil {
		t.Fatal(err)
	}
	if fb.lastWindowID != 456 {
		t.Errorf("lastWindowID=%d", fb.lastWindowID)
	}
}

func TestPostWindowMouseEvent_Closed(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	doc.Close()

	if err := doc.PostWindowMouseEvent(1, MouseButtonDown, 10, 20, 1, MouseLeft, 0); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestPostWindowGestureEvent(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)

	if err := doc.PostWindowGestureEvent(789, "panBegin", 10, 20, 5); err != nil {
		t.Fatal(err)
	}
	if fb.lastWindowID != 789 {
		t.Errorf("lastWindowID=%d", fb.lastWindowID)
	}
	if fb.lastGestureType != "panBegin" {
		t.Errorf("lastGestureType=%q, want panBegin", fb.lastGestureType)
	}
}

func TestPostWindowGestureEvent_Int32Overflow(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)

	cases := []struct {
		name         string
		x, y, offset int64
	}{
		{"x overflow", math.MaxInt32 + 1, 0, 0},
		{"y overflow", 0, math.MaxInt32 + 1, 0},
		{"offset overflow", 0, 0, math.MaxInt32 + 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := doc.PostWindowGestureEvent(1, "pan", tc.x, tc.y, tc.offset)
			var lokErr *LOKError
			if !errors.As(err, &lokErr) || lokErr.Op != "PostWindowGestureEvent" {
				t.Errorf("want *LOKError{Op:PostWindowGestureEvent}, got %v", err)
			}
		})
	}
}

func TestPostWindowGestureEvent_Closed(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	doc.Close()

	if err := doc.PostWindowGestureEvent(1, "pan", 10, 20, 5); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestPostWindowExtTextInputEvent(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)

	if err := doc.PostWindowExtTextInputEvent(321, 1, "hello"); err != nil {
		t.Fatal(err)
	}
	if fb.lastWindowID != 321 {
		t.Errorf("lastWindowID=%d", fb.lastWindowID)
	}
	if fb.lastExtTextInputType != 1 {
		t.Errorf("lastExtTextInputType=%d, want 1", fb.lastExtTextInputType)
	}
	if fb.lastExtTextInputText != "hello" {
		t.Errorf("lastExtTextInputText=%q, want hello", fb.lastExtTextInputText)
	}
}

func TestPostWindowExtTextInputEvent_Closed(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	doc.Close()

	if err := doc.PostWindowExtTextInputEvent(1, 1, "x"); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestResizeWindow(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)

	if err := doc.ResizeWindow(100, 200, 300); err != nil {
		t.Fatal(err)
	}
	if fb.lastWindowID != 100 {
		t.Errorf("lastWindowID=%d", fb.lastWindowID)
	}
}

func TestResizeWindow_InvalidSize(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)

	cases := []struct {
		name    string
		w, h    int
		wantErr error
	}{
		{"negative w", -10, 100, ErrInvalidOption},
		{"negative h", 100, -10, ErrInvalidOption},
		{"zero w", 0, 100, ErrInvalidOption},
		{"zero h", 100, 0, ErrInvalidOption},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := doc.ResizeWindow(1, tc.w, tc.h)
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("want %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestResizeWindow_Closed(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	doc.Close()

	if err := doc.ResizeWindow(1, 200, 200); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestPaintWindow(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)

	buf := make([]byte, 4*100*100)
	if err := doc.PaintWindow(1, buf, 0, 0, 100, 100); err != nil {
		t.Fatal(err)
	}
	if fb.lastWindowID != 1 {
		t.Errorf("lastWindowID=%d", fb.lastWindowID)
	}
}

func TestPaintWindow_Closed(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	doc.Close()

	buf := make([]byte, 4*100*100)
	if err := doc.PaintWindow(1, buf, 0, 0, 100, 100); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestPaintWindowDPI(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)

	buf := make([]byte, 4*50*50)
	if err := doc.PaintWindowDPI(2, buf, 0, 0, 50, 50, 1.5); err != nil {
		t.Fatal(err)
	}
	if fb.lastWindowID != 2 {
		t.Errorf("lastWindowID=%d", fb.lastWindowID)
	}
}

func TestPaintWindowDPI_Closed(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	doc.Close()

	buf := make([]byte, 4*50*50)
	if err := doc.PaintWindowDPI(1, buf, 0, 0, 50, 50, 1.0); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestPaintWindowForView(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)

	buf := make([]byte, 4*64*64)
	if err := doc.PaintWindowForView(3, ViewID(10), buf, 0, 0, 64, 64, 2.0); err != nil {
		t.Fatal(err)
	}
	if fb.lastWindowID != 3 {
		t.Errorf("lastWindowID=%d", fb.lastWindowID)
	}
}

func TestPaintWindowForView_Closed(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	doc.Close()

	buf := make([]byte, 4*64*64)
	if err := doc.PaintWindowForView(1, ViewID(0), buf, 0, 0, 64, 64, 1.0); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}
