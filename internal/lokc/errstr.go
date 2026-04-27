//go:build linux || darwin

package lokc

/*
#include <stdlib.h>
*/
import "C"

import (
	"errors"
	"unsafe"
)

// ErrUnsupported is returned when the LOK function pointer for an
// operation is NULL on the loaded LibreOffice build. realBackend
// translates this into the public lok.ErrUnsupported sentinel via
// errors.Is so callers don't depend on internal/lokc.
var ErrUnsupported = errors.New("lokc: LOK vtable slot is NULL")

// ErrNoValue is returned when LOK accepted the call but produced no
// payload. Distinct from ErrUnsupported (which means "the LO build
// does not expose this operation at all"). Callers can use ErrNoValue
// to detect e.g. "command exists but has no current value to report"
// without confusing it with a missing vtable slot.
var ErrNoValue = errors.New("lokc: LOK returned NULL/no value")

// copyAndFree copies a C string into a Go string and frees the
// original with free(3). Safe on nil input (returns "").
//
// LOK returns char* from getError / getVersionInfo / etc. that the
// caller owns; every wrapper that sees such a pointer should pass it
// through here so the free cannot be forgotten.
func copyAndFree(cs *C.char) string {
	if cs == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(cs))
	return C.GoString(cs)
}
