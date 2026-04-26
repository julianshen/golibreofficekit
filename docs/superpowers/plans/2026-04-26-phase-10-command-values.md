# Phase 10 — Command values & window events Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bind LibreOfficeKit's command-value query API (`getCommandValues`, `completeFunction`) and window-level APIs (window events, window paint, dialog/content-control/form-field events) so users can query command states and interact with windows from Go.

**Architecture:** Same four-layer pattern as Phases 3–9. `internal/lokc` owns the C shims for command/window functions; `lok` owns the public API, error mapping, and JSON handling.

**Tech Stack:** Go + cgo on `linux || darwin`, LibreOfficeKit C ABI, `//export` trampolines already in place from Phase 9, `go test` + `lok_integration` build tag.

**Branch:** `feat/command-values` (to be created from `main`)

**Spec:** `docs/superpowers/specs/2026-04-26-phase-10-command-values-design.md`

---

## File Structure

Files created:

- `lok/windows.go` — window event APIs (key/mouse/gesture/text/resize).
- `lok/windows_paint.go` — window paint APIs (`PaintWindow`,
  `PaintWindowDPI`, `PaintWindowForView`); split from `windows.go`
  so the paint contract stays adjacent to the buffer-validation rules
  inherited from `PaintTileRaw`.
- `lok/windows_test.go` — unit tests for window event + paint APIs.
- `lok/forms.go` — dialog/content-control/form-field event helpers.
- `lok/forms_test.go` — unit tests for form events.
- `internal/lokc/commands.go` — Go wrappers around the command-value and
  document-level dialog/content-control/form-field shims.
- `internal/lokc/commands.c` — C shims for `getCommandValues`,
  `completeFunction`, and the three send*Event slots.
- `internal/lokc/commands.h` — declarations for `commands.c`.
- `internal/lokc/windows.go` — Go wrappers around the window event +
  paint shims (incl. `len(buf) == 4*pxW*pxH` validation).
- `internal/lokc/windows.c` — C shims for window paint/resize/events.
- `internal/lokc/windows.h` — declarations for `windows.c`.

Each new `internal/lokc/*.go` file carries its own cgo preamble that
includes the matching `*.h`; there is no shared `internal/lokc/lokc.go`.

Files modified:

- `lok/commands.go` — **append** `GetCommandValues`, `CompleteFunction`,
  and typed helpers (`IsCommandEnabled`, `GetFontNames`) to the existing
  Phase 6 file.
- `lok/commands_test.go` — **append** unit tests for command values.
- `lok/backend.go` — add new interface methods.
- `lok/real_backend.go` — one-line forwarders to `internal/lokc`.
- `lok/office_test.go` — extend `fakeBackend` with Phase 10 tracking
  fields and stub methods.
- `lok/integration_test.go` — extend `TestIntegration_FullLifecycle`
  with command-values and window assertions (no new top-level test that
  would call `New` a second time).

---

## Task 1: Extend backend interface and fakeBackend

**Files:**
- `lok/backend.go`
- `lok/office_test.go` (fakeBackend)

- [ ] **Step 1: Add interface methods**

Append to `backend` interface in `lok/backend.go`:

```go
// Command & window operations (Phase 10).
GetCommandValues(d documentHandle, command string) (string, error)
CompleteFunction(d documentHandle, name string) error
SendDialogEvent(d documentHandle, windowID uint64, argsJSON string) error
SendContentControlEvent(d documentHandle, argsJSON string) error
SendFormFieldEvent(d documentHandle, argsJSON string) error
PostWindowKeyEvent(d documentHandle, windowID uint32, typ int, charCode, keyCode int) error
PostWindowMouseEvent(d documentHandle, windowID uint32, typ int, x, y int64, count int, buttons, mods int) error
PostWindowGestureEvent(d documentHandle, windowID uint32, typ string, x, y, offset int64) error
PostWindowExtTextInputEvent(d documentHandle, windowID uint32, typ int, text string) error
ResizeWindow(d documentHandle, windowID uint32, w, h int) error
PaintWindow(d documentHandle, windowID uint32, buf []byte, x, y, pxW, pxH int) error
PaintWindowDPI(d documentHandle, windowID uint32, buf []byte, x, y, pxW, pxH int, dpiScale float64) error
PaintWindowForView(d documentHandle, windowID uint32, view int, buf []byte, x, y, pxW, pxH int, dpiScale float64) error
// NOTE: ResetWindow and GetFontSubset are NOT in LOK 24.8 — omitted.
// Views are passed as plain `int`/ViewID (matching DocumentSetView et al.);
// no `viewHandle` brand type exists in this binding.
```

- [ ] **Step 2: Extend fakeBackend**

In `lok/office_test.go`, add fields to `fakeBackend`:

