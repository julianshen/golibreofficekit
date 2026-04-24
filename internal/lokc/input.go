//go:build linux || darwin

package lokc

/*
#cgo CFLAGS: -I${SRCDIR}/../../third_party/lok -DLOK_USE_UNSTABLE_API
#include <stdbool.h>
#include <stdlib.h>
#include "LibreOfficeKit/LibreOfficeKit.h"

static void go_doc_post_key_event(LibreOfficeKitDocument* d, int type, int charCode, int keyCode) {
    if (d == NULL || d->pClass == NULL || d->pClass->postKeyEvent == NULL) return;
    d->pClass->postKeyEvent(d, type, charCode, keyCode);
}
static void go_doc_post_mouse_event(LibreOfficeKitDocument* d, int type, int x, int y,
    int count, int buttons, int mods) {
    if (d == NULL || d->pClass == NULL || d->pClass->postMouseEvent == NULL) return;
    d->pClass->postMouseEvent(d, type, x, y, count, buttons, mods);
}
static void go_doc_post_uno_command(LibreOfficeKitDocument* d, const char* cmd,
    const char* args, bool notifyWhenFinished) {
    if (d == NULL || d->pClass == NULL || d->pClass->postUnoCommand == NULL) return;
    d->pClass->postUnoCommand(d, cmd, args, notifyWhenFinished);
}
*/
import "C"

import "unsafe"

// DocumentPostKeyEvent forwards to pClass->postKeyEvent. typ is
// a LOK_KEYEVENT_* value; charCode is a Unicode code point (0 for
// non-printables); keyCode is a com::sun::star::awt::Key value
// (0 for plain characters).
func DocumentPostKeyEvent(d DocumentHandle, typ, charCode, keyCode int) {
	if !d.IsValid() {
		return
	}
	C.go_doc_post_key_event(d.p, C.int(typ), C.int(charCode), C.int(keyCode))
}

// DocumentPostMouseEvent forwards to pClass->postMouseEvent. typ is
// LOK_MOUSEEVENT_*; x, y are twip coordinates (caller must ensure
// they fit in C int — 32-bit on LP64); count is click count; buttons
// and mods are OR-ed awt::MouseButton and awt::KeyModifier bitsets.
func DocumentPostMouseEvent(d DocumentHandle, typ, x, y, count, buttons, mods int) {
	if !d.IsValid() {
		return
	}
	C.go_doc_post_mouse_event(d.p, C.int(typ), C.int(x), C.int(y),
		C.int(count), C.int(buttons), C.int(mods))
}

// DocumentPostUnoCommand forwards to pClass->postUnoCommand. args
// may be empty; notifyWhenFinished requests a
// LOK_CALLBACK_UNO_COMMAND_RESULT on completion, observable only via
// a registered document callback.
func DocumentPostUnoCommand(d DocumentHandle, cmd, args string, notifyWhenFinished bool) {
	if !d.IsValid() {
		return
	}
	ccmd := C.CString(cmd)
	defer C.free(unsafe.Pointer(ccmd))
	cargs := C.CString(args)
	defer C.free(unsafe.Pointer(cargs))
	C.go_doc_post_uno_command(d.p, ccmd, cargs, C.bool(notifyWhenFinished))
}
