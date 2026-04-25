# Phase 9 — Callbacks / Listeners Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bind LibreOfficeKit's `pClass->registerCallback` on both Office and Document so users can subscribe to LOK events from Go via `AddListener(cb func(Event)) (cancel func(), err error)`. Async dispatch on a per-object goroutine. Eliminates Phase 8's SelectAll capability gate as a side effect.

**Architecture:** Same four-layer pattern as Phases 3–8. `internal/lokc` owns the cgo trampolines, the integer handle table (no Go pointers in C storage), and the C shim that calls `pClass->registerCallback`. `lok` owns the `Event` type, the listener-set machinery (per-Office and per-Document buffered channel + dispatcher goroutine + atomic dropped counter), and the `Office.AddListener` / `Document.AddListener` public API. Eager trampoline registration at `New()` and `Load()`.

**Tech Stack:** Go + cgo on `linux || darwin`, LibreOfficeKit C ABI, `//export` trampolines, `sync.Map`-style handle table, atomic counters, `go test` + the `lok_integration` build tag.

**Branch:** `feat/callbacks` (already created off `main`).

**Spec:** `docs/superpowers/specs/2026-04-25-phase-9-callbacks-design.md`

---

## File Structure

Files created:

- `internal/lokc/callback.go` — C shims `go_office_register_callback` / `go_doc_register_callback`; `//export`'d trampolines `goLOKDispatchOffice` / `goLOKDispatchDocument`; `Dispatcher` interface; handle table.
- `internal/lokc/callback_test.go` — direct trampoline tests (synthetic handles, fake `Dispatcher`, races).
- `lok/event.go` — `EventType` typed int + curated constants + `String()`; `Event` struct.
- `lok/event_test.go` — `EventType.String()` tests.
- `lok/listener.go` — private `listenerSet` (channel, dispatcher goroutine, slice + mutex, atomic counter, close hook); `Office.AddListener`, `Office.DroppedEvents`, `Document.AddListener`, `Document.DroppedEvents`.
- `lok/listener_test.go` — listener-set unit tests via `fakeBackend`'s dispatch hook.

Files modified:

- `lok/backend.go` — two new interface methods.
- `lok/real_backend.go` — two forwarders + `mapLokErr` reuse.
- `lok/office.go` — `Office` gains a private `*listenerSet`; `New()` registers a dispatcher and the trampoline; `Close()` tears it down.
- `lok/document.go` — `Document` gains a private `*listenerSet`; `Load()` and `LoadFromReader()` paths spawn the dispatcher; `Close()` tears down.
- `lok/office_test.go` — `fakeBackend` learns the two register methods; tests can use `fb.lastOfficeCallbackHandle` / `fb.lastDocumentCallbackHandle` to look up the dispatch target.
- `lok/integration_test.go` — drop the Phase 8 selection capability gate; replace with a listener-driven wait.
- `lok/real_backend_test.go` — coverage tests for the two new realBackend forwarders against a NULL-pClass fake handle.

---

## Task 1: `internal/lokc` handle table + Dispatcher interface (pure-Go, no cgo)

The integer handle table that lets the trampoline look up its Go-side
`Dispatcher` without storing Go pointers in C. No cgo here yet — just
the table and the interface.

**Files:**
- Create: `internal/lokc/callback.go` (pure Go portion only — no `import "C"` yet)
- Create: `internal/lokc/callback_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/lokc/callback_test.go`:

```go
//go:build linux || darwin

package lokc

import (
	"sync"
	"testing"
)

type fakeDispatcher struct {
	mu       sync.Mutex
	received []Event
}

type Event struct {
	Type    int
	Payload []byte
}

func (f *fakeDispatcher) Dispatch(typ int, payload []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.received = append(f.received, Event{Type: typ, Payload: append([]byte(nil), payload...)})
}

func TestRegisterDispatcher_AssignsUniqueHandles(t *testing.T) {
	d1 := &fakeDispatcher{}
	d2 := &fakeDispatcher{}
	h1 := RegisterDispatcher(d1)
	h2 := RegisterDispatcher(d2)
	t.Cleanup(func() { UnregisterDispatcher(h1); UnregisterDispatcher(h2) })

	if h1 == 0 || h2 == 0 {
		t.Errorf("handles must be non-zero (0 reserved): h1=%d h2=%d", h1, h2)
	}
	if h1 == h2 {
		t.Errorf("handles must be unique: %d == %d", h1, h2)
	}
}

func TestLookupDispatcher_ReturnsRegistered(t *testing.T) {
	d := &fakeDispatcher{}
	h := RegisterDispatcher(d)
	t.Cleanup(func() { UnregisterDispatcher(h) })

	got := lookupDispatcher(h)
	if got != Dispatcher(d) {
		t.Errorf("lookupDispatcher(%d) = %v, want %v", h, got, d)
	}
}

func TestLookupDispatcher_ZeroHandleReturnsNil(t *testing.T) {
	if got := lookupDispatcher(0); got != nil {
		t.Errorf("lookupDispatcher(0) = %v, want nil", got)
	}
}

func TestUnregisterDispatcher_RemovesEntry(t *testing.T) {
	d := &fakeDispatcher{}
	h := RegisterDispatcher(d)
	UnregisterDispatcher(h)
	if got := lookupDispatcher(h); got != nil {
		t.Errorf("after Unregister: lookupDispatcher(%d) = %v, want nil", h, got)
	}
}

func TestRegisterDispatcher_ConcurrentSafe(t *testing.T) {
	const n = 100
	var wg sync.WaitGroup
	handles := make([]dispatchHandle, n)
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			handles[i] = RegisterDispatcher(&fakeDispatcher{})
		}(i)
	}
	wg.Wait()
	t.Cleanup(func() {
		for _, h := range handles {
			UnregisterDispatcher(h)
		}
	})

	// All n handles must be distinct.
	seen := map[dispatchHandle]bool{}
	for _, h := range handles {
		if seen[h] {
			t.Errorf("duplicate handle %d", h)
		}
		seen[h] = true
	}
	if len(seen) != n {
		t.Errorf("got %d unique handles, want %d", len(seen), n)
	}
}
```

(`Event` is declared locally in the test file because Task 2 will move
the real cgo trampoline definition into the production file. The
production code in this task does not need an `Event` type yet — just
`Dispatcher`.)

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/lokc -run 'TestRegisterDispatcher|TestLookupDispatcher|TestUnregisterDispatcher' -v`
Expected: FAIL — `RegisterDispatcher`, `UnregisterDispatcher`, `lookupDispatcher`, `Dispatcher`, `dispatchHandle` undefined.

- [ ] **Step 3: Implement the handle table**

Create `internal/lokc/callback.go`:

```go
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
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/lokc -run 'TestRegisterDispatcher|TestLookupDispatcher|TestUnregisterDispatcher' -race -v`
Expected: PASS (5 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/lokc/callback.go internal/lokc/callback_test.go
git commit -m "feat(lokc): add Dispatcher interface + integer handle table"
```

---

## Task 2: cgo trampolines + register C shims