```go
// Phase 10: command/window tracking.
lastCommand                string
lastCommandResult          string
getCommandValuesErr        error
completeFunctionErr        error
sendDialogEventErr         error
sendContentControlEventErr error
sendFormFieldEventErr      error
lastWindowID               uint32
lastWindowX, lastWindowY   int
lastWindowBuf              []byte
```

Add stub methods (one per interface method) that record calls and return
configurable errors/results. Example:

```go
func (f *fakeBackend) GetCommandValues(_ documentHandle, cmd string) (string, error) {
    f.lastCommand = cmd
    if f.getCommandValuesErr != nil {
        return "", f.getCommandValuesErr
    }
    return f.lastCommandResult, nil
}

func (f *fakeBackend) CompleteFunction(_ documentHandle, name string) error {
    f.lastCommand = "CompleteFunction:" + name
    return f.completeFunctionErr
}

func (f *fakeBackend) PostWindowKeyEvent(_ documentHandle, windowID uint32, typ, charCode, keyCode int) error {
    f.lastWindowID = windowID
    return nil
}
// ... similar stubs for other window methods ...
```

- [ ] **Step 3: Build to verify compilation**

Run: `go build ./lok/...`
Expected: FAIL — realBackend missing methods.

---

## Task 2: `internal/lokc/commands.{go,c,h}` — command-value & document-event shims

cgo lives strictly inside `internal/lokc`. The `lok` package never imports
`"C"`. This task creates the C shim, the matching header, and the Go
wrapper that exposes plain Go signatures.

**Files:**
- `internal/lokc/commands.h`
- `internal/lokc/commands.c`
- `internal/lokc/commands.go`

- [ ] **Step 1: Create `commands.h`**

```c
#ifndef LOKC_COMMANDS_H
#define LOKC_COMMANDS_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

// getCommandValues / completeFunction
//   Return 1 on success, 0 on failure (NULL handle, missing vtable, NULL
//   payload from LOK). For loke_get_command_values, on success *out_len
//   holds the strlen of the returned buffer (no null terminator), *out
//   points at LOK's heap-allocated string — the caller frees it with
//   free().
int loke_get_command_values(void* doc, const char* command, char** out, size_t* out_len);
int loke_complete_function(void* doc, const char* name);

// Document-level dialog / content-control / form-field events
int loke_doc_send_dialog_event(void* doc, uint64_t window_id, const char* args_json);
int loke_doc_send_content_control_event(void* doc, const char* args_json);
int loke_doc_send_form_field_event(void* doc, const char* args_json);

#ifdef __cplusplus
}
#endif

#endif
```

- [ ] **Step 2: Create `commands.c`**

Bodies follow the existing `internal/lokc` shim pattern:

```c
#include "commands.h"
#include "LibreOfficeKit/LibreOfficeKit.h"
#include <stdlib.h>
#include <string.h>

int loke_get_command_values(void* doc, const char* command, char** out, size_t* out_len) {
    if (!doc || !command || !out || !out_len) return 0;
    LibreOfficeKitDocument* d = (LibreOfficeKitDocument*)doc;
    if (!d->pClass || !d->pClass->getCommandValues) return 0;
    char* s = d->pClass->getCommandValues(d, command);
    if (!s) return 0;
    *out_len = strlen(s);
    *out = s;
    return 1;
}

int loke_complete_function(void* doc, const char* name) {
    if (!doc || !name) return 0;
    LibreOfficeKitDocument* d = (LibreOfficeKitDocument*)doc;
    if (!d->pClass || !d->pClass->completeFunction) return 0;
    d->pClass->completeFunction(d, name);
    return 1;
}

// Three send*Event shims follow the same shape: NULL-check, lookup
// vtable slot, call the void function, return 1.
```

- [ ] **Step 3: Create `internal/lokc/commands.go`**

Each `internal/lokc/*.go` file carries its own cgo preamble that pulls in
the matching header. There is no shared `lokc.go`.

```go
//go:build linux || darwin

package lokc

/*
#cgo CFLAGS: -I${SRCDIR}/../../third_party/lok -DLOK_USE_UNSTABLE_API
#include <stdlib.h>
#include "LibreOfficeKit/LibreOfficeKit.h"
#include "commands.h"
*/
import "C"
import "unsafe"

func DocumentGetCommandValues(d DocumentHandle, command string) (string, error) {
    if !d.IsValid() {
        return "", ErrNilDocument
    }
    cCmd := C.CString(command)
    defer C.free(unsafe.Pointer(cCmd))
    var out *C.char
    var outLen C.size_t
    if C.loke_get_command_values(unsafe.Pointer(d.p), cCmd, &out, &outLen) == 0 {
        return "", ErrUnsupported
    }
    defer C.free(unsafe.Pointer(out))
    return C.GoStringN(out, C.int(outLen)), nil
}

func DocumentCompleteFunction(d DocumentHandle, name string) error { /* C.CString + defer C.free + 0/1 → ErrUnsupported/nil */ }
func DocumentSendDialogEvent(d DocumentHandle, windowID uint64, argsJSON string) error { /* same shape */ }
func DocumentSendContentControlEvent(d DocumentHandle, argsJSON string) error { /* same shape */ }
func DocumentSendFormFieldEvent(d DocumentHandle, argsJSON string) error { /* same shape */ }
```

