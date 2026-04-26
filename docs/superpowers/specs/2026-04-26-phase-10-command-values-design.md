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
                         PaintWindowForView, GetFontSubset
      └─ realBackend (lok)  — C wrappers in internal/lokc
          └─ internal/lokc  — C shims for LOK document/window functions
              └─ LOK C ABI
```

Files added:

- `lok/commands.go` — **append** `GetCommandValues`, `CompleteFunction`, typed helpers to existing file.
- `lok/windows.go` — window event and paint APIs.
- `lok/forms.go` — dialog/content-control/form-field event helpers.
- `lok/commands_test.go`, `lok/windows_test.go`, `lok/forms_test.go` — unit tests.
- `internal/lokc/commands.c` / `.h` — C shims for command-value functions.
- `internal/lokc/windows.c` / `.h` — C shims for window paint/resize/events.

Files modified:

- `lok/backend.go` — add new interface methods.
- `lok/real_backend.go` — forwarders for command/window methods.
- `lok/document.go` — attach new methods to `Document`.
- `lok/integration_test.go` — add integration smoke for command values and window paint.
- `internal/lokc/lokc.go` — add `#include` for new headers.

## 3. Public API

### 3.1 Command values

LOK provides `getCommandValues(command)` which returns a JSON string describing
the current state/possible values for a command (e.g. `.uno:FontName` → list of available
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
func (d *Document) GetCommandValues(command string) (json.RawMessage, error)

// CompleteFunction attempts to complete a function (formula) in a spreadsheet.
// name is the function name. Returns an error if the function cannot be completed.
// This is a no-op for non-Calc documents.
func (d *Document) CompleteFunction(name string) error
```

Typed helpers (convenience):

```go
// IsCommandEnabled returns whether command is currently enabled.
// Returns an error if the command JSON cannot be parsed or the "enabled"
// or "state" field is absent/invalid. Absence of these fields is treated
// as disabled (false, nil) to distinguish from parse errors.
func (d *Document) IsCommandEnabled(cmd string) (bool, error)

// GetFontNames returns the list of available font names.
// Returns an error if the command JSON cannot be parsed or the "value"
// field is not a list. Returns empty slice (not nil) if no fonts found.
func (d *Document) GetFontNames() ([]string, error)
```

### 3.2 Dialog / content-control / form-field events

LOK can send and receive events for dialogs, content controls, and form fields.
These are per-document (not per-view) and addressed by window ID or JSON
payload.

```go
// SendDialogEvent sends a dialog event identified by windowID. argsJSON is
// a JSON object whose structure depends on the event type (see LOK docs).
// windowID is uint64 to match LOK's sendDialogEvent (unsigned long long).
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
func (*Document) PostWindowKeyEvent(windowID uint32, typ KeyEventType,
    charCode, keyCode int) error

// PostWindowMouseEvent posts a mouse event to a specific window.
func (*Document) PostWindowMouseEvent(windowID uint32, typ MouseEventType,
    x, y int64, count int, buttons MouseButton, mods Modifier) error

// PostWindowGestureEvent posts a gesture event (pan/zoom) to a window.
func (*Document) PostWindowGestureEvent(windowID uint32, typ string,
    x, y, offset int64) error

// PostWindowExtTextInputEvent posts extended text input to a window.
func (*Document) PostWindowExtTextInputEvent(windowID uint32, typ int,
    text string) error

// ResizeWindow changes the size of a window.
func (*Document) ResizeWindow(windowID uint32, w, h int) error

// PaintWindow paints a window into the provided buffer. The buffer must
// have length 4*pxW*pxH. Returns premultiplied BGRA (same format as
// PaintTileRaw). x, y specify the top-left of the source rectangle in
// twips (window coordinates).
func (*Document) PaintWindow(windowID uint32, buf []byte, x, y, pxW, pxH int) error

// PaintWindowDPI paints a window with a DPI scale factor.
func (*Document) PaintWindowDPI(windowID uint32, buf []byte, x, y, pxW, pxH int,
    dpiScale float64) error

// PaintWindowForView paints a window for a specific view ID.
func (*Document) PaintWindowForView(windowID uint32, view ViewID, buf []byte,
    x, y, pxW, pxH int, dpiScale float64) error

// ResetWindow resets a window's internal state.
// NOTE: LOK 24.8 does NOT provide resetWindow. This method is NOT
// implemented and will return ErrUnsupported. Kept in the spec for
// future LOK versions.
func (*Document) ResetWindow(windowID uint32) error

// GetFontSubset retrieves a subset of a font as a byte slice (SFNT).
// NOTE: LOK 24.8 does NOT provide getFontSubset. This method is NOT
// implemented and will return ErrUnsupported. Kept in the spec for
// future LOK versions.
func (*Document) GetFontSubset(fontName string) ([]byte, error)
```

### 3.4 Reuse of existing types

- `KeyEventType`, `MouseEventType`, `MouseButton`, `Modifier` — already
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
[Go] doc.PaintWindow(id, x, y, buf, 200, 200)
   │
   ▼
[lok] Document.PaintWindow
   │
   ▼
[lokc] DocumentPaintWindow (C wrapper)
   │
   ▼
[LOK C] pClass->paintWindow(doc, id, buf, x, y, w, h)
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
- `GetFontSubset` (when available) returns a newly allocated C buffer that
  is copied to Go and freed with `C.free` (or LOK's `freeError`).
- No Go pointers are stored in C or passed as `pData` for these APIs.
- All C strings passed to LOK are freed immediately after the call
  (no leaks).
- Window IDs are `uint32` (LOK uses `unsigned`). `SendDialogEvent` takes
  `uint64` to match LOK's `unsigned long long`.

## 6. Error handling

- `ErrUnsupported` — returned when LOK's vtable lacks the function (older
  LibreOffice builds).
- `*LOKError` — wraps LOK's `getError()` string for command/window
  operations that fail. **Note:** only Office-level `getError` exists;
  document-level errors are reported via callback or as `ErrUnsupported`
  for functions that return no value.
- `ErrClosed` — document has been closed.
- `ErrInvalidOption` — invalid arguments (e.g. negative width/height,
  buffer size mismatch).
- Functions that return `void` (e.g. `PostWindowKeyEvent`) never fail at
  the Go level — they silently no-op if the LOK vtable slot is NULL.

## 7. Testing

### 7.1 Unit tests (`lok/commands_test.go`)

- `GetCommandValues` with fake backend: success path, error path,
  unsupported path.
- `CompleteFunction` happy path and error cases.
- Typed helpers (`IsCommandEnabled`, `GetFontNames`) parse JSON
  correctly.
- Integration with existing `lok/commands.go` tests.

### 7.2 Unit tests (`lok/windows_test.go`)

- `PostWindowKeyEvent`, `PostWindowMouseEvent` argument validation
  (int32 range checks).
- `PaintWindow` with valid/invalid buffer sizes (uses `checkPaintBuf`).
- `ResizeWindow` error handling.
- `GetFontSubset` returns non-empty data for known font (or
  `ErrUnsupported`).

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
- **Reuses the package-level Office** (no `New` in each test).

### 7.5 Coverage

- Target ≥ 90 % for `lok` package (excluding trivial cgo wrappers).
- All new C shims covered by `lok` unit tests via fake backend.
- Integration tests exercise real LOK for window paint and command
  values when `LOK_PATH` is set.

## 8. Implementation order (see Phase 10 plan)

1. Extend `backend` interface and `fakeBackend`.
2. Add C shims in `internal/lokc/commands.c` and `windows.c`.
3. Implement `realBackend` forwarders (delegating to `internal/lokc`).
4. Append to `lok/commands.go`; create `lok/windows.go`, `lok/forms.go`.
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
- `ResetWindow` and `GetFontSubset` — not in LOK 24.8.

## 10. Notes on compatibility

- Command names are stable across LO 24.8+ but new commands may appear.
  Unknown commands return an error from LOK; we surface it as `*LOKError`.
- Window IDs are `uint32` as used by LOK (except `sendDialogEvent`).
  They may be allocated by LOK and delivered via `EventTypeWindow`
  events; store them for later use.
- `PaintWindowDPI` and `PaintWindowForView` may not be available on older
  LOK builds — check for `ErrUnsupported`.
- `CompleteFunction` is only meaningful for Calc documents; it is a
  silent no-op for other types.
