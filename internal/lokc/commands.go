//go:build linux || darwin

package lokc

/*
#cgo CFLAGS: -I${SRCDIR}/../../third_party/lok -DLOK_USE_UNSTABLE_API
#include <stdlib.h>
#include "LibreOfficeKit/LibreOfficeKit.h"
#include "commands.h"
*/
import "C"
import (
	"unsafe"
)

// DocumentGetCommandValues calls pClass->getCommandValues.
// Returns the JSON string on success, or an error.
// The returned string must be freed by the caller via C.free.
func DocumentGetCommandValues(d DocumentHandle, command string) (string, error) {
	if !d.IsValid() {
		return "", ErrNilDocument
	}
	var out *C.char
	var outLen C.size_t
	cCmd := C.CString(command)
	defer C.free(unsafe.Pointer(cCmd))
	ok := C.loke_get_command_values(unsafe.Pointer(d.p), cCmd, &out, &outLen)
	if ok == 0 {
		return "", ErrUnsupported
	}
	defer C.free(unsafe.Pointer(out))
	return C.GoStringN(out, C.int(outLen)), nil
}

// DocumentCompleteFunction calls pClass->completeFunction.
// Returns ErrUnsupported if the vtable slot is missing.
func DocumentCompleteFunction(d DocumentHandle, name string) error {
	if !d.IsValid() {
		return ErrNilDocument
	}
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	ok := C.loke_complete_function(unsafe.Pointer(d.p), cName)
	if ok == 0 {
		return ErrUnsupported
	}
	return nil
}

// DocumentSendDialogEvent calls pClass->sendDialogEvent.
func DocumentSendDialogEvent(d DocumentHandle, windowID uint64, argsJSON string) error {
	if !d.IsValid() {
		return ErrNilDocument
	}
	cArgs := C.CString(argsJSON)
	defer C.free(unsafe.Pointer(cArgs))
	ok := C.loke_doc_send_dialog_event(unsafe.Pointer(d.p), C.uint64_t(windowID), cArgs)
	if ok == 0 {
		return ErrUnsupported
	}
	return nil
}

// DocumentSendContentControlEvent calls pClass->sendContentControlEvent.
func DocumentSendContentControlEvent(d DocumentHandle, argsJSON string) error {
	if !d.IsValid() {
		return ErrNilDocument
	}
	cArgs := C.CString(argsJSON)
	defer C.free(unsafe.Pointer(cArgs))
	ok := C.loke_doc_send_content_control_event(unsafe.Pointer(d.p), cArgs)
	if ok == 0 {
		return ErrUnsupported
	}
	return nil
}

// DocumentSendFormFieldEvent calls pClass->sendFormFieldEvent.
func DocumentSendFormFieldEvent(d DocumentHandle, argsJSON string) error {
	if !d.IsValid() {
		return ErrNilDocument
	}
	cArgs := C.CString(argsJSON)
	defer C.free(unsafe.Pointer(cArgs))
	ok := C.loke_doc_send_form_field_event(unsafe.Pointer(d.p), cArgs)
	if ok == 0 {
		return ErrUnsupported
	}
	return nil
}
