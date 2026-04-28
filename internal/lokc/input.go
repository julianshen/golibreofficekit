//go:build linux || darwin

package lokc

/*
#cgo CFLAGS: -I${SRCDIR}/../../third_party/lok -DLOK_USE_UNSTABLE_API
#include <stdbool.h>
#include <stdlib.h>
#include "LibreOfficeKit/LibreOfficeKit.h"

// Shims return 1 on success, 0 when the vtable slot is NULL — caller
// maps to ErrUnsupported. Mirrors the selection.go pattern; silent
// no-ops are the audit-flagged defect we're fixing.
static int go_doc_post_key_event(LibreOfficeKitDocument* d, int type, int charCode, int keyCode) {
    if (d == NULL || d->pClass == NULL || d->pClass->postKeyEvent == NULL) return 0;
    d->pClass->postKeyEvent(d, type, charCode, keyCode);
    return 1;
}
static int go_doc_post_mouse_event(LibreOfficeKitDocument* d, int type, int x, int y,
    int count, int buttons, int mods) {
    if (d == NULL || d->pClass == NULL || d->pClass->postMouseEvent == NULL) return 0;
    d->pClass->postMouseEvent(d, type, x, y, count, buttons, mods);
    return 1;
}
static int go_doc_post_uno_command(LibreOfficeKitDocument* d, const char* cmd,
    const char* args, bool notifyWhenFinished) {
    if (d == NULL || d->pClass == NULL || d->pClass->postUnoCommand == NULL) return 0;
    d->pClass->postUnoCommand(d, cmd, args, notifyWhenFinished);
    return 1;
}
*/
import "C"

import "unsafe"

// DocumentPostKeyEvent forwards to pClass->postKeyEvent. typ is
// a LOK_KEYEVENT_* value; charCode is a Unicode code point (0 for
// non-printables); keyCode is a com::sun::star::awt::Key value
// (0 for plain characters). Returns ErrUnsupported when the vtable
// slot is NULL.
func DocumentPostKeyEvent(d DocumentHandle, typ, charCode, keyCode int) error {
	if !d.IsValid() {
		return ErrUnsupported
	}
	if C.go_doc_post_key_event(d.p, C.int(typ), C.int(charCode), C.int(keyCode)) == 0 {
		return ErrUnsupported
	}
	return nil
}

// DocumentPostMouseEvent forwards to pClass->postMouseEvent. typ is
// LOK_MOUSEEVENT_*; x, y are twip coordinates (caller must ensure
// they fit in C int — 32-bit on LP64); count is click count; buttons
// and mods are OR-ed awt::MouseButton and awt::KeyModifier bitsets.
// Returns ErrUnsupported when the vtable slot is NULL.
func DocumentPostMouseEvent(d DocumentHandle, typ, x, y, count, buttons, mods int) error {
	if !d.IsValid() {
		return ErrUnsupported
	}
	if C.go_doc_post_mouse_event(d.p, C.int(typ), C.int(x), C.int(y),
		C.int(count), C.int(buttons), C.int(mods)) == 0 {
		return ErrUnsupported
	}
	return nil
}

// DocumentPostUnoCommand forwards to pClass->postUnoCommand. args
// may be empty; notifyWhenFinished requests a
// LOK_CALLBACK_UNO_COMMAND_RESULT on completion, observable only via
// a registered document callback. Returns ErrUnsupported when the
// vtable slot is NULL.
func DocumentPostUnoCommand(d DocumentHandle, cmd, args string, notifyWhenFinished bool) error {
	if !d.IsValid() {
		return ErrUnsupported
	}
	ccmd := C.CString(cmd)
	defer C.free(unsafe.Pointer(ccmd))
	cargs := C.CString(args)
	defer C.free(unsafe.Pointer(cargs))
	if C.go_doc_post_uno_command(d.p, ccmd, cargs, C.bool(notifyWhenFinished)) == 0 {
		return ErrUnsupported
	}
	return nil
}