Every C string allocated for a LOK call is freed via `defer C.free` in
the same function. There are no cgo allocations that escape the
function boundary except the `out` buffer from `getCommandValues`,
which is freed before the function returns.

- [ ] **Step 4: Build**

Run: `go build ./internal/lokc`
Expected: compiles cleanly.

---

## Task 3: `internal/lokc/windows.{go,c,h}` — window event & paint shims

**Files:**
- `internal/lokc/windows.h`
- `internal/lokc/windows.c`
- `internal/lokc/windows.go`

- [ ] **Step 1: Create `windows.h`**

```c
#ifndef LOKC_WINDOWS_H
#define LOKC_WINDOWS_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

// Window events — all return 1 on success, 0 on NULL handle / missing vtable.
int loke_post_window_key_event(void* doc, uint32_t window_id, int type, int char_code, int key_code);
int loke_post_window_mouse_event(void* doc, uint32_t window_id, int type, int64_t x, int64_t y, int count, int buttons, int mods);
int loke_post_window_gesture_event(void* doc, uint32_t window_id, const char* typ, int64_t x, int64_t y, int64_t offset);
int loke_post_window_ext_text_input_event(void* doc, uint32_t window_id, int typ, const char* text);
int loke_resize_window(void* doc, uint32_t window_id, int w, int h);

// Window paint — x, y are top-left of source rect in twips. Buffer must
// have len == 4*pxW*pxH (validated in Go layer); LOK fills with premul BGRA.
int loke_paint_window(void* doc, uint32_t window_id, void* buf, int x, int y, int px_w, int px_h);
int loke_paint_window_dpi(void* doc, uint32_t window_id, void* buf, int x, int y, int px_w, int px_h, double dpiscale);
int loke_paint_window_for_view(void* doc, uint32_t window_id, int view_id, void* buf, int x, int y, int px_w, int px_h, double dpiscale);

#ifdef __cplusplus
}
#endif

#endif
```

`resetWindow` and `getFontSubset` slots do not exist in LOK 24.8; no
shim is added for either. Revisit if/when the C ABI exposes them.

- [ ] **Step 2: Create `windows.c`**

Each shim NULL-checks `doc` (and any string arg), looks up the vtable
slot, and forwards. Since every LOK window function returns `void`, the
shim returns `1` once the call completes:

```c
int loke_post_window_key_event(void* doc, uint32_t window_id, int type, int char_code, int key_code) {
    if (!doc) return 0;
    LibreOfficeKitDocument* d = (LibreOfficeKitDocument*)doc;
    if (!d->pClass || !d->pClass->postWindowKeyEvent) return 0;
    d->pClass->postWindowKeyEvent(d, window_id, type, char_code, key_code);
    return 1;
}
// remaining shims follow the same shape
```

The paint shims cast `void* buf` to `unsigned char*` before invoking
LOK; the buffer is pinned by the Go runtime for the synchronous call.

- [ ] **Step 3: Create `internal/lokc/windows.go`**

```go
//go:build linux || darwin

package lokc

/*
#cgo CFLAGS: -I${SRCDIR}/../../third_party/lok -DLOK_USE_UNSTABLE_API
#include <stdlib.h>
#include "LibreOfficeKit/LibreOfficeKit.h"
#include "windows.h"
*/
import "C"
import (
    "errors"
    "unsafe"
)

func DocumentPostWindowKeyEvent(d DocumentHandle, windowID uint32, typ, charCode, keyCode int) error { /* 0 → ErrUnsupported */ }
func DocumentPostWindowMouseEvent(d DocumentHandle, windowID uint32, typ int, x, y int64, count int, buttons, mods int) error { /* same */ }
// ... gesture, ext-text-input, resize
func DocumentPaintWindow(d DocumentHandle, windowID uint32, buf []byte, x, y, pxW, pxH int) error {
    if !d.IsValid() {
        return ErrNilDocument
    }
    if len(buf) != 4*pxW*pxH {
        return errors.New("lokc: buffer size mismatch")
    }
    if C.loke_paint_window(unsafe.Pointer(d.p), C.uint32_t(windowID), unsafe.Pointer(&buf[0]),
        C.int(x), C.int(y), C.int(pxW), C.int(pxH)) == 0 {
        return ErrUnsupported
    }
    return nil
}
// PaintWindowDPI / PaintWindowForView mirror the same shape with their extra args.
```

The Go layer is the right place for `len(buf) != 4*pxW*pxH` validation —
it keeps `unsafe.Pointer(&buf[0])` from indexing into a too-small slice.

