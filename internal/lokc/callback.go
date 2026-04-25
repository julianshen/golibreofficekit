//go:build linux || darwin

package lokc

import (
	"sync"
	"sync/atomic"
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
