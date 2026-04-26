//go:build linux || darwin

package lokc

/*
#cgo CFLAGS: -I${SRCDIR}/../../third_party/lok -DLOK_USE_UNSTABLE_API
#include <stdlib.h>
#include "LibreOfficeKit/LibreOfficeKit.h"
#include "windows.h"
*/
import "C"
import (
	"errors"
	"unsafe"
)

// DocumentPostWindowKeyEvent posts a key event to a window.
func DocumentPostWindowKeyEvent(d DocumentHandle, windowID uint32, typ, charCode, keyCode int) error {
	if !d.IsValid() {
		return ErrNilDocument
	}
	ok := C.loke_post_window_key_event(unsafe.Pointer(d.p), C.uint32_t(windowID), C.int(typ), C.int(charCode), C.int(keyCode))
	if ok == 0 {
		return ErrUnsupported
	}
	return nil
}

// DocumentPostWindowMouseEvent posts a mouse event to a window.
func DocumentPostWindowMouseEvent(d DocumentHandle, windowID uint32, typ int, x, y int64, count int, buttons, mods int) error {
	if !d.IsValid() {
		return ErrNilDocument
	}
	ok := C.loke_post_window_mouse_event(unsafe.Pointer(d.p), C.uint32_t(windowID), C.int(typ), C.int64_t(x), C.int64_t(y), C.int(count), C.int(buttons), C.int(mods))
	if ok == 0 {
		return ErrUnsupported
	}
	return nil
}

// DocumentPostWindowGestureEvent posts a gesture event to a window.
func DocumentPostWindowGestureEvent(d DocumentHandle, windowID uint32, typ string, x, y, offset int64) error {
	if !d.IsValid() {
		return ErrNilDocument
	}
	cTyp := C.CString(typ)
	defer C.free(unsafe.Pointer(cTyp))
	ok := C.loke_post_window_gesture_event(unsafe.Pointer(d.p), C.uint32_t(windowID), cTyp, C.int64_t(x), C.int64_t(y), C.int64_t(offset))
	if ok == 0 {
		return ErrUnsupported
	}
	return nil
}

// DocumentPostWindowExtTextInputEvent posts extended text input to a window.
func DocumentPostWindowExtTextInputEvent(d DocumentHandle, windowID uint32, typ int, text string) error {
	if !d.IsValid() {
		return ErrNilDocument
	}
	cText := C.CString(text)
	defer C.free(unsafe.Pointer(cText))
	ok := C.loke_post_window_ext_text_input_event(unsafe.Pointer(d.p), C.uint32_t(windowID), C.int(typ), cText)
	if ok == 0 {
		return ErrUnsupported
	}
	return nil
}

// DocumentResizeWindow resizes a window.
func DocumentResizeWindow(d DocumentHandle, windowID uint32, w, h int) error {
	if !d.IsValid() {
		return ErrNilDocument
	}
	ok := C.loke_resize_window(unsafe.Pointer(d.p), C.uint32_t(windowID), C.int(w), C.int(h))
	if ok == 0 {
		return ErrUnsupported
	}
	return nil
}

// DocumentPaintWindow paints a window.
func DocumentPaintWindow(d DocumentHandle, windowID uint32, buf []byte, x, y, pxW, pxH int) error {
	if !d.IsValid() {
		return ErrNilDocument
	}
	if len(buf) != 4*pxW*pxH {
		return errors.New("lokc: buffer size mismatch")
	}
	ok := C.loke_paint_window(unsafe.Pointer(d.p), C.uint32_t(windowID), unsafe.Pointer(&buf[0]), C.int(x), C.int(y), C.int(pxW), C.int(pxH))
	if ok == 0 {
		return ErrUnsupported
	}
	return nil
}

// DocumentPaintWindowDPI paints a window with DPI scaling.
func DocumentPaintWindowDPI(d DocumentHandle, windowID uint32, buf []byte, x, y, pxW, pxH int, dpiScale float64) error {
	if !d.IsValid() {
		return ErrNilDocument
	}
	if len(buf) != 4*pxW*pxH {
		return errors.New("lokc: buffer size mismatch")
	}
	ok := C.loke_paint_window_dpi(unsafe.Pointer(d.p), C.uint32_t(windowID), unsafe.Pointer(&buf[0]), C.int(x), C.int(y), C.int(pxW), C.int(pxH), C.double(dpiScale))
	if ok == 0 {
		return ErrUnsupported
	}
	return nil
}

// DocumentPaintWindowForView paints a window for a specific view.
func DocumentPaintWindowForView(d DocumentHandle, windowID uint32, view int, buf []byte, x, y, pxW, pxH int, dpiScale float64) error {
	if !d.IsValid() {
		return ErrNilDocument
	}
	if len(buf) != 4*pxW*pxH {
		return errors.New("lokc: buffer size mismatch")
	}
	ok := C.loke_paint_window_for_view(unsafe.Pointer(d.p), C.uint32_t(windowID), C.int(view), unsafe.Pointer(&buf[0]), C.int(x), C.int(y), C.int(pxW), C.int(pxH), C.double(dpiScale))
	if ok == 0 {
		return ErrUnsupported
	}
	return nil
}