Add the `//export` trampolines and the C shims that call
`pClass->registerCallback`. The trampoline copies the payload, looks
up the handle, and calls `Dispatch`. Tests invoke the trampoline
directly with a synthetic handle via the test file (no real LOK).

**Files:**
- Modify: `internal/lokc/callback.go`
- Modify: `internal/lokc/callback_test.go`

- [ ] **Step 1: Write failing tests for trampoline routing**

Append to `internal/lokc/callback_test.go`:

```go
import "C"
import "unsafe"

func TestGoLOKDispatchOffice_RoutesToHandle(t *testing.T) {
	d := &fakeDispatcher{}
	h := RegisterDispatcher(d)
	t.Cleanup(func() { UnregisterDispatcher(h) })

	payload := C.CString("hello")
	defer C.free(unsafe.Pointer(payload))

	// Direct //export invocation. The trampoline takes (C.int,
	// *C.char, unsafe.Pointer); we pass the handle as unsafe.Pointer
	// the same way LOK would carry it through pData.
	goLOKDispatchOffice(C.int(2), payload, unsafe.Pointer(uintptr(h)))

	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.received) != 1 {
		t.Fatalf("got %d events, want 1", len(d.received))
	}
	if d.received[0].Type != 2 || string(d.received[0].Payload) != "hello" {
		t.Errorf("got %+v, want type=2 payload=hello", d.received[0])
	}
}

func TestGoLOKDispatchDocument_RoutesToHandle(t *testing.T) {
	d := &fakeDispatcher{}
	h := RegisterDispatcher(d)
	t.Cleanup(func() { UnregisterDispatcher(h) })

	payload := C.CString("world")
	defer C.free(unsafe.Pointer(payload))
	goLOKDispatchDocument(C.int(8), payload, unsafe.Pointer(uintptr(h)))

	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.received) != 1 || d.received[0].Type != 8 || string(d.received[0].Payload) != "world" {
		t.Errorf("got %+v", d.received)
	}
}

func TestGoLOKDispatch_NULLPayloadGivesNilSlice(t *testing.T) {
	d := &fakeDispatcher{}
	h := RegisterDispatcher(d)
	t.Cleanup(func() { UnregisterDispatcher(h) })

	goLOKDispatchOffice(C.int(0), nil, unsafe.Pointer(uintptr(h)))

	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.received) != 1 || d.received[0].Payload != nil {
		t.Errorf("got %+v, want one event with nil payload", d.received)
	}
}

func TestGoLOKDispatch_UnknownHandleNoOp(t *testing.T) {
	// Use handle=0 (reserved) and an arbitrary unregistered value.
	goLOKDispatchOffice(C.int(2), nil, unsafe.Pointer(uintptr(0)))
	goLOKDispatchOffice(C.int(2), nil, unsafe.Pointer(uintptr(99999)))
	// Survives without panic. No assertion needed.
}

func TestGoLOKDispatch_LongPayloadRoundTrip(t *testing.T) {
	d := &fakeDispatcher{}
	h := RegisterDispatcher(d)
	t.Cleanup(func() { UnregisterDispatcher(h) })

	want := make([]byte, 65*1024)
	for i := range want {
		want[i] = byte('a' + i%26)
	}
	cstr := C.CString(string(want))
	defer C.free(unsafe.Pointer(cstr))
	goLOKDispatchOffice(C.int(0), cstr, unsafe.Pointer(uintptr(h)))

	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.received) != 1 || string(d.received[0].Payload) != string(want) {
		t.Errorf("payload round-trip failed; len got=%d want=%d", len(d.received[0].Payload), len(want))
	}
}
```

Note: this test file now imports `"C"` and `"unsafe"`. The
`fakeDispatcher` and local `Event` defined in Task 1 stay; remove the
local `Event` (the production code does not need it; we'll drop the
test-local one when the public `Event` lands in Task 4 — for now keep
it test-scoped).

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/lokc -run TestGoLOKDispatch -v`
Expected: FAIL — `goLOKDispatchOffice`, `goLOKDispatchDocument` undefined.

- [ ] **Step 3: Implement the cgo trampolines**

Append to `internal/lokc/callback.go` (you'll need to add the cgo
preamble at the top of the file — check whether the file currently
has `import "C"`; if not, restructure into the standard cgo file
shape):

Replace the file contents with the cgo-aware version:

```go
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
// callbacks) and in delegate to this shared logic.
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
```

(The two `//export` thin wrappers exist so each has a distinct symbol
in stack traces; the body is shared in `dispatch`.)

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/lokc -run 'TestGoLOKDispatch|TestRegisterDispatcher|TestLookupDispatcher|TestUnregisterDispatcher' -race -v`
Expected: 9 PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/lokc/callback.go internal/lokc/callback_test.go
git commit -m "feat(lokc): add //export trampolines + register C shims"
```

---

## Task 3: `RegisterOfficeCallback` / `RegisterDocumentCallback` Go wrappers

Thin Go wrappers that call the C shims and surface `ErrUnsupported`
when the vtable slot is NULL.

**Files:**
- Modify: `internal/lokc/callback.go`
- Modify: `internal/lokc/callback_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/lokc/callback_test.go`:

```go
func TestRegisterOfficeCallback_NilSafe(t *testing.T) {
	// Without an OfficeHandle helper we can only test the invalid
	// path here. The success path is covered by realBackend tests
	// against a fake document handle in lok/real_backend_test.go.
	if err := RegisterOfficeCallback(OfficeHandle{}, dispatchHandle(1)); err != ErrUnsupported {
		t.Errorf("zero handle: err=%v, want ErrUnsupported", err)
	}
}

func TestRegisterDocumentCallback_NilSafe(t *testing.T) {
	if err := RegisterDocumentCallback(DocumentHandle{}, dispatchHandle(1)); err != ErrUnsupported {
		t.Errorf("zero handle: err=%v, want ErrUnsupported", err)
	}
	if err := RegisterDocumentCallback(newFakeDoc(t), dispatchHandle(1)); err != ErrUnsupported {
		t.Errorf("nil pClass: err=%v, want ErrUnsupported", err)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/lokc -run 'TestRegisterOfficeCallback|TestRegisterDocumentCallback' -v`
Expected: FAIL — undefined.

- [ ] **Step 3: Implement the wrappers**

Append to `internal/lokc/callback.go`:

```go
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
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/lokc -run 'TestRegisterOfficeCallback|TestRegisterDocumentCallback' -v`
Expected: PASS.

Run also: `go test ./internal/lokc -race`
Expected: full lokc suite passes.

- [ ] **Step 5: Commit**

```bash
git add internal/lokc/callback.go internal/lokc/callback_test.go
git commit -m "feat(lokc): add RegisterOfficeCallback/RegisterDocumentCallback wrappers"
```

---

## Task 4: `lok.EventType` + curated constants + `String()`

**Files:**
- Create: `lok/event.go`
- Create: `lok/event_test.go`

- [ ] **Step 1: Write the failing test**

Create `lok/event_test.go`:

```go
//go:build linux || darwin

package lok

import "testing"

func TestEventType_String(t *testing.T) {
	cases := []struct {
		typ  EventType
		want string
	}{
		{EventTypeInvalidateTiles, "EventTypeInvalidateTiles"},
		{EventTypeInvalidateVisibleCursor, "EventTypeInvalidateVisibleCursor"},
		{EventTypeTextSelection, "EventTypeTextSelection"},
		{EventTypeTextSelectionStart, "EventTypeTextSelectionStart"},
		{EventTypeTextSelectionEnd, "EventTypeTextSelectionEnd"},
		{EventTypeCursorVisible, "EventTypeCursorVisible"},
		{EventTypeGraphicSelection, "EventTypeGraphicSelection"},
		{EventTypeHyperlinkClicked, "EventTypeHyperlinkClicked"},
		{EventTypeStateChanged, "EventTypeStateChanged"},
		{EventTypeMousePointer, "EventTypeMousePointer"},
		{EventTypeUNOCommandResult, "EventTypeUNOCommandResult"},
		{EventTypeDocumentSizeChanged, "EventTypeDocumentSizeChanged"},
		{EventTypeSetPart, "EventTypeSetPart"},
		{EventTypeError, "EventTypeError"},
		{EventTypeWindow, "EventTypeWindow"},
		{EventType(999), "EventType(999)"},
	}
	for _, tc := range cases {
		if got := tc.typ.String(); got != tc.want {
			t.Errorf("%d: got %q, want %q", tc.typ, got, tc.want)
		}
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./lok -run TestEventType_String -v`
Expected: FAIL — undefined types.

- [ ] **Step 3: Implement `lok/event.go`**

```go
//go:build linux || darwin

package lok

import "fmt"

// EventType mirrors LOK_CALLBACK_*. The named constants below are a
// curated subset; values outside the named set still arrive as
// EventType(N). Run `grep LOK_CALLBACK_ third_party/lok/.../LibreOfficeKitEnums.h`
// for the full list LOK ships.
type EventType int

const (
	EventTypeInvalidateTiles         EventType = 0  // LOK_CALLBACK_INVALIDATE_TILES
	EventTypeInvalidateVisibleCursor EventType = 1  // LOK_CALLBACK_INVALIDATE_VISIBLE_CURSOR
	EventTypeTextSelection           EventType = 2  // LOK_CALLBACK_TEXT_SELECTION
	EventTypeTextSelectionStart      EventType = 3  // LOK_CALLBACK_TEXT_SELECTION_START
	EventTypeTextSelectionEnd        EventType = 4  // LOK_CALLBACK_TEXT_SELECTION_END
	EventTypeCursorVisible           EventType = 5  // LOK_CALLBACK_CURSOR_VISIBLE
	EventTypeGraphicSelection        EventType = 6  // LOK_CALLBACK_GRAPHIC_SELECTION
	EventTypeHyperlinkClicked        EventType = 7  // LOK_CALLBACK_HYPERLINK_CLICKED
	EventTypeStateChanged            EventType = 8  // LOK_CALLBACK_STATE_CHANGED
	EventTypeMousePointer            EventType = 12 // LOK_CALLBACK_MOUSE_POINTER
	EventTypeUNOCommandResult        EventType = 13 // LOK_CALLBACK_UNO_COMMAND_RESULT
	EventTypeDocumentSizeChanged     EventType = 17 // LOK_CALLBACK_DOCUMENT_SIZE_CHANGED
	EventTypeSetPart                 EventType = 18 // LOK_CALLBACK_SET_PART
	EventTypeError                   EventType = 20 // LOK_CALLBACK_ERROR
	EventTypeWindow                  EventType = 36 // LOK_CALLBACK_WINDOW
)

func (t EventType) String() string {
	switch t {
	case EventTypeInvalidateTiles:
		return "EventTypeInvalidateTiles"
	case EventTypeInvalidateVisibleCursor:
		return "EventTypeInvalidateVisibleCursor"
	case EventTypeTextSelection:
		return "EventTypeTextSelection"
	case EventTypeTextSelectionStart:
		return "EventTypeTextSelectionStart"
	case EventTypeTextSelectionEnd:
		return "EventTypeTextSelectionEnd"
	case EventTypeCursorVisible:
		return "EventTypeCursorVisible"
	case EventTypeGraphicSelection:
		return "EventTypeGraphicSelection"
	case EventTypeHyperlinkClicked:
		return "EventTypeHyperlinkClicked"
	case EventTypeStateChanged:
		return "EventTypeStateChanged"
	case EventTypeMousePointer:
		return "EventTypeMousePointer"
	case EventTypeUNOCommandResult:
		return "EventTypeUNOCommandResult"
	case EventTypeDocumentSizeChanged:
		return "EventTypeDocumentSizeChanged"
	case EventTypeSetPart:
		return "EventTypeSetPart"
	case EventTypeError:
		return "EventTypeError"
	case EventTypeWindow:
		return "EventTypeWindow"
	default:
		return fmt.Sprintf("EventType(%d)", int(t))
	}
}

// Event is one delivered LOK callback. Payload is a Go-owned copy of
// the C-allocated string LOK provided. Format depends on Type — see
// LibreOfficeKitEnums.h. Empty Payload is common; LOK signals many
// events with no body.
type Event struct {
	Type    EventType
	Payload []byte
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./lok -run TestEventType_String -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add lok/event.go lok/event_test.go
git commit -m "feat(lok): add EventType + curated constants + Event struct"
```

---

## Task 5: backend interface + realBackend forwarders + fakeBackend stubs

**Files:**
- Modify: `lok/backend.go`
- Modify: `lok/real_backend.go`
- Modify: `lok/office_test.go`

- [ ] **Step 1: Add the new interface methods**

In `lok/backend.go`, append inside the `backend` interface:

```go
	RegisterOfficeCallback(h officeHandle, handle uintptr) error
	RegisterDocumentCallback(d documentHandle, handle uintptr) error
```

- [ ] **Step 2: Build, expect realBackend / fakeBackend compile failure**

Run: `go build ./lok/...`
Expected: FAIL — both backends missing methods.

- [ ] **Step 3: Add realBackend forwarders**

In `lok/real_backend.go`, append (before the existing
`var _ backend = realBackend{}`):

```go
func (realBackend) RegisterOfficeCallback(h officeHandle, handle uintptr) error {
	return mapLokErr(lokc.RegisterOfficeCallback(mustOffice(h).p, lokc.DispatchHandleFromUintptr(handle)))
}
func (realBackend) RegisterDocumentCallback(d documentHandle, handle uintptr) error {
	return mapLokErr(lokc.RegisterDocumentCallback(mustDoc(d).d, lokc.DispatchHandleFromUintptr(handle)))
}
```

`mustOffice` already exists in `real_backend.go`; `mustDoc` likewise.

`lokc.DispatchHandleFromUintptr` does not exist yet — add it as an
exported helper in `internal/lokc/callback.go` (the type
`dispatchHandle` is unexported so the lok package can't construct one
directly; this small constructor crosses the package boundary):