- [ ] **Step 4: Build**

Run: `go build ./internal/lokc`
Expected: compiles cleanly.

---

## Task 4: realBackend forwarders

`lok/real_backend.go` is **not** a cgo file. Each forwarder is a
one-line delegate to the corresponding `lokc.Document*` Go wrapper from
Task 2/3.

**Files:**
- `lok/real_backend.go`

- [ ] **Step 1: Add forwarders before `var _ backend = realBackend{}`**

```go
// --- Command & window operations (Phase 10) ---

func (realBackend) GetCommandValues(d documentHandle, command string) (string, error) {
    return lokc.DocumentGetCommandValues(mustDoc(d).d, command)
}

func (realBackend) CompleteFunction(d documentHandle, name string) error {
    return lokc.DocumentCompleteFunction(mustDoc(d).d, name)
}

func (realBackend) SendDialogEvent(d documentHandle, windowID uint64, argsJSON string) error {
    return lokc.DocumentSendDialogEvent(mustDoc(d).d, windowID, argsJSON)
}

func (realBackend) SendContentControlEvent(d documentHandle, argsJSON string) error {
    return lokc.DocumentSendContentControlEvent(mustDoc(d).d, argsJSON)
}

func (realBackend) SendFormFieldEvent(d documentHandle, argsJSON string) error {
    return lokc.DocumentSendFormFieldEvent(mustDoc(d).d, argsJSON)
}

func (realBackend) PostWindowKeyEvent(d documentHandle, windowID uint32, typ, charCode, keyCode int) error {
    return lokc.DocumentPostWindowKeyEvent(mustDoc(d).d, windowID, typ, charCode, keyCode)
}

func (realBackend) PostWindowMouseEvent(d documentHandle, windowID uint32, typ int, x, y int64, count int, buttons, mods int) error {
    return lokc.DocumentPostWindowMouseEvent(mustDoc(d).d, windowID, typ, x, y, count, buttons, mods)
}

// PostWindowGestureEvent, PostWindowExtTextInputEvent, ResizeWindow,
// PaintWindow, PaintWindowDPI follow the same one-liner pattern.

func (realBackend) PaintWindowForView(d documentHandle, windowID uint32, view int, buf []byte, x, y, pxW, pxH int, dpiScale float64) error {
    return lokc.DocumentPaintWindowForView(mustDoc(d).d, windowID, view, buf, x, y, pxW, pxH, dpiScale)
}
```

No `import "C"`, no `unsafe`, no `C.CString`, no `defer C.free` — all of
that lives in `internal/lokc` (Task 2/3). The `lok` package's only new
import is the existing `lokc` import.

- [ ] **Step 2: Build**

Run: `go build ./...`
Expected: compiles cleanly.

---

## Task 5: Public API — append to `lok/commands.go`

**Files:**
- `lok/commands.go` (append)
- `lok/commands_test.go` (append)

- [ ] **Step 1: Append `GetCommandValues` and `CompleteFunction`**

Add to end of `lok/commands.go`:

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
func (d *Document) GetCommandValues(command string) (json.RawMessage, error) {
    unlock, err := d.guard()
    if err != nil {
        return nil, err
    }
    defer unlock()
    s, err := d.office.be.GetCommandValues(d.h, command)
    if err != nil {
        return nil, err
    }
    return json.RawMessage(s), nil
}

// CompleteFunction attempts to complete a function (formula) in a spreadsheet.
// part is the part index (sheet), name is the function name. Returns an error
// if the document is not a spreadsheet or the function cannot be completed.
// This is a no-op for non-Calc documents.
func (d *Document) CompleteFunction(part int, name string) error {
    unlock, err := d.guard()
    if err != nil {
        return err
    }
    defer unlock()
    return d.office.be.CompleteFunction(d.h, part, name)
}

// IsCommandEnabled returns whether command is currently enabled.
// Returns an error if the command JSON cannot be parsed. If the "enabled"
// or "state" field is absent, returns false with no error.
func (d *Document) IsCommandEnabled(cmd string) (bool, error) {
    raw, err := d.GetCommandValues(cmd)
    if err != nil {
        return false, err
    }
    var m map[string]interface{}
    if err := json.Unmarshal(raw, &m); err != nil {
        return false, err
    }
    if v, ok := m["enabled"].(bool); ok {
        return v, nil
    }
    if v, ok := m["state"].(bool); ok {
        return v, nil
    }
    return false, nil
}

