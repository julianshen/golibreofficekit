# Phase 10 — Command values & window events (design)

**Branch:** `feat/command-values`
**Status:** draft (ready for implementation)
**Predecessor:** Phase 9 (callbacks & listeners, merged in PR #20)

## 1. Goals

Bind LibreOfficeKit's command-value query API and window-level APIs so Go
programs can:

1. Query the set of available commands and their current state/values for a
document (`.uno:Save`, `.uno:Bold`, `.uno:InsertTable`, etc.).
2. Receive and respond to window-level events (dialogs, popups, sidebars,
content controls, form fields).
3. Paint into separate window surfaces (not just the main document tile).
4. Dispatch extended text input events to windows.

This phase **does not** implement macro execution, digital signing, or
certificate management — those are deferred to Phase 11 (`lok_advanced`).

## 2. Architecture

Same four-layer pattern as previous phases.

```text
lok (public)           — Document.GetCommandValues, Document.CompleteFunction,
                         Document.PostWindow*Event, Document.PaintWindow*,
                         Document.Send*Event
  └─ backend seam      — GetCommandValues, SendDialogEvent, SendContentControlEvent,
                         SendFormFieldEvent, CompleteFunction,
                         PostWindowKeyEvent, PostWindowMouseEvent,
                         PostWindowGestureEvent, PostWindowExtTextInputEvent,
                         ResizeWindow, PaintWindow, PaintWindowDPI,
                         PaintWindowForView, ResetWindow, GetFontSubset
      └─ realBackend (lok)  — C wrappers in internal/lokc
          └─ internal/lokc  — C shims for LOK document/window functions
              └─ LOK C ABI
```

Files added:

- `lok/commands.go` — `GetCommandValues`, `CompleteFunction`, typed helpers for
  common commands.
- `lok/windows.go` — window event and paint APIs.
- `lok/forms.go` — dialog/content-control/form-field event helpers.
- `internal/lokc/commands.c` / `.h` — C shims for command-value and window
  functions (kept separate from core callback code).
- `internal/lokc/windows.c` / `.h` — C shims for window paint/resize/events.
- `lok/commands_test.go`, `lok/windows_test.go`, `lok/forms_test.go` — unit
  tests via `fakeBackend`.
- `cmd/lok-render/` and `cmd/lok-convert/` examples (Phase 12) will consume
  these new APIs.

Files modified:

- `lok/backend.go` — add new interface methods for command/window operations.
- `lok/real_backend.go` — forwarders for the new methods.
- `lok/document.go` — no structural changes; new methods attached to `Document`.
- `lok/integration_test.go` — add integration smoke for command values and
  window paint round-trips.

## 3. Public API

### 3.1 Command values

LOK provides `getCommandValues(command)` which returns a JSON string describing
the current state/values for a command (e.g. `.uno:FontName` → list of available
fonts; `.uno:StyleApply` → style list; `.uno:CharFontName` → current font).

```go
// GetCommandValues returns a JSON document describing the current
// state/possible values for command. The returned JSON is specific to
// the command; see LibreOfficeKitEnums.h for command names and
// expected payload formats.
//
// Common commands:
//   ".uno:Save"                     — always enabled when document is modifiable
//   ".uno:Undo" / ".uno:Redo"       — enabled/disabled state
//   ".uno:Bold" / ".uno:Italic"     — checked state
//   ".uno:FontName"                 — list of available fonts
//   ".uno:StyleApply"               — list of styles
//   ".uno:CharFontName"             — current font
//
// Returns ErrUnsupported if LOK does not implement getCommandValues for this
// build. Returns a non-nil error for invalid commands or closed documents.
func (*Document) GetCommandValues(command string) (json.RawMessage, error)

// CompleteFunction attempts to complete a function (formula) in a spreadsheet.
// part is the part index (sheet), name is the function name. Returns an error
// if the document is not a spreadsheet or the function cannot be completed.
func (*Document) CompleteFunction(part int, name string) error
```

Typed helpers (convenience):

```go
// IsCommandEnabled returns whether command is currently enabled.
func (d *Document) IsCommandEnabled(cmd string) (bool, error)

// GetFontNames returns the list of available font names.
func (d *Document) GetFontNames() ([]string, error)

// GetStyleNames returns the list of available style names.
func (d *Document) GetStyleNames() ([]string, error)
```

### 3.2 Dialog / content-control / form-field events

LOK can send and receive events for dialogs, content controls, and form fields.
These are per-document (not per-view) and addressed by window ID or JSON
payload.

```go
// SendDialogEvent sends a dialog event identified by windowID. argsJSON is
// a JSON object whose structure depends on the event type (see LOK docs).
func (*Document) SendDialogEvent(windowID uint64, argsJSON string) error

// SendContentControlEvent sends an event for a content control. argsJSON
// specifies the control and action.
func (*Document) SendContentControlEvent(argsJSON string) error

// SendFormFieldEvent sends an event for a form field. argsJSON specifies
// the field and action.
func (*Document) SendFormFieldEvent(argsJSON string) error
```

### 3.3 Window events and painting

Windows are separate surfaces managed by LOK (sidebars, popups, dialogs).
Each window can be painted independently.

```go
// PostWindowKeyEvent posts a key event to a specific window.
func (*Document) PostWindowKeyEvent(windowID uint64, typ KeyEventType,
    charCode, keyCode int) error

// PostWindowMouseEvent posts a mouse event to a specific window.
func (*Document) PostWindowMouseEvent(windowID uint64, typ MouseEventType,
    x, y int64, count int, buttons MouseButtons, mods Modifiers) error

// PostWindowGestureEvent posts a gesture event (pan/zoom) to a window.
func (*Document) PostWindowGestureEvent(windowID uint64, typ string,
    x, y, offset int64) error

// PostWindowExtTextInputEvent posts extended text input to a window.
func (*Document) PostWindowExtTextInputEvent(windowID uint64, typ int,
    text string) error

// ResizeWindow changes the size of a window.
func (*Document) ResizeWindow(windowID uint64, w, h int) error

// PaintWindow paints a window into the provided buffer. The buffer must
// have length 4*pxW*pxH. Returns premultiplied BGRA (same format as
// PaintTileRaw). Prefer PaintWindowDPI for high-DPI aware rendering.
func (*Document) PaintWindow(windowID uint64, buf []byte, pxW, pxH int) error

// PaintWindowDPI paints a window with a DPI scale factor.
func (*Document) PaintWindowDPI(windowID uint64, buf []byte, pxW, pxH int,
    dpiScale float64) error

// PaintWindowForView paints a window for a specific view ID.
func (*Document) PaintWindowForView(windowID uint64, view ViewID, buf []byte,
    pxW, pxH int, dpiScale float64) error

// ResetWindow resets a window's internal state.
func (*Document) ResetWindow(windowID uint64) error

// GetFontSubset retrieves a subset of a font as a byte slice (SFNT).
func (*Document) GetFontSubset(fontName string) ([]byte, error)
```

### 3.4 Reuse of existing types

- `KeyEventType`, `MouseEventType`, `MouseButtons`, `Modifiers` — already
  defined in `lok/input.go` (Phase 7).
- `ViewID` — already defined in `lok/view.go` (Phase 4).
- `EventTypeWindow` — already in `lok/event.go` (Phase 9) for window events
  delivered via callbacks.

## 4. Data flow

### 4.1 Command values

```text
[Go] doc.GetCommandValues(".uno:FontName")
   │
   ▼
[lok] Document.GetCommandValues
   │
   ▼
[lokc] DocumentGetCommandValues (C wrapper)
   │
   ▼
[LOK C] pClass->getCommandValues(doc, ".uno:FontName")
   │
   ▼
[LOK] returns char* (JSON)
   │
   ▼
[lokc] copy to Go string, free with LOK's freeError
   │
   ▼
[lok] return json.RawMessage (no parsing)
```

The caller receives raw JSON and can unmarshal into typed structs as needed.
No parsing is done inside `lok` — this keeps the binding thin and lets
callers handle version-specific formats.

### 4.2 Window paint

```text
[Go] doc.PaintWindow(id, buf, 200, 200)
   │
   ▼
[lok] Document.PaintWindow
   │
   ▼
[lokc] DocumentPaintWindow (C wrapper)
   │
   ▼
[LOK C] pClass->paintWindow(doc, id, buf, ...)
   │
   ▼
[LOK] renders into buf (premul BGRA)
```

Same pointer-safety rules as `PaintTileRaw`: buffer is pinned for the
single synchronous call, not retained.

### 4.3 Window events

Window events can arrive via two paths:

1. **Callback path** — if LOK sends `EventTypeWindow` through the
   registered callback, it is delivered to Document listeners like any
   other event (Phase 9 infrastructure).
2. **Direct call path** — `PostWindow*Event` and `Send*Event` are
   synchronous calls into LOK that may trigger immediate updates.

## 5. cgo safety

- All window and command functions are synchronous and do not retain Go
  pointers beyond the call.
- `PaintWindow*` follows the same rules as `PaintTileRaw`: buffer is
  pinned only for the duration of the C call.
- `GetFontSubset` returns a newly allocated C buffer that is copied to Go
  and freed with `C.free` (or LOK's `freeError` if used).
- No Go pointers are stored in C or passed as `pData` for these APIs.

## 6. Error handling

- `ErrUnsupported` — returned when LOK's vtable lacks the function (older
  LibreOffice builds).
- `*LOKError` — wraps LOK's `getError()` string for command/window
  operations that fail.
- `ErrClosed` — document has been closed.
- Invalid arguments (e.g. negative width/height) return `ErrInvalidOption`.

## 7. Testing

### 7.1 Unit tests (`lok/commands_test.go`)

- `GetCommandValues` with fake backend: success path, error path,
  unsupported path.
- `CompleteFunction` happy path and error cases.
- Typed helpers (`IsCommandEnabled`, `GetFontNames`, etc.) parse JSON
  correctly.

### 7.2 Unit tests (`lok/windows_test.go`)

- `PostWindowKeyEvent`, `PostWindowMouseEvent` argument validation.
- `PaintWindow` with valid/invalid buffer sizes (panic-free).
- `ResizeWindow` error handling.
- `GetFontSubset` returns non-empty data for known font.

### 7.3 Unit tests (`lok/forms_test.go`)

- `SendDialogEvent`, `SendContentControlEvent`, `SendFormFieldEvent`
  forward correctly to backend.

### 7.4 Integration tests (`lok/integration_test.go`)

- Load a document with a form field; send a form field event; verify no
  crash.
- Create a second view; use `PaintWindowForView` to paint into a buffer;
  assert non-trivial output.
- Query command values for `.uno:Save` and `.uno:Bold`; assert valid JSON.
- Use `CompleteFunction` in a spreadsheet document; verify it returns
  without error.

### 7.5 Coverage

- Target ≥ 90 % for `lok` package (excluding trivial cgo wrappers).
- All new C shims covered by `lok` unit tests via fake backend.
- Integration tests exercise real LOK for window paint and command
  values when `LOK_PATH` is set.

## 8. Implementation order (see Phase 10 plan)

1. Extend `backend` interface and `fakeBackend`.
2. Add C shims in `internal/lokc/commands.c` and `windows.c`.
3. Implement `realBackend` forwarders.
4. Write `lok/commands.go`, `lok/windows.go`, `lok/forms.go`.
5. Write unit tests.
6. Add integration smoke tests.
7. Verify coverage ≥ 90 %.

## 9. Out of scope / deferred

- **High-level typed command helpers** (e.g. `doc.SetBold(true)`). Keep
  binding low-level; add helpers later if users request them.
- **Async command execution**. All commands are synchronous.
- **Window enumeration / discovery**. LOK does not expose a list of
  windows; callers must know window IDs from other sources (e.g. events).
- **Advanced macro/signing**. Phase 11 (`lok_advanced`).
- **Complex JSON parsing inside `lok`**. Return `json.RawMessage`; let
  callers decode.

## 10. Notes on compatibility

- Command names are stable across LO 24.8+ but new commands may appear.
  Unknown commands return an error from LOK; we surface it as `*LOKError`.
- Window IDs are `uint64` as used by LOK. They may be allocated by LOK
  and delivered via `EventTypeWindow` events; store them for later use.
- `PaintWindowDPI` and `PaintWindowForView` may not be available on older
  LOK builds — check for `ErrUnsupported`.