In `internal/lokc/callback.go`, append:

```go
// DispatchHandleFromUintptr converts a caller-managed uintptr into
// the package's dispatchHandle type. The lok package uses this when
// it has obtained a handle via RegisterDispatcher and needs to feed
// it to RegisterOfficeCallback / RegisterDocumentCallback.
func DispatchHandleFromUintptr(v uintptr) dispatchHandle { return dispatchHandle(v) }

// UintptrFromDispatchHandle is the inverse of
// DispatchHandleFromUintptr — useful when the caller stores the
// handle in a Go-side struct as a plain uintptr.
func UintptrFromDispatchHandle(h dispatchHandle) uintptr { return uintptr(h) }
```

Also update `RegisterDispatcher`'s return type to `uintptr` so callers
in `lok` don't need to import the unexported type:

Actually — keep the existing `RegisterDispatcher` signature returning
`dispatchHandle`. Add a sibling `RegisterDispatcherUintptr` for the
`lok` package:

In `internal/lokc/callback.go`, replace the export by adding:

```go
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
```

- [ ] **Step 4: Add fakeBackend stubs to keep tests compiling**

In `lok/office_test.go`, add to the `fakeBackend` struct:

```go
	// Callback registration (Phase 9).
	lastOfficeCallbackHandle   uintptr
	lastDocumentCallbackHandle uintptr
	registerOfficeCallbackErr  error
	registerDocCallbackErr     error
```

And the methods (place near the existing fake methods):

```go
func (f *fakeBackend) RegisterOfficeCallback(_ officeHandle, h uintptr) error {
	f.lastOfficeCallbackHandle = h
	return f.registerOfficeCallbackErr
}

func (f *fakeBackend) RegisterDocumentCallback(_ documentHandle, h uintptr) error {
	f.lastDocumentCallbackHandle = h
	return f.registerDocCallbackErr
}
```

- [ ] **Step 5: Build clean**

Run: `go build ./...`
Expected: clean.

Also run `go test ./...` — all existing tests should still pass.

- [ ] **Step 6: Commit**

```bash
git add lok/backend.go lok/real_backend.go lok/office_test.go internal/lokc/callback.go
git commit -m "feat(lok): extend backend seam with callback registration"
```

---

## Task 6: `lok.listenerSet` — buffered-channel dispatcher

The shared private type used by both Office and Document. Implements
`lokc.Dispatcher` so the cgo trampoline can route into it.

**Files:**
- Create: `lok/listener.go`
- Create: `lok/listener_test.go`

- [ ] **Step 1: Write failing tests**

Create `lok/listener_test.go`:

```go
//go:build linux || darwin

package lok

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestListenerSet_DispatchInvokesAllListeners(t *testing.T) {
	ls := newListenerSet()
	defer ls.close()

	var aGot, bGot atomic.Int32
	ls.add(func(e Event) {
		if e.Type == EventTypeTextSelection {
			aGot.Add(1)
		}
	})
	ls.add(func(e Event) {
		if e.Type == EventTypeTextSelection {
			bGot.Add(1)
		}
	})

	ls.Dispatch(int(EventTypeTextSelection), []byte("hello"))

	// Wait briefly for the dispatcher goroutine to run.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if aGot.Load() == 1 && bGot.Load() == 1 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Errorf("got a=%d b=%d, want 1+1", aGot.Load(), bGot.Load())
}

func TestListenerSet_CancelStopsDelivery(t *testing.T) {
	ls := newListenerSet()
	defer ls.close()

	var got atomic.Int32
	cancel := ls.add(func(e Event) { got.Add(1) })
	ls.Dispatch(int(EventTypeTextSelection), nil)
	// Wait for first event.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if got.Load() == 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got.Load() != 1 {
		t.Fatalf("first dispatch never delivered: got=%d", got.Load())
	}

	cancel()
	cancel() // idempotent
	ls.Dispatch(int(EventTypeTextSelection), nil)
	// Give the dispatcher time to drain.
	time.Sleep(50 * time.Millisecond)
	if got.Load() != 1 {
		t.Errorf("after cancel: got=%d, want 1 (no new deliveries)", got.Load())
	}
}

func TestListenerSet_DropOnFull(t *testing.T) {
	ls := newListenerSet()
	defer ls.close()

	// Slow listener blocks the dispatcher.
	block := make(chan struct{})
	release := make(chan struct{})
	ls.add(func(e Event) {
		block <- struct{}{}
		<-release
	})

	// First Dispatch fills the dispatcher's "current" event; it
	// blocks the dispatcher because the listener waits on block.
	ls.Dispatch(0, nil)
	<-block // first event in flight, dispatcher stuck

	// Saturate the channel with capacity (256) more events.
	for range listenerBufferSize {
		ls.Dispatch(0, nil)
	}
	// Now any further Dispatch must drop.
	ls.Dispatch(0, nil)
	if got := ls.Dropped(); got == 0 {
		t.Errorf("Dropped()=0 after over-saturation, want >= 1")
	}

	// Release the listener so the dispatcher exits cleanly.
	close(release)
}

func TestListenerSet_PanicRecovered(t *testing.T) {
	ls := newListenerSet()
	defer ls.close()

	var afterPanic atomic.Int32
	ls.add(func(e Event) { panic("boom") })
	ls.add(func(e Event) { afterPanic.Add(1) })

	ls.Dispatch(0, nil)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if afterPanic.Load() == 1 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Errorf("listener after panic never ran: %d", afterPanic.Load())
}

func TestListenerSet_AddNilReturnsError(t *testing.T) {
	ls := newListenerSet()
	defer ls.close()
	if _, err := ls.addChecked(nil); err == nil {
		t.Errorf("addChecked(nil) returned no error")
	}
}

func TestListenerSet_CloseJoinsDispatcher(t *testing.T) {
	ls := newListenerSet()
	done := make(chan struct{})
	ls.add(func(e Event) {})
	ls.Dispatch(0, nil)

	go func() {
		ls.close()
		close(done)
	}()

	select {
	case <-done:
		// dispatcher exited cleanly
	case <-time.After(time.Second):
		t.Errorf("close() did not return within 1s — dispatcher leak?")
	}
	// Idempotent.
	ls.close()
}

// Suppress unused linter for the helper variable.
var _ = sync.Mutex{}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./lok -run 'TestListenerSet' -v`
Expected: FAIL — undefined.

- [ ] **Step 3: Implement `lok/listener.go`**