// GetFontNames returns the list of available font names.
// Returns an error if the command JSON cannot be parsed. If the "value"
// field is absent or not a list, returns an empty slice.
func (d *Document) GetFontNames() ([]string, error) {
    raw, err := d.GetCommandValues(".uno:FontName")
    if err != nil {
        return nil, err
    }
    var m map[string]interface{}
    if err := json.Unmarshal(raw, &m); err != nil {
        return nil, err
    }
    if v, ok := m["value"].([]interface{}); ok {
        names := make([]string, len(v))
        for i, x := range v {
            names[i] = fmt.Sprint(x)
        }
        return names, nil
    }
    return nil, nil
}
```

Don't forget to add imports: `"encoding/json"`, `"fmt"`.

- [ ] **Step 2: Append tests to `lok/commands_test.go`**

Add to end of file:

```go
func TestGetCommandValues(t *testing.T) {
    fb := &fakeBackend{}
    withFakeBackend(t, fb)
    o, _ := New("/install")
    defer o.Close()
    doc, _ := o.Load("/tmp/x.odt")
    defer doc.Close()

    fb.getCommandValuesResult = `{"enabled":true}`
    raw, err := doc.GetCommandValues(".uno:Save")
    if err != nil {
        t.Fatalf("GetCommandValues: %v", err)
    }
    if string(raw) != `{"enabled":true}` {
        t.Errorf("got %s", raw)
    }
    if fb.lastCommand != ".uno:Save" {
        t.Errorf("lastCommand=%s", fb.lastCommand)
    }
}

func TestGetCommandValues_Closed(t *testing.T) {
    withFakeBackend(t, &fakeBackend{})
    o, _ := New("/install")
    defer o.Close()
    doc, _ := o.Load("/tmp/x.odt")
    doc.Close()
    _, err := doc.GetCommandValues(".uno:Save")
    if !errors.Is(err, ErrClosed) {
        t.Errorf("want ErrClosed, got %v", err)
    }
}

func TestIsCommandEnabled(t *testing.T) {
    fb := &fakeBackend{}
    withFakeBackend(t, fb)
    o, _ := New("/install")
    defer o.Close()
    doc, _ := o.Load("/tmp/x.odt")
    defer doc.Close()

    fb.getCommandValuesResult = `{"enabled":true}`
    ok, err := doc.IsCommandEnabled(".uno:Bold")
    if err != nil {
        t.Fatal(err)
    }
    if !ok {
        t.Error("expected true")
    }
}

