//go:build linux || darwin

package lokc

/*
#cgo CFLAGS: -I${SRCDIR}/../../third_party/lok -DLOK_USE_UNSTABLE_API
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include "LibreOfficeKit/LibreOfficeKit.h"

// Forward declarations of the //export Go trampolines so the C code
// below can pass their addresses to LOK.
void goLOKDispatchOffice(int typ, char* payload, void* pData);
void goLOKDispatchDocument(int typ, char* payload, void* pData);

// Returns 1 on success, 0 when the vtable slot is NULL (unsupported).
static int go_office_register_callback(LibreOfficeKit* p, uintptr_t handle) {
    if (p == NULL || p->pClass == NULL || p->pClass->registerCallback == NULL) return 0;
    p->pClass->registerCallback(p,
                                (LibreOfficeKitCallback)goLOKDispatchOffice,
                                (void*)handle);
    return 1;
}

static int go_doc_register_callback(LibreOfficeKitDocument* d, uintptr_t handle) {
    if (d == NULL || d->pClass == NULL || d->pClass->registerCallback == NULL) return 0;
    d->pClass->registerCallback(d,
                                (LibreOfficeKitCallback)goLOKDispatchDocument,
                                (void*)handle);
    return 1;
}
*/
import "C"

import (
	"sync"
	"sync/atomic"
	"unsafe"
)

// Dispatcher is the lok-side adapter the cgo trampoline routes
// events into. The concrete implementation lives in lok/listener.go;
// internal/lokc only knows the interface.
type Dispatcher interface {
	Dispatch(typ int, payload []byte)
}

// dispatchHandle is an opaque integer key the cgo trampoline
// receives via pData. 0 is reserved as "unregistered".
type dispatchHandle uintptr

var (
	handleNext  atomic.Uintptr // monotonic; 0 reserved
	handleMu    sync.RWMutex
	handleTable = map[dispatchHandle]Dispatcher{}
)

// RegisterDispatcher adds d to the handle table and returns the
// opaque handle that should be passed to LOK as pData. Subsequent
// trampoline invocations carrying this handle will be routed to d.
func RegisterDispatcher(d Dispatcher) dispatchHandle {
	h := dispatchHandle(handleNext.Add(1))
	handleMu.Lock()
	handleTable[h] = d
	handleMu.Unlock()
	return h
}

// UnregisterDispatcher removes h from the handle table. Subsequent
// trampoline lookups for h return nil (a safe no-op).
func UnregisterDispatcher(h dispatchHandle) {
	handleMu.Lock()
	delete(handleTable, h)
	handleMu.Unlock()
}

// lookupDispatcher returns the Dispatcher registered under h, or nil
// when h is 0 or has been unregistered.
func lookupDispatcher(h dispatchHandle) Dispatcher {
	handleMu.RLock()
	defer handleMu.RUnlock()
	return handleTable[h]
}

// dispatch is the shared trampoline body. The two //export functions
// differ only in name (so stack traces distinguish office vs doc
// callbacks) and delegate to this shared logic.
func dispatch(typ C.int, payload *C.char, pData unsafe.Pointer) {
	h := dispatchHandle(uintptr(pData))
	d := lookupDispatcher(h)
	if d == nil {
		return
	}
	var b []byte
	if payload != nil {
		b = C.GoBytes(unsafe.Pointer(payload), C.int(C.strlen(payload)))
	}
	d.Dispatch(int(typ), b)
}

//export goLOKDispatchOffice
func goLOKDispatchOffice(typ C.int, payload *C.char, pData unsafe.Pointer) {
	dispatch(typ, payload, pData)
}

//export goLOKDispatchDocument
func goLOKDispatchDocument(typ C.int, payload *C.char, pData unsafe.Pointer) {
	dispatch(typ, payload, pData)
}

// DispatchHandleFromUintptr converts a caller-managed uintptr into
// the package's dispatchHandle. The unexported handle type is
// intentional; conversions cross via these helpers so callers don't
// depend on the concrete type.
func DispatchHandleFromUintptr(v uintptr) dispatchHandle { return dispatchHandle(v) }

// UintptrFromDispatchHandle is the inverse of DispatchHandleFromUintptr.
func UintptrFromDispatchHandle(h dispatchHandle) uintptr { return uintptr(h) }

// RegisterDispatcherUintptr is a convenience wrapper around
// RegisterDispatcher that returns the handle as a plain uintptr so
// callers don't depend on the unexported dispatchHandle type.
func RegisterDispatcherUintptr(d Dispatcher) uintptr {
	return uintptr(RegisterDispatcher(d))
}

// UnregisterDispatcherUintptr is the symmetric inverse.
func UnregisterDispatcherUintptr(h uintptr) {
	UnregisterDispatcher(dispatchHandle(h))
}

// RegisterOfficeCallback wires the Office-level trampoline into LOK
// using h as the pData handle. Returns ErrUnsupported when the
// vtable slot is NULL.
func RegisterOfficeCallback(o OfficeHandle, h dispatchHandle) error {
	if !o.IsValid() {
		return ErrUnsupported
	}
	if C.go_office_register_callback(o.p, C.uintptr_t(h)) == 0 {
		return ErrUnsupported
	}
	return nil
}

// LookupDispatcherForTest exposes lookupDispatcher to tests in other
// packages. Production code must not call it.
func LookupDispatcherForTest(h uintptr) Dispatcher {
	return lookupDispatcher(dispatchHandle(h))
}

// RegisterDocumentCallback wires the Document-level trampoline into
// LOK using h as the pData handle. Returns ErrUnsupported when the
// vtable slot is NULL.
func RegisterDocumentCallback(d DocumentHandle, h dispatchHandle) error {
	if !d.IsValid() {
		return ErrUnsupported
	}
	if C.go_doc_register_callback(d.p, C.uintptr_t(h)) == 0 {
		return ErrUnsupported
	}
	return nil
}