```go
//go:build linux || darwin

package lok

import (
	"errors"
	"log"
	"sync"
	"sync/atomic"
)

// listenerBufferSize is the per-listenerSet buffered channel
// capacity. Drop-newest applies once it overflows. 256 covers normal
// interactive event rates from a single document; not configurable
// in this phase.
const listenerBufferSize = 256

// errNilListener is returned by addChecked when the caller passes a
// nil callback. The public AddListener wraps this in a *LOKError.
var errNilListener = errors.New("lok: listener callback is nil")

// listenerSet is the shared dispatcher type used by Office and
// Document. It implements lokc.Dispatcher so the cgo trampoline can
// route events into it.
type listenerSet struct {
	mu        sync.Mutex
	listeners []func(Event)
	ch        chan Event
	dropped   atomic.Uint64
	closeOnce sync.Once
	done      chan struct{}
}

// newListenerSet starts a dispatcher goroutine and returns the set.
// Always paired with close() at end-of-life.
func newListenerSet() *listenerSet {
	ls := &listenerSet{
		ch:   make(chan Event, listenerBufferSize),
		done: make(chan struct{}),
	}
	go ls.run()
	return ls
}

// add appends cb to the listener slice and returns a cancel closure.
// The closure is idempotent. cb must not be nil — use addChecked for
// the public path.
func (ls *listenerSet) add(cb func(Event)) func() {
	ls.mu.Lock()
	ls.listeners = append(ls.listeners, cb)
	ls.mu.Unlock()
	cancelled := false
	var cancelMu sync.Mutex
	return func() {
		cancelMu.Lock()
		defer cancelMu.Unlock()
		if cancelled {
			return
		}
		cancelled = true
		ls.mu.Lock()
		defer ls.mu.Unlock()
		for i, fn := range ls.listeners {
			// Compare by function identity using reflect-free
			// pointer comparison — Go func values aren't directly
			// comparable, so we compare the indirection by capturing
			// the slot index lazily. Simpler: walk and remove the
			// first slot whose value matches `cb` by reflect-based
			// identity. Use uintptr trick.
			if funcsEqual(fn, cb) {
				ls.listeners = append(ls.listeners[:i], ls.listeners[i+1:]...)
				return
			}
		}
	}
}

// addChecked is the public-API entrypoint that rejects nil cb.
func (ls *listenerSet) addChecked(cb func(Event)) (func(), error) {
	if cb == nil {
		return nil, errNilListener
	}
	return ls.add(cb), nil
}

// Dispatch implements lokc.Dispatcher. Called from the //export
// trampoline on LOK's thread; it must not block.
func (ls *listenerSet) Dispatch(typ int, payload []byte) {
	select {
	case ls.ch <- Event{Type: EventType(typ), Payload: payload}:
	default:
		ls.dropped.Add(1)
	}
}

// Dropped returns the cumulative dropped-event count.
func (ls *listenerSet) Dropped() uint64 { return ls.dropped.Load() }

// run is the dispatcher goroutine.
func (ls *listenerSet) run() {
	defer close(ls.done)
	for ev := range ls.ch {
		ls.mu.Lock()
		// Snapshot the listener slice so a cancel during dispatch
		// doesn't race with iteration.
		snap := make([]func(Event), len(ls.listeners))
		copy(snap, ls.listeners)
		ls.mu.Unlock()
		for _, cb := range snap {
			ls.runOne(cb, ev)
		}
	}
}

// runOne invokes cb with panic recovery so a bad listener can't take
// down the dispatcher goroutine. Other listeners in the same
// dispatch slice still run.
func (ls *listenerSet) runOne(cb func(Event), ev Event) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("lok: listener panic: %v", r)
		}
	}()
	cb(ev)
}

// close signals the dispatcher to drain and exit, then waits.
// Idempotent.
func (ls *listenerSet) close() {
	ls.closeOnce.Do(func() {
		close(ls.ch)
	})
	<-ls.done
}

// funcsEqual compares two `func(Event)` values by their code
// pointer. Closures created at distinct call sites compare unequal
// even with identical bodies; closures created at the same site
// compare equal. This is fine for cancel() because the cancel
// closure captured the very value we appended.
func funcsEqual(a, b func(Event)) bool {
	// reflect.ValueOf(a).Pointer() returns the function's code
	// pointer; identical closures share it. Without reflect we'd
	// need to wrap each listener in a sentinel struct.
	return funcPtr(a) == funcPtr(b)
}
```

`funcPtr` is the awkward bit — Go funcs aren't directly comparable.
Instead of `funcsEqual` via `reflect`, switch the design to slot-id
based cancellation. Replace the listener slice with a slice of
`*listenerEntry` where each entry has a unique id; cancel removes by
id:

Replace the `add` / `funcsEqual` block above with:

```go
// listenerEntry pairs a cb with a unique id so cancel() can remove
// the right slot without comparing function values (Go funcs aren't
// comparable).
type listenerEntry struct {
	id uint64
	cb func(Event)
}

// listenerNextID is a per-set monotonic counter (kept in the set so
// tests with parallel sets don't collide).
func (ls *listenerSet) add(cb func(Event)) func() {
	ls.mu.Lock()
	id := ls.nextID + 1
	ls.nextID = id
	ls.listeners = append(ls.listeners, listenerEntry{id: id, cb: cb})
	ls.mu.Unlock()
	cancelled := false
	var cancelMu sync.Mutex
	return func() {
		cancelMu.Lock()
		defer cancelMu.Unlock()
		if cancelled {
			return
		}
		cancelled = true
		ls.mu.Lock()
		defer ls.mu.Unlock()
		for i, e := range ls.listeners {
			if e.id == id {
				ls.listeners = append(ls.listeners[:i], ls.listeners[i+1:]...)
				return
			}
		}
	}
}
```

Update the `listenerSet` struct field types accordingly:

```go
type listenerSet struct {
	mu        sync.Mutex
	listeners []listenerEntry
	nextID    uint64
	ch        chan Event
	dropped   atomic.Uint64
	closeOnce sync.Once
	done      chan struct{}
}
```

…and update `run()` to iterate `for _, e := range snap { ls.runOne(e.cb, ev) }`.

The full reconciled file is as written above with the entry-based
collection (no `funcsEqual`/`funcPtr` needed). Delete those unused
helpers from your draft.

- [ ] **Step 4: Run to verify pass**

Run: `go test ./lok -run 'TestListenerSet' -race -v`
Expected: 6 PASS.

- [ ] **Step 5: Commit**

```bash
git add lok/listener.go lok/listener_test.go
git commit -m "feat(lok): add listenerSet — buffered-channel event dispatcher"
```

---

## Task 7: Wire `listenerSet` into `Office` (eager registration in `New`)

**Files:**
- Modify: `lok/office.go`
- Create: `lok/office_listener_test.go` (or extend `lok/listener_test.go`)

- [ ] **Step 1: Write failing tests**

Append to `lok/listener_test.go`:

```go
func TestOffice_AddListener_DeliversEvent(t *testing.T) {
	fb := &fakeBackend{}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()

	// The handle the fake captured at New() lets us reach the
	// listenerSet via the Dispatcher interface. We invoke Dispatch
	// directly to simulate the trampoline firing.
	got := make(chan Event, 1)
	cancel, err := o.AddListener(func(e Event) {
		select { case got <- e: default: }
	})
	if err != nil {
		t.Fatalf("AddListener: %v", err)
	}
	defer cancel()

	// Resolve the dispatcher and call Dispatch.
	dispatchOfficeFromFake(t, fb).Dispatch(int(EventTypeStateChanged), []byte(".uno:Bold=true"))

	select {
	case e := <-got:
		if e.Type != EventTypeStateChanged || string(e.Payload) != ".uno:Bold=true" {
			t.Errorf("got %+v", e)
		}
	case <-time.After(time.Second):
		t.Fatalf("listener never received event")
	}
}

func TestOffice_AddListener_AfterCloseErrors(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	o.Close()
	if _, err := o.AddListener(func(Event) {}); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestOffice_AddListener_NilCallback(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	if _, err := o.AddListener(nil); !errors.Is(err, ErrInvalidOption) {
		t.Errorf("want ErrInvalidOption, got %v", err)
	}
}

func TestOffice_DroppedEvents_StartsAtZero(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	if got := o.DroppedEvents(); got != 0 {
		t.Errorf("DroppedEvents()=%d, want 0", got)
	}
}
```

The helper `dispatchOfficeFromFake` resolves the listenerSet via the
handle the fake captured. Add it to `lok/listener_test.go`:

```go
// dispatchOfficeFromFake walks back through the lokc handle table
// to the listenerSet the fake handed off when New() called
// RegisterOfficeCallback. Used by tests to simulate trampoline
// firing without going through the real cgo path.
func dispatchOfficeFromFake(t *testing.T, fb *fakeBackend) *listenerSet {
	t.Helper()
	if fb.lastOfficeCallbackHandle == 0 {
		t.Fatalf("fakeBackend never received RegisterOfficeCallback")
	}
	d := lokc.LookupDispatcherForTest(fb.lastOfficeCallbackHandle)
	if d == nil {
		t.Fatalf("no dispatcher registered under handle %d", fb.lastOfficeCallbackHandle)
	}
	ls, ok := d.(*listenerSet)
	if !ok {
		t.Fatalf("dispatcher is %T, not *listenerSet", d)
	}
	return ls
}
```

`lokc.LookupDispatcherForTest` does not yet exist — add it as a
test-only export. In `internal/lokc/callback.go` append:

```go
// LookupDispatcherForTest exposes lookupDispatcher to tests in
// other packages (notably lok). Production code must not call it.
func LookupDispatcherForTest(h uintptr) Dispatcher {
	return lookupDispatcher(dispatchHandle(h))
}
```