func TestGetFontNames(t *testing.T) {
    fb := &fakeBackend{}
    withFakeBackend(t, fb)
    o, _ := New("/install")
    defer o.Close()
    doc, _ := o.Load("/tmp/x.odt")
    defer doc.Close()

    fb.getCommandValuesResult = `{"type":"list","command":".uno:FontName","value":["Arial","Times"]}`
    names, err := doc.GetFontNames()
    if err != nil {
        t.Fatal(err)
    }
    if len(names) != 2 || names[0] != "Arial" {
        t.Errorf("got %v", names)
    }
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./lok -run 'TestGetCommand|TestIsCommand|TestGetFont' -v`
Expected: PASS.

---

## Task 6: Public API — `lok/windows.go` and `lok/windows_paint.go`

**Files:**
- `lok/windows.go`        — events (key/mouse/gesture/text/resize)
- `lok/windows_paint.go`  — paint (PaintWindow / DPI / ForView)
- `lok/windows_test.go`   — unit tests for both

The split keeps the paint contract (buffer pinning, BGRA layout)
adjacent to the paint methods without the event surface having to read
that block. Both files live in package `lok` and need no imports
beyond what their bodies use — neither imports `errors` or `lokc`
directly; `requireInt32*` and the office mutex come from the same
package.

- [ ] **Step 1: Create `lok/windows.go` with the event methods**

```go
//go:build linux || darwin

package lok

// PostWindowKeyEvent posts a key event to a specific window.
func (d *Document) PostWindowKeyEvent(windowID uint32, typ KeyEventType, charCode, keyCode int) error {
    unlock, err := d.guard()
    if err != nil {
        return err
    }
    defer unlock()
    if err := requireInt32Key("PostWindowKeyEvent", charCode, keyCode); err != nil {
        return err
    }
    return d.office.be.PostWindowKeyEvent(d.h, windowID, int(typ), charCode, keyCode)
}

// PostWindowMouseEvent posts a mouse event to a specific window.
func (d *Document) PostWindowMouseEvent(windowID uint32, typ MouseEventType, x, y int64, count int, buttons MouseButton, mods Modifier) error {
    unlock, err := d.guard()
    if err != nil {
        return err
    }
    defer unlock()
    if err := requireInt32XY("PostWindowMouseEvent", x, y); err != nil {
        return err
    }
    return d.office.be.PostWindowMouseEvent(d.h, windowID, int(typ), x, y, count, int(buttons), int(mods))
}

// PostWindowGestureEvent / PostWindowExtTextInputEvent follow the same
// shape: guard → forward to backend.

// ResizeWindow changes the size of a window.
func (d *Document) ResizeWindow(windowID uint32, w, h int) error {
    unlock, err := d.guard()
    if err != nil {
        return err
    }
    defer unlock()
    if w <= 0 || h <= 0 {
        return ErrInvalidOption
    }
    return d.office.be.ResizeWindow(d.h, windowID, w, h)
}
```

- [ ] **Step 2: Create `lok/windows_paint.go`**

```go
//go:build linux || darwin

package lok

// PaintWindow paints a window into the provided buffer. len(buf) must
// equal 4*pxW*pxH (validated in internal/lokc); the format is
// premultiplied BGRA (same as PaintTileRaw). x, y are the top-left of
// the source rectangle in twips.
func (d *Document) PaintWindow(windowID uint32, buf []byte, x, y, pxW, pxH int) error {
    unlock, err := d.guard()
    if err != nil {
        return err
    }
    defer unlock()
    return d.office.be.PaintWindow(d.h, windowID, buf, x, y, pxW, pxH)
}

// PaintWindowDPI / PaintWindowForView follow the same shape; the latter
// passes int(view) directly — there is no view-handle indirection.
func (d *Document) PaintWindowForView(windowID uint32, view ViewID, buf []byte, x, y, pxW, pxH int, dpiScale float64) error {
    unlock, err := d.guard()
    if err != nil {
        return err
    }
    defer unlock()
    return d.office.be.PaintWindowForView(d.h, windowID, int(view), buf, x, y, pxW, pxH, dpiScale)
}
```

`ResetWindow` and `GetFontSubset` are intentionally not exported — the
LOK 24.8 vtable lacks both slots, and stubbing them would burn API
surface for hypothetical future versions (CLAUDE.md "don't design for
hypothetical future requirements"). Spec §9 records the deferral.

- [ ] **Step 3: Write unit tests**

Create `lok/windows_test.go`:

```go
//go:build linux || darwin

package lok

import (
    "errors"
    "testing"
)

func TestPostWindowKeyEvent(t *testing.T) {
    fb := &fakeBackend{}
    withFakeBackend(t, fb)
    o, _ := New("/install")
    defer o.Close()
    doc, _ := o.Load("/tmp/x.odt")
    defer doc.Close()

    if err := doc.PostWindowKeyEvent(123, KeyEventInput, 'A', 65); err != nil {
        t.Fatal(err)
    }
    if fb.lastWindowID != 123 {
        t.Errorf("lastWindowID=%d", fb.lastWindowID)
    }
}

func TestPostWindowMouseEvent(t *testing.T) {
    fb := &fakeBackend{}
    withFakeBackend(t, fb)
    o, _ := New("/install")
    defer o.Close()
    doc, _ := o.Load("/tmp/x.odt")
    defer doc.Close()

    if err := doc.PostWindowMouseEvent(456, MouseButtonDown, 100, 200, 1, MouseLeft, ModShift); err != nil {
        t.Fatal(err)
    }
}

func TestResizeWindow_InvalidSize(t *testing.T) {
    withFakeBackend(t, &fakeBackend{})
    o, _ := New("/install")
    defer o.Close()
    doc, _ := o.Load("/tmp/x.odt")
    defer doc.Close()

    if err := doc.ResizeWindow(1, -10, 100); !errors.Is(err, ErrInvalidOption) {
        t.Errorf("want ErrInvalidOption, got %v", err)
    }
}

func TestPaintWindow_Closed(t *testing.T) {
    withFakeBackend(t, &fakeBackend{})
    o, _ := New("/install")
    defer o.Close()
    doc, _ := o.Load("/tmp/x.odt")
    doc.Close()

    buf := make([]byte, 4*100*100)
    if err := doc.PaintWindow(1, buf, 0, 0, 100, 100); !errors.Is(err, ErrClosed) {
        t.Errorf("want ErrClosed, got %v", err)
    }
}
```

Buffer-size validation lives in `internal/lokc` (see Task 3 Step 3) and
is exercised by `internal/lokc/windows_test.go`, so the lok-level tests
cover the guard/forward path only.

- [ ] **Step 4: Run tests**

Run: `go test ./lok -run 'TestPostWindow|TestResizeWindow|TestPaintWindow' -v`
Expected: PASS.

---

## Task 7: Public API — `lok/forms.go`

**Files:**
- `lok/forms.go`
- `lok/forms_test.go`

- [ ] **Step 1: Implement form event methods**

Create `lok/forms.go`:

```go
//go:build linux || darwin

package lok

// SendDialogEvent sends a dialog event identified by windowID.
// argsJSON is a JSON object whose structure depends on the event type.
func (d *Document) SendDialogEvent(windowID uint64, argsJSON string) error {
    unlock, err := d.guard()
    if err != nil {
        return err
    }
    defer unlock()
    return d.office.be.SendDialogEvent(d.h, windowID, argsJSON)
}

// SendContentControlEvent sends an event for a content control.
func (d *Document) SendContentControlEvent(argsJSON string) error {
    unlock, err := d.guard()
    if err != nil {
        return err
    }
    defer unlock()
    return d.office.be.SendContentControlEvent(d.h, argsJSON)
}

// SendFormFieldEvent sends an event for a form field.
func (d *Document) SendFormFieldEvent(argsJSON string) error {
    unlock, err := d.guard()
    if err != nil {
        return err
    }
    defer unlock()
    return d.office.be.SendFormFieldEvent(d.h, argsJSON)
}
```

- [ ] **Step 2: Write unit tests**

Create `lok/forms_test.go`:

```go
//go:build linux || darwin

package lok

import (
    "testing"
)

func TestSendDialogEvent(t *testing.T) {
    fb := &fakeBackend{}
    withFakeBackend(t, fb)
    o, _ := New("/install")
    defer o.Close()
    doc, _ := o.Load("/tmp/x.odt")
    defer doc.Close()

    args := `{"type":"dialog","action":"execute"}`
    err := doc.SendDialogEvent(42, args)
    if err != nil {
        t.Fatal(err)
    }
    if fb.lastWindowID != 42 {
        t.Errorf("lastWindowID=%d", fb.lastWindowID)
    }
}

func TestSendContentControlEvent(t *testing.T) {
    fb := &fakeBackend{}
    withFakeBackend(t, fb)
    o, _ := New("/install")
    defer o.Close()
    doc, _ := o.Load("/tmp/x.odt")
    defer doc.Close()

    args := `{"control":"checkbox","action":"check"}`
    err := doc.SendContentControlEvent(args)
    if err != nil {
        t.Fatal(err)
    }
}

func TestSendFormFieldEvent(t *testing.T) {
    fb := &fakeBackend{}
    withFakeBackend(t, fb)
    o, _ := New("/install")
    defer o.Close()
    doc, _ := o.Load("/tmp/x.odt")
    defer doc.Close()

    args := `{"field":"name","action":"changed"}`
    err := doc.SendFormFieldEvent(args)
    if err != nil {
        t.Fatal(err)
    }
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./lok -run 'TestSend.*Event' -v`
Expected: PASS.

---

## Task 8: Integration smoke tests

**Files:**
- `lok/integration_test.go`

`lok_init` is a one-shot per process — see the file header (`lok/integration_test.go:14-32`)
and CLAUDE.md memory ("Singleton per process … integration tests share
ONE New/Close pair"). The package today exposes no `testOffice` helper;
all integration assertions live inside `TestIntegration_FullLifecycle`
and reuse its local `o`. Phase 10 follows the same pattern: extend that
test, do not introduce new top-level integration tests.

- [ ] **Step 1: Extend `TestIntegration_FullLifecycle` with command-value assertions**

After the existing `doc, err := o.Load(fixture)` block, add:

```go
// Phase 10: command values.
raw, err := doc.GetCommandValues(".uno:Save")
if err != nil {
    t.Errorf("GetCommandValues(.uno:Save): %v", err)
} else {
    t.Logf(".uno:Save: %s", raw)
    var m map[string]any
    if jerr := json.Unmarshal(raw, &m); jerr != nil {
        t.Errorf("GetCommandValues returned invalid JSON: %v", jerr)
    }
}

// CompleteFunction is silent for non-Calc documents; just verify it
// doesn't blow up on the loaded fixture.
if err := doc.CompleteFunction("SUM"); err != nil && !errors.Is(err, ErrUnsupported) {
    t.Errorf("CompleteFunction: %v", err)
}
```

Add `"encoding/json"` to the file's import block if not already present.

- [ ] **Step 2: Extend `TestIntegration_FullLifecycle` with window event assertions**

LOK does not expose a synchronous way to discover a window ID; the IDs
arrive asynchronously via `EventTypeWindow` callbacks (Phase 9). The
test attaches a listener, drives an interaction that LOK is known to
respond to with a window event, and skips with a logged message if no
window ID surfaces inside a short deadline:

```go
// Phase 10: window event smoke. LOK only allocates window IDs in
// response to UI activity; if none arrive within the deadline, skip
// the body rather than fail (the rest of the lifecycle has run).
gotID := make(chan uint32, 1)
unsubscribe, _ := doc.AddListener(func(e Event) {
    if e.Type == EventTypeWindow {
        select {
        case gotID <- parseWindowIDFromPayload(e.Payload):
        default:
        }
    }
})
defer unsubscribe()

// Trigger something likely to produce a window: opening the styles sidebar.
_ = doc.PostUnoCommand(".uno:DesignerDialog", "", false)

select {
case wid := <-gotID:
    if err := doc.ResizeWindow(wid, 200, 200); err != nil {
        t.Errorf("ResizeWindow: %v", err)
    }
case <-time.After(2 * time.Second):
    t.Logf("no EventTypeWindow within deadline; skipping window-event smoke")
}
```

`parseWindowIDFromPayload` is a tiny test helper that decodes the JSON
payload LOK delivers for `EventTypeWindow` (a JSON object with an
`"id"` field). Keep it in the same test file.

- [ ] **Step 3: Run integration tests**

```bash
make test-integration
# equivalent: GODEBUG=asyncpreemptoff=1 LOK_PATH=/usr/lib/libreoffice/program \
#   go test -tags=lok_integration -run TestIntegration_FullLifecycle ./lok -v
```

`GODEBUG=asyncpreemptoff=1` is required — LO installs SIGWINCH/SIGPIPE
handlers without `SA_ONSTACK` and Go's async preemption (SIGURG)
crashes the runtime otherwise. The `Makefile` target sets it for you.

Expected: PASS, with the window-event smoke either asserting
`ResizeWindow` cleanly or logging the skip.

---

## Task 9: Coverage verification

**Files:**
- All test files

- [ ] **Step 1: Run coverage**

```bash
go test -covermode=atomic -coverprofile=coverage.out ./lok/...
go tool cover -func=coverage.out | tail -5
```

Expected: `lok` package coverage ≥ 90%.

- [ ] **Step 2: Check uncovered lines**

```bash
go tool cover -html=coverage.out -o coverage.html
```

Verify uncovered lines are trivial or cgo wrappers.

- [ ] **Step 3: Race detector — unit tests only**

```bash
go test -race ./lok/...
```

Expected: All unit tests pass, no race conditions.

`-race` is **not** combined with `-tags=lok_integration` per CLAUDE.md
memory ("no -race on test-integration"); LO's signal-handler quirks
make race-instrumented integration runs unstable.

---

## Task 10: Final review and commit

- [ ] **Step 1: Ensure all files compile**

```bash
go build ./...
go vet ./...
gofmt -s -l .  # should be empty
```

- [ ] **Step 2: Create feature branch**

```bash
git checkout -b feat/command-values main
```

- [ ] **Step 3: Add and commit**

```bash
git add .
git commit -m "feat(lok): Phase 10 — command values & window events

- Add Document.GetCommandValues, CompleteFunction, IsCommandEnabled,
  GetFontNames (appended to existing lok/commands.go)
- Add window event APIs: PostWindow{Key,Mouse,Gesture,ExtTextInput}Event,
  ResizeWindow (lok/windows.go)
- Add window paint APIs: PaintWindow, PaintWindowDPI, PaintWindowForView
  (lok/windows_paint.go)
- Add dialog/content-control/form-field event helpers (lok/forms.go)
- internal/lokc: new commands.{go,c,h} and windows.{go,c,h} expose plain
  Go signatures over the new vtable slots (lok/real_backend.go stays
  cgo-free, one-line forwarders only)
- fakeBackend stubs and unit tests for the new surface
- Extend TestIntegration_FullLifecycle with command-value and window-event
  smoke checks (no new top-level integration test)
- Coverage ≥ 90% for lok package"
```

- [ ] **Step 4: Verify commit**

```bash
git log --oneline -1
git diff HEAD~1 --stat
```

Expected: Clean commit with all Phase 10 files.

---

## Success Criteria

- [ ] All unit tests pass (`go test ./lok -race`).
- [ ] `make test-integration` passes when `LOK_PATH` is set (no `-race`).
- [ ] `lok` package coverage ≥ 90%.
- [ ] `internal/lokc` coverage ≥ 90% (excluding trivial cgo wrappers).
- [ ] No new lint errors (`go vet`, `gofmt`).
- [ ] Public API matches design spec.
- [ ] Documentation (godoc) added for all new exported symbols.
- [ ] `lok/real_backend.go` remains free of `import "C"`.

## Notes

- Command names are case-sensitive and must match LOK's `.uno:*` constants.
- Window IDs are `uint32` (LOK uses `unsigned`). `SendDialogEvent` takes
  `uint64` to match LOK's `unsigned long long`.
- `PaintWindow*` methods follow the same pointer-safety contract as
  `PaintTileRaw`: buffer is pinned only for the synchronous C call.
  `internal/lokc/windows.go` validates `len(buf) == 4*pxW*pxH` before
  invoking the shim.
- JSON returned by `GetCommandValues` is not parsed by `lok`; callers
  unmarshal into their own types as needed. The two convenience helpers
  (`IsCommandEnabled`, `GetFontNames`) parse only the top-level shape
  and treat absent fields as "disabled" / "empty list" — they intentionally
  do not surface "field missing" as an error.
- All new methods on `Document` use `guard()` to acquire the office mutex
  and check both `d.closed` and `d.office.closed`.
- `ResetWindow` / `GetFontSubset` — neither vtable slot exists in LOK
  24.8; no Go method ships in this phase. Revisit when LO adds them.
- `CompleteFunction` is a no-op for non-Calc documents (LOK silently
  ignores it).
- All cgo lives strictly inside `internal/lokc`; the `lok` package
  contains no `import "C"`. This keeps `lok` testable without cgo where
  possible and matches the layering of Phases 3–9.
