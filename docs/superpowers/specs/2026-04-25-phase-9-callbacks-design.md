# Phase 9 — Callbacks / listeners (design)

**Branch:** `feat/callbacks`
**Status:** approved in brainstorming 2026-04-25
**Predecessor:** Phase 8 (selection & clipboard, merged in PR #18)

## 1. Goals

Bind LibreOfficeKit's `registerCallback` C ABI on both `Office` and
`Document` so users can subscribe to LOK events from Go.

The public surface is intentionally narrow — one `AddListener`
method per object, returning a cancel closure. Multiple listeners
compose. Events arrive on a Go-side dispatcher goroutine, off LOK's
own thread, so user closures can call back into LOK without
violating LOK's not-free-threaded contract.

As a side effect, this phase **eliminates the `t.Logf` capability
gate** that Phase 8's `TestIntegration_FullLifecycle` selection
block has been carrying: LO 24.8 on Fedora drops posted input
until a document-level callback is registered (memory
`feedback_lok_input_needs_callback`), and Phase 9's eager
trampoline registration unblocks that path.

## 2. Architecture

Same four-layer pattern as Phases 3–8.

```text
lok (public)        — Office.AddListener, Document.AddListener,
                      Event, EventType, DroppedEvents
  └─ backend seam   — RegisterOfficeCallback / RegisterDocumentCallback
      └─ realBackend (lok)  — installs the trampoline + handle
                              table entry against pClass->registerCallback
          └─ internal/lokc  — //export'd dispatch trampolines + handle
                              table + C shims
              └─ LOK C ABI
```

Files added:

- `internal/lokc/callback.go` — cgo C shims `go_office_register_callback`
  and `go_doc_register_callback`; two `//export`'d Go trampolines
  `goLOKDispatchOffice` and `goLOKDispatchDocument`; the handle
  table; the `Dispatcher` interface that `lok` implements.
- `internal/lokc/callback_test.go` — direct trampoline invocation
  with a fake `Dispatcher`; handle table races; nil-handle safety.
- `lok/event.go` — `Event` struct, `EventType` typed int, ~15
  curated `EventType*` constants + `String()`.
- `lok/listener.go` — listener-set machinery shared by Office and
  Document: per-object buffered channel, dispatcher goroutine,
  atomic dropped counter, `AddListener`, `DroppedEvents`. Internal
  `listenerSet` private type used by both.
- `lok/event_test.go`, `lok/listener_test.go` — unit tests via the
  existing `fakeBackend`.

Files modified:

- `lok/backend.go` — two new interface methods:
  `RegisterOfficeCallback(h officeHandle, handle uintptr) error`,
  `RegisterDocumentCallback(d documentHandle, handle uintptr) error`.
- `lok/real_backend.go` — forwarders into `lokc`.
- `lok/office.go` — Office gains a private `*listenerSet`
  initialised at `New()`; `Close()` tears it down.
- `lok/document.go` — Document gains a private `*listenerSet`
  initialised at `Load()`/variants; `Close()` tears it down.
- `lok/integration_test.go` — drop the Phase 8 selection
  capability gate; replace the poll loop with a listener-driven
  wait. Smoke calls already in place.
- `lok/office_test.go` — `fakeBackend` learns
  `RegisterOfficeCallback` / `RegisterDocumentCallback` (record
  the handle; tests can later invoke the trampoline directly to
  fan an event into the dispatcher).

## 3. Public API

### 3.1 Event types

```go
// EventType mirrors LOK_CALLBACK_*. Curated constants cover the
// events real callers want; obscure values still arrive as
// EventType(N). Run `grep LOK_CALLBACK_ third_party/lok/.../LibreOfficeKitEnums.h`
// for the full list.
type EventType int

const (
    EventTypeInvalidateTiles            EventType = 0
    EventTypeInvalidateVisibleCursor    EventType = 1
    EventTypeTextSelection              EventType = 2
    EventTypeTextSelectionStart         EventType = 3
    EventTypeTextSelectionEnd           EventType = 4
    EventTypeCursorVisible              EventType = 5
    EventTypeGraphicSelection           EventType = 6
    EventTypeHyperlinkClicked           EventType = 7
    EventTypeStateChanged               EventType = 8
    EventTypeMousePointer               EventType = 12
    EventTypeUNOCommandResult           EventType = 13
    EventTypeDocumentSizeChanged        EventType = 17
    EventTypeSetPart                    EventType = 18
    EventTypeError                      EventType = 20
    EventTypeWindow                     EventType = 36
)

func (EventType) String() string

// Event is one delivered LOK callback. Payload is a Go-owned copy
// of the C-allocated string LOK provided. Format depends on Type —
// see LibreOfficeKitEnums.h. Empty Payload is common; LOK signals
// many events with no body.
type Event struct {
    Type    EventType
    Payload []byte
}
```

### 3.2 Listener API

```go
// AddListener registers cb to receive office-wide events. Returns
// a cancel closure; calling cancel() removes cb from the dispatch
// set. An invocation already in flight on the dispatcher goroutine
// completes; subsequent dispatcher iterations skip cb. Idempotent.
//
// Listeners are dispatched in the order they were registered, on a
// single per-Office dispatcher goroutine. Do not block long inside
// cb — the dispatcher does not advance until cb returns. Calling
// back into LOK from cb is safe (LOK's own thread is not blocked
// by cb), but the document's mutex still applies.
func (*Office) AddListener(cb func(Event)) (cancel func(), err error)

// DroppedEvents returns the cumulative count of office-level
// events the trampoline could not enqueue because the dispatch
// buffer was full. Atomic; may be polled at any time.
func (*Office) DroppedEvents() uint64

// AddListener mirrors Office.AddListener for document-level events.
// Each Document has its own buffered channel, dispatcher goroutine
// and dropped counter — independent of the Office's.
func (*Document) AddListener(cb func(Event)) (cancel func(), err error)
func (*Document) DroppedEvents() uint64
```

Errors:

- `AddListener` returns `ErrClosed` when the receiver has already
  been closed.
- `cancel()` returns nothing; calling it on a closed Office /
  Document is a no-op.

### 3.3 Defaults

| Setting              | Value                            |
| -------------------- | -------------------------------- |
| Buffer size          | 256 events (per Office, per Doc) |
| Overflow policy      | drop newest, increment counter   |
| Cancel semantics     | best-effort, non-blocking        |
| Trampoline lifecycle | eager (at New / Load)            |

The buffer size and overflow policy are not configurable in this
phase. Add tuning options only if a real workload demonstrates the
need.

## 4. Data flow

One event from LOK to user code:

```text
[LOK render thread]
  pClass->registerCallback fires the trampoline
  │
  ▼
[C ↔ Go boundary] goLOKDispatchDocument(typ, payload, pData)
  • copy payload to Go []byte (C.GoBytes + strlen, "" if NULL)
  • lookup handleTable[uintptr(pData)] → Dispatcher
  • non-blocking send Event{typ, bytes} on dispatcher.ch
    (select default: increment dropped counter)
  • return — LOK thread is free
  │
  ▼
[dispatcher goroutine] target.run()
  • range over target.ch
  • snapshot listener set under target.mu (slice copy)
  • for each cb in snapshot: cb(event)
  │
  ▼
[user closure] runs on the dispatcher goroutine
```

The trampoline does no Go allocations beyond the `C.GoBytes` copy
(bounded by `strlen(payload)`). The send uses `select { case ch <-
e: default: drop }` so LOK's thread is never blocked. No mutex is
acquired on LOK's thread.

The dispatcher snapshots the listener slice under a per-object
`sync.Mutex` before iterating. A listener cancelled between the
snapshot and the call site may still receive the in-flight event —
this is consistent with the cancel contract ("no *new* dispatcher
iterations after cancel; an in-flight invocation completes").

## 5. cgo safety + handle table

The hardest constraint: cgo forbids storing Go pointers in C
storage that outlives the cgo call. LOK keeps `pData` for the life
of the registration, so we cannot pass a Go pointer.

Solution: integer handle table.

```go
// internal/lokc/callback.go
type dispatchHandle uintptr

type Dispatcher interface {
    Dispatch(typ int, payload []byte)
}

var (
    handleMu    sync.RWMutex
    handleNext  atomic.Uintptr  // 0 reserved for "unregistered"
    handleTable = map[dispatchHandle]Dispatcher{}
)

func RegisterDispatcher(d Dispatcher) dispatchHandle {
    h := dispatchHandle(handleNext.Add(1))
    handleMu.Lock()
    handleTable[h] = d
    handleMu.Unlock()
    return h
}

func UnregisterDispatcher(h dispatchHandle) {
    handleMu.Lock()
    delete(handleTable, h)
    handleMu.Unlock()
}
```

The C shim casts the Go-supplied uintptr to `void*`:

```c
static void go_office_register_callback(LibreOfficeKit* p, uintptr_t handle) {
    if (p == NULL || p->pClass == NULL || p->pClass->registerCallback == NULL) return;
    p->pClass->registerCallback(p, (LibreOfficeKitCallback)goLOKDispatchOffice, (void*)handle);
}
```

The trampoline recovers the handle from `pData` and looks up the
Go-side `Dispatcher`:

```go
//export goLOKDispatchOffice
func goLOKDispatchOffice(typ C.int, payload *C.char, pData unsafe.Pointer) {
    h := dispatchHandle(uintptr(pData))
    handleMu.RLock()
    d := handleTable[h]
    handleMu.RUnlock()
    if d == nil {
        return
    }
    var b []byte
    if payload != nil {
        b = C.GoBytes(unsafe.Pointer(payload), C.int(C.strlen(payload)))
    }
    d.Dispatch(int(typ), b)
}
```

`goLOKDispatchDocument` is identical — separate names let us
distinguish in stack traces and reason per-class if we ever need
to differentiate.

The handle is an opaque integer, never a Go pointer. The
`handleTable` is the Go-side mapping; entries are removed on
`Close()` so a late LOK callback after close is a safe no-op (the
lookup returns `nil`).

## 6. Lifecycle

- **`New(installPath)`** creates the Office, then:
  1. Allocates an Office `listenerSet` (channel of capacity 256,
     dispatcher goroutine, atomic counter, mutex-protected
     listener slice).
  2. `RegisterDispatcher(officeListenerSet)` → handle.
  3. `RegisterOfficeCallback(officeHandle, handle)` against the
     LOK C ABI. From this point LOK calls the trampoline.
- **`Office.Load(path)` / variants** create the Document, then:
  1. Allocate Document `listenerSet`.
  2. `RegisterDispatcher` → handle.
  3. `RegisterDocumentCallback(documentHandle, handle)`.
- **`AddListener(cb)`** appends `cb` to the listener slice under
  the `listenerSet` mutex; returns a closure that removes `cb`
  under the same mutex. Idempotent.
- **`Document.Close()`** sets the listenerSet's `closed` flag,
  unregisters the handle from the table, closes the buffered
  channel (causing the dispatcher goroutine to exit after draining
  what it has), waits for the goroutine to exit, then calls
  `documentDestroy`. Order: handle unregistration first so any
  in-flight LOK callback after this point is a safe no-op; then C
  destroy. Already-cancelled listeners' cancel closures are
  idempotent no-ops.
- **`Office.Close()`** mirrors Document.Close for the Office's
  listenerSet, then `officeDestroy`. Active documents are the
  caller's responsibility (matches today's contract).

The order of teardown matters: a future LOK call that arrives
between "C document destroyed" and "handle unregistered" would
look up a `nil` Dispatcher and no-op cleanly. The reverse order
("handle unregistered" then "C document destroyed") is also safe
because the trampoline returns early when the dispatcher is `nil`.
We do handle-unregistration first because it stops the channel
sends; the C destroy then runs against a quiesced object.

## 7. Error handling and invariants

- **`ErrClosed`**: `AddListener` on a closed Office or Document.
- **`ErrUnsupported`**: surfaced when LOK's `registerCallback` slot
  itself is NULL on the loaded build. Should not happen on any
  supported LO version, but we still guard it (matches Phase 8's
  pattern). The C shim returns 0 on NULL slot; the Go wrapper
  returns `ErrUnsupported`.
- **Listener panics**: the dispatcher recovers from a panicking
  listener so a bad callback can't take down the dispatcher
  goroutine. The recovered panic is logged via the standard `log`
  package (`log.Printf("lok: listener panic: %v", r)`); other
  listeners in the same dispatch slice still run.
- **`cb` may not be nil**: `AddListener` returns
  `ErrInvalidOption` for a nil callback.
- **No deadlock from inside cb**: cancel() is non-blocking, so
  calling cancel() from inside the closure is safe. Calling Close
  on the same Document from inside its own listener is **not**
  safe — Close waits for the dispatcher goroutine, which is the
  one running the closure. Document this.

## 8. Testing

### 8.1 Unit tests (`internal/lokc/callback_test.go`)

- `goLOKDispatchOffice` with a registered fake Dispatcher: verifies
  type + payload pass-through.
- Dispatch with an unregistered handle (handle = 0 or a freed
  handle): returns cleanly, no panic.
- Concurrent `RegisterDispatcher` / `UnregisterDispatcher` races
  under `-race`.
- Empty payload (NULL `*C.char`): Dispatch receives `nil` slice.
- Long payload (≥ 64 KiB) round-trips intact.
- `goLOKDispatchDocument` symmetric.

### 8.2 Unit tests (`lok/event_test.go`, `lok/listener_test.go`)

`event_test.go`:
- `EventType.String()` — every named constant + one unknown value.

`listener_test.go` (using the existing `fakeBackend` extended with
`RegisterOfficeCallback` / `RegisterDocumentCallback` recorders):
- `AddListener` happy path: install a listener, simulate a
  trampoline event by calling the dispatcher's `Dispatch` directly
  (the fake hands us a hook), assert the closure ran.
- Multiple listeners: each receives the event in registration
  order.
- `cancel()` removes the listener; subsequent dispatch skips it.
- `cancel()` is idempotent across multiple calls.
- `Office.Close()` cancels all listeners + joins dispatcher
  cleanly. Verify the goroutine exits (e.g. by waiting on a done
  channel that the dispatcher closes on exit).
- `Document.Close()` symmetric.
- `AddListener(nil)` returns `ErrInvalidOption`.
- `AddListener` after `Close()` returns `ErrClosed`.
- Drop-on-full: install a slow listener that takes the dispatcher
  goroutine's attention; saturate the channel; verify
  `DroppedEvents()` increments and other listeners eventually
  receive subsequent events.
- Panicking listener: assert the dispatcher does not exit and
  other listeners still run. Recovered panic is logged (we accept
  log output in the test rather than assert on the message).

### 8.3 Integration test (`lok/integration_test.go`)

Replace the Phase 8 selection capability gate. The new structure:

```go
// Phase 9 closes the Phase 8 SelectAll capability gate.
selFired := make(chan struct{}, 1)
cancelSel, err := doc.AddListener(func(e Event) {
    switch e.Type {
    case EventTypeTextSelection, EventTypeTextSelectionStart, EventTypeTextSelectionEnd:
        // Probe-fire; the actual kind/text comes from
        // GetSelectionKind / GetTextSelection below.
        select { case selFired <- struct{}{}: default: }
    }
})
if err != nil { t.Fatalf("AddListener: %v", err) }
defer cancelSel()

if err := doc.PostUnoCommand(".uno:SelectAll", "", false); err != nil {
    t.Errorf("SelectAll: %v", err)
}

select {
case <-selFired:
    // Event arrived; assertions below run unconditionally.
case <-time.After(2 * time.Second):
    t.Fatalf("timed out waiting for text-selection callback")
}

// All previously-skipped assertions now run.
text, usedMime, err := doc.GetTextSelection("text/plain;charset=utf-8")
// ...
```

The 500 ms poll deadline becomes a 2 s event-driven wait; the
`t.Logf` skip path is removed. If the listener never fires, the
test fails — which is the right signal because Phase 9 is the
phase that's supposed to make this work.

A second integration sub-block exercises office-level events:
register an Office listener, trigger something that LOK signals at
the office level (e.g. setting `OfficeOptionalFeatures` or
trimming memory), and assert at least one event arrives. We do
not assert specific event types because the office event surface
varies across LO versions.

### 8.4 Coverage target

≥ 90 % per package. The trampoline is reachable from
`internal/lokc` unit tests via direct `//export` invocation. The
listener-set logic is reachable from `lok` unit tests via the
fake's `Dispatch` hook. The `realBackend` forwarders are reachable
from `lok/real_backend_test.go` using the existing
`NewFakeDocumentHandle` + NULL-pClass pattern; `RegisterDispatcher`
must succeed and `RegisterOfficeCallback` / `RegisterDocumentCallback`
return cleanly when the slot is NULL.

## 9. Phase 8 cleanup that ships with this phase

- `lok/integration_test.go`: drop the `selectionAppeared`
  capability-gate poll. Replace with the listener-driven wait
  shown in §8.3.
- Memory `feedback_lok_input_needs_callback`: superseded by Phase 9.
  Update or delete after this phase merges.

## 10. Out of scope / deferred

- **Typed event payloads.** LOK's per-type formats (CSV
  rectangles, JSON dialogs, .uno:Cmd=value pairs) stay as raw
  `[]byte`. Add `(Event).AsRectangles()` etc. when concrete callers
  need them.
- **Per-listener buffer / configurable buffer size.** Single per-Office
  / per-Document buffer of 256, drop newest. Add `WithListenerBuffer`
  if a workload shows it's needed.
- **Per-listener goroutine.** A slow listener still slows the
  per-Object dispatcher. Documented contract: don't block long.
  Add per-listener queues only if a real caller needs them.
- **`cancelAndWait()`.** Synchronous "guarantee no more calls"
  cancel is a footgun (deadlocks if called from inside the
  closure). Add only if a caller proves the need and accepts the
  contract.
- **Office event coverage detail.** The integration-test
  office-level sub-block asserts only "at least one event
  arrived", not specific types — those vary across LO versions.