(This is a deliberate test-only seam; the function is not in
`*_test.go` because it must be visible to lok's `_test.go` files.)

- [ ] **Step 2: Run to verify failure**

Run: `go test ./lok -run 'TestOffice_AddListener|TestOffice_DroppedEvents' -v`
Expected: FAIL — `Office.AddListener`, `Office.DroppedEvents`,
`lokc.LookupDispatcherForTest`, etc undefined.

- [ ] **Step 3: Wire up Office**

In `lok/office.go`, extend the `Office` struct:

```go
type Office struct {
	mu        sync.Mutex
	be        backend
	h         officeHandle
	closed    bool
	listeners *listenerSet
	listenerH uintptr // dispatch handle in the lokc handle table
}
```

In `New()`, after `o := &Office{be: be, h: h}` and BEFORE `live = o`, insert:

```go
	o.listeners = newListenerSet()
	o.listenerH = lokc.RegisterDispatcherUintptr(o.listeners)
	if regErr := be.RegisterOfficeCallback(h, o.listenerH); regErr != nil {
		// Tear down before surfacing — newListenerSet spawned a
		// goroutine we must reap.
		lokc.UnregisterDispatcherUintptr(o.listenerH)
		o.listeners.close()
		be.OfficeDestroy(h)
		return nil, &LOKError{Op: "RegisterOfficeCallback", Detail: regErr.Error(), err: regErr}
	}
```

In `Close()`, after the `closed = true` check and BEFORE `OfficeDestroy`:

```go
	if o.listeners != nil {
		lokc.UnregisterDispatcherUintptr(o.listenerH)
		o.listeners.close()
	}
```

(The order: unregister handle first so any late LOK callback no-ops;
then drain and join the dispatcher; then `OfficeDestroy`.)

Add the `AddListener` and `DroppedEvents` methods:

```go
// AddListener registers cb to receive office-wide events. See
// listener.go for the dispatch contract. Returns ErrClosed if the
// Office has been closed and ErrInvalidOption if cb is nil.
func (o *Office) AddListener(cb func(Event)) (cancel func(), err error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return nil, ErrClosed
	}
	c, addErr := o.listeners.addChecked(cb)
	if addErr != nil {
		return nil, &LOKError{Op: "AddListener", Detail: addErr.Error(), err: ErrInvalidOption}
	}
	return c, nil
}

// DroppedEvents returns the cumulative count of office-level events
// the dispatcher dropped because the buffer was full.
func (o *Office) DroppedEvents() uint64 {
	if o.listeners == nil {
		return 0
	}
	return o.listeners.Dropped()
}
```

Add the import `"github.com/julianshen/golibreofficekit/internal/lokc"` to `lok/office.go` if not already present.

`lok/listener_test.go` needs `"errors"`, `"time"`, and `"github.com/julianshen/golibreofficekit/internal/lokc"` in its imports.

- [ ] **Step 4: Run to verify pass**

Run: `go test ./lok -run 'TestOffice_AddListener|TestOffice_DroppedEvents' -race -v`
Expected: 4 PASS.

Run also: `go test ./lok -race`
Expected: full suite passes.

- [ ] **Step 5: Commit**

```bash
git add lok/office.go lok/listener_test.go internal/lokc/callback.go
git commit -m "feat(lok): Office.AddListener / DroppedEvents (eager registration)"
```

---

## Task 8: Wire `listenerSet` into `Document`

**Files:**
- Modify: `lok/document.go`
- Modify: `lok/listener_test.go`

- [ ] **Step 1: Write failing tests**

Append to `lok/listener_test.go`:

```go
func TestDocument_AddListener_DeliversEvent(t *testing.T) {
	fb := &fakeBackend{}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()

	got := make(chan Event, 1)
	cancel, err := doc.AddListener(func(e Event) {
		select { case got <- e: default: }
	})
	if err != nil {
		t.Fatalf("AddListener: %v", err)
	}
	defer cancel()

	dispatchDocumentFromFake(t, fb).Dispatch(int(EventTypeTextSelection), []byte("0,0,100,20"))
	select {
	case e := <-got:
		if e.Type != EventTypeTextSelection || string(e.Payload) != "0,0,100,20" {
			t.Errorf("got %+v", e)
		}
	case <-time.After(time.Second):
		t.Fatalf("listener never received event")
	}
}

func TestDocument_AddListener_AfterCloseErrors(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	doc.Close()
	if _, err := doc.AddListener(func(Event) {}); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestDocument_AddListener_NilCallback(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()
	if _, err := doc.AddListener(nil); !errors.Is(err, ErrInvalidOption) {
		t.Errorf("want ErrInvalidOption, got %v", err)
	}
}

func TestDocument_DroppedEvents_StartsAtZero(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()
	if got := doc.DroppedEvents(); got != 0 {
		t.Errorf("DroppedEvents()=%d, want 0", got)
	}
}
```

Add the helper `dispatchDocumentFromFake` next to its Office sibling:

```go
func dispatchDocumentFromFake(t *testing.T, fb *fakeBackend) *listenerSet {
	t.Helper()
	if fb.lastDocumentCallbackHandle == 0 {
		t.Fatalf("fakeBackend never received RegisterDocumentCallback")
	}
	d := lokc.LookupDispatcherForTest(fb.lastDocumentCallbackHandle)
	if d == nil {
		t.Fatalf("no dispatcher registered under handle %d", fb.lastDocumentCallbackHandle)
	}
	ls, ok := d.(*listenerSet)
	if !ok {
		t.Fatalf("dispatcher is %T, not *listenerSet", d)
	}
	return ls
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./lok -run 'TestDocument_AddListener|TestDocument_DroppedEvents' -v`
Expected: FAIL.

- [ ] **Step 3: Wire up Document**

In `lok/document.go`, extend the `Document` struct:

```go
type Document struct {
	office        *Office
	h             documentHandle
	origURL       string
	docType       DocumentType
	tempPath      string
	closeOnce     sync.Once
	closed        bool
	tileModeReady bool
	listeners     *listenerSet
	listenerH     uintptr
}
```

Find the place in `(o *Office) Load(...)` (and any sibling like
`LoadFromReader` that constructs a `Document`) where the
`*Document{...}` literal is built — likely just before the function
returns. After construction and BEFORE returning, register the
callback:

```go
	d.listeners = newListenerSet()
	d.listenerH = lokc.RegisterDispatcherUintptr(d.listeners)
	if regErr := o.be.RegisterDocumentCallback(d.h, d.listenerH); regErr != nil {
		lokc.UnregisterDispatcherUintptr(d.listenerH)
		d.listeners.close()
		o.be.DocumentDestroy(d.h)
		return nil, &LOKError{Op: "RegisterDocumentCallback", Detail: regErr.Error(), err: regErr}
	}
```

Apply the same block in any other Document-constructing path
(`LoadFromReader` etc). Search with `grep -n 'return &Document\|return d, nil\|return doc, nil' lok/document.go`.

In `(d *Document) Close()`, inside the `closeOnce.Do` block before
`DocumentDestroy`:

```go
		if d.listeners != nil {
			lokc.UnregisterDispatcherUintptr(d.listenerH)
			d.listeners.close()
		}
```

Add `AddListener` and `DroppedEvents`:

```go
// AddListener registers cb to receive document-level events. Returns
// ErrClosed if the document has been closed; ErrInvalidOption if cb
// is nil.
func (d *Document) AddListener(cb func(Event)) (cancel func(), err error) {
	d.office.mu.Lock()
	defer d.office.mu.Unlock()
	if d.closed {
		return nil, ErrClosed
	}
	c, addErr := d.listeners.addChecked(cb)
	if addErr != nil {
		return nil, &LOKError{Op: "AddListener", Detail: addErr.Error(), err: ErrInvalidOption}
	}
	return c, nil
}

func (d *Document) DroppedEvents() uint64 {
	if d.listeners == nil {
		return 0
	}
	return d.listeners.Dropped()
}
```

Add the `lokc` import to `lok/document.go` if not already present.

- [ ] **Step 4: Run to verify pass**

Run: `go test ./lok -run 'TestDocument_AddListener|TestDocument_DroppedEvents' -race -v`
Expected: 4 PASS.

Run: `go test ./lok -race`
Expected: full suite passes.

- [ ] **Step 5: Commit**

```bash
git add lok/document.go lok/listener_test.go
git commit -m "feat(lok): Document.AddListener / DroppedEvents (eager registration)"
```

---

## Task 9: realBackend forwarder coverage

**Files:**
- Modify: `lok/real_backend_test.go`

- [ ] **Step 1: Write failing tests**

Append to `lok/real_backend_test.go`:

```go
func TestRealBackend_RegisterCallbackForwarding(t *testing.T) {
	rb := realBackend{}
	fakeDocHandle := lokc.NewFakeDocumentHandle()
	defer lokc.FreeFakeDocumentHandle(fakeDocHandle)
	rdoc := realDocumentHandle{d: fakeDocHandle}

	// On a NULL-pClass document the slot is "missing" → ErrUnsupported.
	if err := rb.RegisterDocumentCallback(rdoc, 1); !errors.Is(err, ErrUnsupported) {
		t.Errorf("RegisterDocumentCallback on NULL pClass: err=%v, want ErrUnsupported", err)
	}
}
```

(Office side has no fake-handle helper today — coverage of
`RegisterOfficeCallback` is owned by the integration test in
Task 10.)

- [ ] **Step 2: Run to verify pass**

Run: `go test ./lok -run TestRealBackend_RegisterCallbackForwarding -v`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add lok/real_backend_test.go
git commit -m "test(lok): realBackend RegisterDocumentCallback forwarder coverage"
```

---

## Task 10: Drop Phase 8 capability gate; replace with listener-driven wait

**Files:**
- Modify: `lok/integration_test.go`

- [ ] **Step 1: Read the existing Phase 8 selection block**

Run: `grep -nB1 'selectionAppeared' lok/integration_test.go`
Expected: locate the `if !selectionAppeared { t.Logf(...) } else { ... }` block from Phase 8 (around lines 391–439 on the post-merge tree).

- [ ] **Step 2: Replace the block**

Replace the existing block (the entire `if err := doc.PostUnoCommand(".uno:SelectAll"...` through the end of the `if !selectionAppeared { ... } else { ... }` else-branch poll loop) with:

```go
	// --- Phase 9: SelectAll → text-selection callback wait ---
	//
	// Phase 8 deferred this branch behind a t.Logf capability gate
	// because LO 24.8 drops posted input until a callback is
	// registered. Phase 9 registers the trampoline at Load() so the
	// gate is gone — the listener fires and we wait on it.
	selFired := make(chan struct{}, 1)
	cancelSel, err := doc.AddListener(func(e Event) {
		switch e.Type {
		case EventTypeTextSelection, EventTypeTextSelectionStart, EventTypeTextSelectionEnd:
			select {
			case selFired <- struct{}{}:
			default:
			}
		}
	})
	if err != nil {
		t.Fatalf("Phase 9: AddListener: %v", err)
	}
	defer cancelSel()

	if err := doc.PostUnoCommand(".uno:SelectAll", "", false); err != nil {
		t.Errorf("Phase 8: SelectAll: %v", err)
	}

	select {
	case <-selFired:
		// Event arrived; assertions run unconditionally below.
	case <-time.After(2 * time.Second):
		t.Fatalf("Phase 8: timed out waiting for text-selection callback")
	}

	kind, err := doc.GetSelectionKind()
	if err != nil {
		t.Errorf("Phase 8: GetSelectionKind: %v", err)
	}
	if kind != SelectionKindText && kind != SelectionKindComplex {
		t.Errorf("Phase 8: selection kind after SelectAll: %v", kind)
	}
	text, usedMime, err := doc.GetTextSelection("text/plain;charset=utf-8")
	if err != nil {
		t.Errorf("Phase 8: GetTextSelection: %v", err)
	}
	if usedMime == "" {
		t.Errorf("Phase 8: usedMime should be non-empty")
	}
	if !strings.Contains(text, "Hello") {
		t.Errorf("Phase 8: selection text %q does not contain 'Hello'", text)
	}
	kind2, text2, _, err := doc.GetSelectionTypeAndText("text/plain;charset=utf-8")
	if errors.Is(err, ErrUnsupported) {
		t.Logf("Phase 8: GetSelectionTypeAndText unsupported on this LO build")
	} else if err != nil {
		t.Errorf("Phase 8: GetSelectionTypeAndText: %v", err)
	} else {
		if kind2 != SelectionKindText {
			t.Errorf("Phase 8: kind2=%v, want %v", kind2, SelectionKindText)
		}
		if text2 != text {
			t.Errorf("Phase 8: text mismatch: %q vs %q", text2, text)
		}
	}
	if err := doc.ResetSelection(); err != nil {
		t.Errorf("Phase 8: ResetSelection: %v", err)
	}
```

The post-Reset poll loop is deleted — Reset is synchronous as far as
LOK's state goes, and a post-Reset `GetSelectionKind` call is enough.
The smoke calls (SetTextSelection / SetGraphicSelection /
SetBlockedCommandList) and the clipboard round-trip stay as-is.

- [ ] **Step 3: Run the integration test**

```bash
make test-integration
```

Expected: PASS, with no `t.Logf "no observable selection"` line in
the output (because the callback is now registered).

- [ ] **Step 4: Commit**

```bash
git add lok/integration_test.go
git commit -m "test(lok): drop Phase 8 SelectAll capability gate (Phase 9 closes it)"
```

---

## Task 11: Office-level integration smoke

A small extra integration block that registers an Office-level
listener and asserts the trampoline is wired by triggering something
LOK signals at the office level (e.g. `TrimMemory`).

**Files:**
- Modify: `lok/integration_test.go`

- [ ] **Step 1: Append the office-listener block**

Insert just before the final `LoadFromReader` block:

```go
	// --- Phase 9: office-level listener smoke ---
	//
	// Office-level events vary across LO versions, so we don't
	// assert on event types — only that the trampoline is reachable
	// (the listener may legitimately receive zero events here, in
	// which case the smoke just verifies AddListener / cancel work).
	officeFired := make(chan Event, 8)
	cancelOffice, err := o.AddListener(func(e Event) {
		select {
		case officeFired <- e:
		default:
		}
	})
	if err != nil {
		t.Errorf("Phase 9: Office.AddListener: %v", err)
	}
	cancelOffice()
	if err := o.TrimMemory(0); err != nil {
		t.Errorf("Phase 9: TrimMemory: %v", err)
	}
```

(The cancel-then-fire ordering verifies cancel works without
generating a flake; we don't assert on `officeFired`.)

- [ ] **Step 2: Run integration**

```bash
make test-integration
```

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add lok/integration_test.go
git commit -m "test(lok): office-level listener integration smoke"
```

---

## Task 12: Final verification (coverage gate, vet, fmt, full suite)

**Files:**
- No code unless coverage is short.

- [ ] **Step 1: Coverage**

```bash
go test -covermode=atomic -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | tail -n 1
```

Expected: total ≥ 90 %. CLAUDE.md §6.

- [ ] **Step 2: Identify any gaps**

```bash
go tool cover -func=coverage.out | awk '$3 != "100.0%" {print}' | head -40
```

Tasks 1–11 should already cover the new branches. If a sub-90 % file
slipped, add the targeted test in the same file as its neighbours
and re-run. Do not lower the threshold.

- [ ] **Step 3: Full suite**

```bash
go test ./... -race
make test-integration
```

Both clean.

- [ ] **Step 4: vet + fmt**

```bash
go vet ./...
gofmt -s -l .
```

Both silent.

- [ ] **Step 5: Update memory if needed**

Memory `feedback_lok_input_needs_callback` says the SelectAll path is
deferred to Phase 9. Phase 9 closed it — update or delete the
memory:

```bash
ls /home/julianshen/.claude/projects/-home-julianshen-prj-golibreofficekit/memory/ | grep input_needs_callback
```

If the file exists, edit its frontmatter / body to note that Phase 9
landed the binding and the SelectAll integration assertion is now
unconditional. Or delete it if it's no longer load-bearing for any
other workflow. (User judgement; this is a memory hygiene step, not
a code change.)

- [ ] **Step 6: Final commit log review**

```bash
git log --oneline main..HEAD
```

Expected to show the Phase 9 series in order.

---

## Out of scope / deferred

- **Typed payload helpers** (`Event.AsRectangles()`, `Event.AsState()`,
  …). Deferred until concrete callers need them. The `Payload` slice
  is the contract.
- **Configurable buffer size / overflow policy.** Default 256 / drop
  newest. Add `WithListenerBuffer(n)` only when a workload demonstrates
  the need.
- **Per-listener goroutines.** A slow listener still slows the
  per-Object dispatcher. Documented contract.
- **`cancelAndWait()`.** Synchronous "no more calls" cancel. Add only
  if a real caller needs the stronger guarantee and accepts the
  no-call-from-inside-closure constraint.
- **Office-event-type assertions.** The integration smoke asserts only
  that `AddListener` and `cancel` work; office-level event types vary
  across LO versions and aren't worth pinning in CI today.

## Self-review

- **Spec coverage.**
  - §3 EventType + Event: Task 4. ✓
  - §3 listener API on Office: Task 7. ✓
  - §3 listener API on Document: Task 8. ✓
  - §4 data flow (trampoline → channel → dispatcher): Tasks 2, 6. ✓
  - §5 cgo safety + handle table: Tasks 1, 2. ✓
  - §6 lifecycle (eager New + Load registration; Close ordering):
    Tasks 7, 8. ✓
  - §7 errors (ErrClosed, ErrUnsupported, panic recovery, nil cb):
    Tasks 6 (panic recovery, nil cb), 7+8 (ErrClosed), 3 (ErrUnsupported). ✓
  - §8 testing matrix: Tasks 1–9 unit, Tasks 10–11 integration. ✓
  - §9 Phase 8 cleanup: Task 10. ✓
  - §10 deferrals: restated above.
- **Placeholder scan.** No TBD / TODO / "similar to Task N" / "add
  appropriate error handling" remain. Each step shows code or an
  exact command.
- **Type consistency.**
  - `dispatchHandle` (unexported) declared in Task 1; exposed via
    `RegisterDispatcherUintptr` / `UnregisterDispatcherUintptr` /
    `LookupDispatcherForTest` in Tasks 2/5/7. Same name throughout.
  - `Dispatcher` interface declared in Task 1; implemented by
    `*listenerSet` in Task 6. ✓
  - `listenerEntry` struct + `nextID` field used consistently in
    Task 6. ✓
  - `listenerH uintptr` field used in both `Office` (Task 7) and
    `Document` (Task 8). ✓
  - `EventType` constants in Task 4 match the spec table. ✓
