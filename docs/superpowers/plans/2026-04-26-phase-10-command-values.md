# Phase 10 — Command values & window events Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bind LibreOfficeKit's command-value query API (`getCommandValues`, `getCommandValues` for documents) and window-level APIs (window events, window paint, dialog/content-control/form-field events) so users can query command states and interact with windows from Go.

**Architecture:** Same four-layer pattern as Phases 3–9. `internal/lokc` owns the C shims for command/window functions; `lok` owns the public API, error mapping, and JSON handling.

**Tech Stack:** Go + cgo on `linux || darwin`, LibreOfficeKit C ABI, `//export` trampolines already in place from Phase 9, `go test` + `lok_integration` build tag.

**Branch:** `feat/command-values` (to be created from `main`)

**Spec:** `docs/superpowers/specs/2026-04-26-phase-10-command-values-design.md`

---

## File Structure

Files created:

- `lok/commands.go` — `GetCommandValues`, `CompleteFunction`, typed helpers.
- `lok/commands_test.go` — unit tests for command values.
- `lok/windows.go` — window events, paint, resize, font subset.
- `lok/windows_test.go` — unit tests for window APIs.
- `lok/forms.go` — dialog/content-control/form-field event helpers.
- `lok/forms_test.go` — unit tests for form events.
- `internal/lokc/commands.c` — C shims for `getCommandValues`, `completeFunction`, etc.
- `internal/lokc/commands.h` — declarations for above.
- `internal/lokc/windows.c` — C shims for window paint/resize/events.
- `internal/lokc/windows.h` — declarations for above.

Files modified:

- `lok/backend.go` — add new interface methods.
- `lok/real_backend.go` — forwarders for command/window methods.
- `lok/document.go` — attach new methods to `Document`.
- `lok/integration_test.go` — add integration smoke for command values and window paint.
- `internal/lokc/lokc.go` — add `#include` for new headers.

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
CompleteFunction(d documentHandle, part int, name string) error
SendDialogEvent(d documentHandle, windowID uint64, argsJSON string) error
SendContentControlEvent(d documentHandle, argsJSON string) error
SendFormFieldEvent(d documentHandle, argsJSON string) error
PostWindowKeyEvent(d documentHandle, windowID uint64, typ int, charCode, keyCode int) error
PostWindowMouseEvent(d documentHandle, windowID uint64, typ int, x, y int64, count int, buttons, mods int) error
PostWindowGestureEvent(d documentHandle, windowID uint64, typ string, x, y, offset int64) error
PostWindowExtTextInputEvent(d documentHandle, windowID uint64, typ int, text string) error
ResizeWindow(d documentHandle, windowID uint64, w, h int) error
PaintWindow(d documentHandle, windowID uint64, buf []byte, pxW, pxH int) error
PaintWindowDPI(d documentHandle, windowID uint64, buf []byte, pxW, pxH int, dpiScale float64) error
PaintWindowForView(d documentHandle, windowID uint64, view viewHandle, buf []byte, pxW, pxH int, dpiScale float64) error
ResetWindow(d documentHandle, windowID uint64) error
GetFontSubset(d documentHandle, fontName string) ([]byte, error)
```

Note: Use `[]byte` for buffers; keep paint methods consistent with `PaintTileRaw`.

- [ ] **Step 2: Extend fakeBackend**

In `lok/office_test.go`, add fields to `fakeBackend`:

```go
// Phase 10: command/window tracking.
lastCommand           string
lastCommandResult     string
lastCommandErr        error
lastWindowID          uint64
lastWindowBuf         []byte
lastWindowFontName    string
lastWindowFontData    []byte
```

Add stub methods (one per interface method) that record calls and return configurable errors/results. Example:

```go
func (f *fakeBackend) GetCommandValues(_ documentHandle, cmd string) (string, error) {
    f.lastCommand = cmd
    if f.getCommandValuesErr != nil {
        return "", f.getCommandValuesErr
    }
    return f.lastCommandResult, nil
}
```

- [ ] **Step 3: Build to verify compilation**

Run: `go build ./lok/...`
Expected: FAIL — realBackend missing methods.

---

## Task 2: C shims for commands

**Files:**
- `internal/lokc/commands.h`
- `internal/lokc/commands.c`
- `internal/lokc/lokc.go` (add `#include`)

- [ ] **Step 1: Create `commands.h`**

```c
#ifndef LOKC_COMMANDS_H
#define LOKC_COMMANDS_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

// Returns 1 on success, 0 on failure (e.g. NULL handle, missing vtable).
// On success, *out_len is set to the length of the returned string (no null terminator).
// The caller must free the returned buffer with free().
int loke_get_command_values(void* doc, const char* command, char** out, size_t* out_len);

// Returns 1 on success, 0 on failure.
int loke_complete_function(void* doc, int part, const char* name);

#ifdef __cplusplus
}
#endif

#endif
```

- [ ] **Step 2: Create `commands.c`**

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
    *out = s;  // caller will free
    return 1;
}

int loke_complete_function(void* doc, int part, const char* name) {
    if (!doc || !name) return 0;
    LibreOfficeKitDocument* d = (LibreOfficeKitDocument*)doc;
    if (!d->pClass || !d->pClass->completeFunction) return 0;
    d->pClass->completeFunction(d, part, name);
    return 1;
}
```

- [ ] **Step 3: Update `lokc.go` to include commands**

Add to the cgo preamble section (or create a separate file included from `lokc.go`):

```go
/*
#cgo CFLAGS: -I${SRCDIR}/../../third_party/lok -DLOK_USE_UNSTABLE_API
#include <stdlib.h>
#include "LibreOfficeKit/LibreOfficeKit.h"
#include "callback.h"  // existing
#include "commands.h"  // new
#include "windows.h"   // new (will create)
*/
import "C"
```

- [ ] **Step 4: Build C code**

Run: `go build ./internal/lokc`
Expected: compiles cleanly.

---

## Task 3: C shims for windows

**Files:**
- `internal/lokc/windows.h`
- `internal/lokc/windows.c`

- [ ] **Step 1: Create `windows.h`**

```c
#ifndef LOKC_WINDOWS_H
#define LOKC_WINDOWS_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

// Window events
int loke_post_window_key_event(void* doc, uint64_t window_id, int type, int char_code, int key_code);
int loke_post_window_mouse_event(void* doc, uint64_t window_id, int type, int64_t x, int64_t y, int count, int buttons, int mods);
int loke_post_window_gesture_event(void* doc, uint64_t window_id, const char* typ, int64_t x, int64_t y, int64_t offset);
int loke_post_window_ext_text_input_event(void* doc, uint64_t window_id, int typ, const char* text);
int loke_resize_window(void* doc, uint64_t window_id, int w, int h);

// Window paint
int loke_paint_window(void* doc, uint64_t window_id, void* buf, int px_w, int px_h);
int loke_paint_window_dpi(void* doc, uint64_t window_id, void* buf, int px_w, int px_h, double dpi_scale);
int loke_paint_window_for_view(void* doc, uint64_t window_id, void* view, void* buf, int px_w, int px_h, double dpi_scale);
int loke_reset_window(void* doc, uint64_t window_id);

// Font subset
// Returns 1 on success; *out must be freed by caller with free().
int loke_get_font_subset(void* doc, const char* font_name, char** out, size_t* out_len);

#ifdef __cplusplus
}
#endif

#endif
```

- [ ] **Step 2: Create `windows.c`**

```c
#include "windows.h"
#include "LibreOfficeKit/LibreOfficeKit.h"
#include <stdlib.h>
#include <string.h>

int loke_post_window_key_event(void* doc, uint64_t window_id, int type, int char_code, int key_code) {
    if (!doc) return 0;
    LibreOfficeKitDocument* d = (LibreOfficeKitDocument*)doc;
    if (!d->pClass || !d->pClass->postWindowKeyEvent) return 0;
    return d->pClass->postWindowKeyEvent(d, window_id, type, char_code, key_code);
}

int loke_post_window_mouse_event(void* doc, uint64_t window_id, int type, int64_t x, int64_t y, int count, int buttons, int mods) {
    if (!doc) return 0;
    LibreOfficeKitDocument* d = (LibreOfficeKitDocument*)doc;
    if (!d->pClass || !d->pClass->postWindowMouseEvent) return 0;
    return d->pClass->postWindowMouseEvent(d, window_id, type, x, y, count, buttons, mods);
}

int loke_post_window_gesture_event(void* doc, uint64_t window_id, const char* typ, int64_t x, int64_t y, int64_t offset) {
    if (!doc || !typ) return 0;
    LibreOfficeKitDocument* d = (LibreOfficeKitDocument*)doc;
    if (!d->pClass || !d->pClass->postWindowGestureEvent) return 0;
    return d->pClass->postWindowGestureEvent(d, window_id, typ, x, y, offset);
}

int loke_post_window_ext_text_input_event(void* doc, uint64_t window_id, int typ, const char* text) {
    if (!doc || !text) return 0;
    LibreOfficeKitDocument* d = (LibreOfficeKitDocument*)doc;
    if (!d->pClass || !d->pClass->postWindowExtTextInputEvent) return 0;
    return d->pClass->postWindowExtTextInputEvent(d, window_id, typ, text);
}

int loke_resize_window(void* doc, uint64_t window_id, int w, int h) {
    if (!doc) return 0;
    LibreOfficeKitDocument* d = (LibreOfficeKitDocument*)doc;
    if (!d->pClass || !d->pClass->resizeWindow) return 0;
    return d->pClass->resizeWindow(d, window_id, w, h);
}

int loke_paint_window(void* doc, uint64_t window_id, void* buf, int px_w, int px_h) {
    if (!doc || !buf) return 0;
    LibreOfficeKitDocument* d = (LibreOfficeKitDocument*)doc;
    if (!d->pClass || !d->pClass->paintWindow) return 0;
    return d->pClass->paintWindow(d, window_id, buf, px_w, px_h);
}

int loke_paint_window_dpi(void* doc, uint64_t window_id, void* buf, int px_w, int px_h, double dpi_scale) {
    if (!doc || !buf) return 0;
    LibreOfficeKitDocument* d = (LibreOfficeKitDocument*)doc;
    if (!d->pClass || !d->pClass->paintWindowDPI) return 0;
    return d->pClass->paintWindowDPI(d, window_id, buf, px_w, px_h, dpi_scale);
}

int loke_paint_window_for_view(void* doc, uint64_t window_id, void* view, void* buf, int px_w, int px_h, double dpi_scale) {
    if (!doc || !buf) return 0;
    LibreOfficeKitDocument* d = (LibreOfficeKitDocument*)doc;
    if (!d->pClass || !d->pClass->paintWindowForView) return 0;
    return d->pClass->paintWindowForView(d, window_id, view, buf, px_w, px_h, dpi_scale);
}

int loke_reset_window(void* doc, uint64_t window_id) {
    if (!doc) return 0;
    LibreOfficeKitDocument* d = (LibreOfficeKitDocument*)doc;
    if (!d->pClass || !d->pClass->resetWindow) return 0;
    return d->pClass->resetWindow(d, window_id);
}

int loke_get_font_subset(void* doc, const char* font_name, char** out, size_t* out_len) {
    if (!doc || !font_name || !out || !out_len) return 0;
    LibreOfficeKitDocument* d = (LibreOfficeKitDocument*)doc;
    if (!d->pClass || !d->pClass->getFontSubset) return 0;
    char* s = d->pClass->getFontSubset(d, font_name);
    if (!s) return 0;
    *out_len = strlen(s);
    *out = s;
    return 1;
}
```

- [ ] **Step 3: Build**

Run: `go build ./internal/lokc`
Expected: compiles cleanly.

---

## Task 4: realBackend forwarders

**Files:**
- `lok/real_backend.go`
- `internal/lokc/callback.go` (add DispatchHandle helpers)

- [ ] **Step 1: Add DispatchHandle helpers to `internal/lokc/callback.go`**

Append:

```go
// DispatchHandleFromUintptr converts a uintptr to dispatchHandle.
func DispatchHandleFromUintptr(v uintptr) dispatchHandle {
    return dispatchHandle(v)
}

// UintptrFromDispatchHandle converts dispatchHandle to uintptr.
func UintptrFromDispatchHandle(h dispatchHandle) uintptr {
    return uintptr(h)
}

// RegisterDispatcherUintptr is a convenience wrapper returning uintptr.
func RegisterDispatcherUintptr(d Dispatcher) uintptr {
    return uintptr(RegisterDispatcher(d))
}

// UnregisterDispatcherUintptr is the symmetric inverse.
func UnregisterDispatcherUintptr(h uintptr) {
    UnregisterDispatcher(dispatchHandle(h))
}
```

- [ ] **Step 2: Add forwarders in `lok/real_backend.go`**

Before the `var _ backend = realBackend{}` line, add:

```go
// --- Command & window operations (Phase 10) ---

func (realBackend) GetCommandValues(d documentHandle, command string) (string, error) {
    var out *C.char
    var outLen C.size_t
    ok := C.loke_get_command_values(mustDoc(d).d, C.CString(command), &out, &outLen)
    if ok == 0 {
        return "", mapLokErr(getDocError(mustDoc(d).d))
    }
    defer C.free(unsafe.Pointer(out))
    // Return as string; lok layer will convert to json.RawMessage.
    return C.GoStringN(out, C.int(outLen)), nil
}

func (realBackend) CompleteFunction(d documentHandle, part int, name string) error {
    ok := C.loke_complete_function(mustDoc(d).d, C.int(part), C.CString(name))
    if ok == 0 {
        return mapLokErr(getDocError(mustDoc(d).d))
    }
    return nil
}

func (realBackend) SendDialogEvent(d documentHandle, windowID uint64, argsJSON string) error {
    // Note: LOK's sendDialogEvent takes windowID and argsJSON.
    // We need to check if the function exists in the vtable.
    // For now, use a generic approach via document's pClass.
    // The C shim will handle NULL slot.
    ok := C.loke_send_dialog_event(mustDoc(d).d, C.uint64_t(windowID), C.CString(argsJSON))
    if ok == 0 {
        return mapLokErr(getDocError(mustDoc(d).d))
    }
    return nil
}

func (realBackend) SendContentControlEvent(d documentHandle, argsJSON string) error {
    ok := C.loke_send_content_control_event(mustDoc(d).d, C.CString(argsJSON))
    if ok == 0 {
        return mapLokErr(getDocError(mustDoc(d).d))
    }
    return nil
}

func (realBackend) SendFormFieldEvent(d documentHandle, argsJSON string) error {
    ok := C.loke_send_form_field_event(mustDoc(d).d, C.CString(argsJSON))
    if ok == 0 {
        return mapLokErr(getDocError(mustDoc(d).d))
    }
    return nil
}

func (realBackend) PostWindowKeyEvent(d documentHandle, windowID uint64, typ, charCode, keyCode int) error {
    ok := C.loke_post_window_key_event(mustDoc(d).d, C.uint64_t(windowID), C.int(typ), C.int(charCode), C.int(keyCode))
    if ok == 0 {
        return mapLokErr(getDocError(mustDoc(d).d))
    }
    return nil
}

func (realBackend) PostWindowMouseEvent(d documentHandle, windowID uint64, typ int, x, y int64, count int, buttons, mods int) error {
    ok := C.loke_post_window_mouse_event(mustDoc(d).d, C.uint64_t(windowID), C.int(typ), C.int64_t(x), C.int64_t(y), C.int(count), C.int(buttons), C.int(mods))
    if ok == 0 {
        return mapLokErr(getDocError(mustDoc(d).d))
    }
    return nil
}

func (realBackend) PostWindowGestureEvent(d documentHandle, windowID uint64, typ string, x, y, offset int64) error {
    ok := C.loke_post_window_gesture_event(mustDoc(d).d, C.uint64_t(windowID), C.CString(typ), C.int64_t(x), C.int64_t(y), C.int64_t(offset))
    if ok == 0 {
        return mapLokErr(getDocError(mustDoc(d).d))
    }
    return nil
}

func (realBackend) PostWindowExtTextInputEvent(d documentHandle, windowID uint64, typ int, text string) error {
    ok := C.loke_post_window_ext_text_input_event(mustDoc(d).d, C.uint64_t(windowID), C.int(typ), C.CString(text))
    if ok == 0 {
        return mapLokErr(getDocError(mustDoc(d).d))
    }
    return nil
}

func (realBackend) ResizeWindow(d documentHandle, windowID uint64, w, h int) error {
    ok := C.loke_resize_window(mustDoc(d).d, C.uint64_t(windowID), C.int(w), C.int(h))
    if ok == 0 {
        return mapLokErr(getDocError(mustDoc(d).d))
    }
    return nil
}

func (realBackend) PaintWindow(d documentHandle, windowID uint64, buf []byte, pxW, pxH int) error {
    if len(buf) != 4*pxW*pxH {
        return ErrInvalidOption
    }
    ok := C.loke_paint_window(mustDoc(d).d, C.uint64_t(windowID), unsafe.Pointer(&buf[0]), C.int(pxW), C.int(pxH))
    if ok == 0 {
        return mapLokErr(getDocError(mustDoc(d).d))
    }
    return nil
}

func (realBackend) PaintWindowDPI(d documentHandle, windowID uint64, buf []byte, pxW, pxH int, dpiScale float64) error {
    if len(buf) != 4*pxW*pxH {
        return ErrInvalidOption
    }
    ok := C.loke_paint_window_dpi(mustDoc(d).d, C.uint64_t(windowID), unsafe.Pointer(&buf[0]), C.int(pxW), C.int(pxH), C.double(dpiScale))
    if ok == 0 {
        return mapLokErr(getDocError(mustDoc(d).d))
    }
    return nil
}

func (realBackend) PaintWindowForView(d documentHandle, windowID uint64, view viewHandle, buf []byte, pxW, pxH int, dpiScale float64) error {
    if len(buf) != 4*pxW*pxH {
        return ErrInvalidOption
    }
    ok := C.loke_paint_window_for_view(mustDoc(d).d, C.uint64_t(windowID), mustView(view).v, unsafe.Pointer(&buf[0]), C.int(pxW), C.int(pxH), C.double(dpiScale))
    if ok == 0 {
        return mapLokErr(getDocError(mustDoc(d).d))
    }
    return nil
}

func (realBackend) ResetWindow(d documentHandle, windowID uint64) error {
    ok := C.loke_reset_window(mustDoc(d).d, C.uint64_t(windowID))
    if ok == 0 {
        return mapLokErr(getDocError(mustDoc(d).d))
    }
    return nil
}

func (realBackend) GetFontSubset(d documentHandle, fontName string) ([]byte, error) {
    var out *C.char
    var outLen C.size_t
    ok := C.loke_get_font_subset(mustDoc(d).d, C.CString(fontName), &out, &outLen)
    if ok == 0 {
        return nil, mapLokErr(getDocError(mustDoc(d).d))
    }
    defer C.free(unsafe.Pointer(out))
    return C.GoBytes(unsafe.Pointer(out), C.int(outLen)), nil
}
```

Also add the missing C shims to `commands.h`/`windows.h` for dialog/content-control/form-field events:

In `commands.h` add:
```c
int loke_send_dialog_event(void* doc, uint64_t window_id, const char* args_json);
int loke_send_content_control_event(void* doc, const char* args_json);
int loke_send_form_field_event(void* doc, const char* args_json);
```

In `commands.c` add:
```c
int loke_send_dialog_event(void* doc, uint64_t window_id, const char* args_json) {
    if (!doc || !args_json) return 0;
    LibreOfficeKitDocument* d = (LibreOfficeKitDocument*)doc;
    if (!d->pClass || !d->pClass->sendDialogEvent) return 0;
    return d->pClass->sendDialogEvent(d, window_id, args_json);
}
int loke_send_content_control_event(void* doc, const char* args_json) {
    if (!doc || !args_json) return 0;
    LibreOfficeKitDocument* d = (LibreOfficeKitDocument*)doc;
    if (!d->pClass || !d->pClass->sendContentControlEvent) return 0;
    return d->pClass->sendContentControlEvent(d, args_json);
}
int loke_send_form_field_event(void* doc, const char* args_json) {
    if (!doc || !args_json) return 0;
    LibreOfficeKitDocument* d = (LibreOfficeKitDocument*)doc;
    if (!d->pClass || !d->pClass->sendFormFieldEvent) return 0;
    return d->pClass->sendFormFieldEvent(d, args_json);
}
```

- [ ] **Step 3: Add `getDocError` helper to `real_backend.go`**

```go
func getDocError(d *C.LibreOfficeKitDocument) error {
    if d == nil || d.pClass == nil || d.pClass->getError == nil {
        return nil
    }
    cerr := d.pClass.getError(d)
    if cerr == nil {
        return nil
    }
    defer C.free(unsafe.Pointer(cerr))
    return &LOKError{Op: "getError", Detail: C.GoString(cerr)}
}
```

- [ ] **Step 4: Build**

Run: `go build ./...`
Expected: compiles cleanly.

---

## Task 5: Public API — `lok/commands.go`

**Files:**
- `lok/commands.go`
- `lok/commands_test.go`

- [ ] **Step 1: Implement `GetCommandValues`**

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
    d.office.mu.Lock()
    defer d.office.mu.Unlock()
    if d.closed {
        return nil, ErrClosed
    }
    s, err := d.office.be.GetCommandValues(d.h, command)
    if err != nil {
        return nil, err
    }
    return json.RawMessage(s), nil
}
```

- [ ] **Step 2: Implement `CompleteFunction`**

```go
// CompleteFunction attempts to complete a function (formula) in a spreadsheet.
// part is the part index (sheet), name is the function name. Returns an error
// if the document is not a spreadsheet or the function cannot be completed.
func (d *Document) CompleteFunction(part int, name string) error {
    d.office.mu.Lock()
    defer d.office.mu.Unlock()
    if d.closed {
        return ErrClosed
    }
    return d.office.be.CompleteFunction(d.h, part, name)
}
```

- [ ] **Step 3: Typed helpers**

```go
// IsCommandEnabled returns whether command is currently enabled.
func (d *Document) IsCommandEnabled(cmd string) (bool, error) {
    raw, err := d.GetCommandValues(cmd)
    if err != nil {
        return false, err
    }
    // LOK returns JSON like {"enabled": true} or {"state": true}
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
func (d *Document) GetFontNames() ([]string, error) {
    raw, err := d.GetCommandValues(".uno:FontName")
    if err != nil {
        return nil, err
    }
    // Expected: { "type": "list", "command": ".uno:FontName",
    //             "value": [ "Arial", "Times New Roman", ... ] }
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

- [ ] **Step 4: Write unit tests**

Create `lok/commands_test.go`:

```go
//go:build linux || darwin

package lok

import (
    "encoding/json"
    "testing"
)

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

- [ ] **Step 5: Run tests**

Run: `go test ./lok -run 'TestGetCommand|TestIsCommand|TestGetFont' -v`
Expected: PASS.

---

## Task 6: Public API — `lok/windows.go`

**Files:**
- `lok/windows.go`
- `lok/windows_test.go`

- [ ] **Step 1: Implement window event methods**

```go
// PostWindowKeyEvent posts a key event to a specific window.
func (d *Document) PostWindowKeyEvent(windowID uint64, typ KeyEventType, charCode, keyCode int) error {
    d.office.mu.Lock()
    defer d.office.mu.Unlock()
    if d.closed {
        return ErrClosed
    }
    return d.office.be.PostWindowKeyEvent(d.h, windowID, int(typ), charCode, keyCode)
}

// PostWindowMouseEvent posts a mouse event to a specific window.
func (d *Document) PostWindowMouseEvent(windowID uint64, typ MouseEventType, x, y int64, count int, buttons MouseButtons, mods Modifiers) error {
    d.office.mu.Lock()
    defer d.office.mu.Unlock()
    if d.closed {
        return ErrClosed
    }
    return d.office.be.PostWindowMouseEvent(d.h, windowID, int(typ), x, y, count, int(buttons), int(mods))
}

// PostWindowGestureEvent posts a gesture event (pan/zoom) to a window.
func (d *Document) PostWindowGestureEvent(windowID uint64, typ string, x, y, offset int64) error {
    d.office.mu.Lock()
    defer d.office.mu.Unlock()
    if d.closed {
        return ErrClosed
    }
    return d.office.be.PostWindowGestureEvent(d.h, windowID, typ, x, y, offset)
}

// PostWindowExtTextInputEvent posts extended text input to a window.
func (d *Document) PostWindowExtTextInputEvent(windowID uint64, typ int, text string) error {
    d.office.mu.Lock()
    defer d.office.mu.Unlock()
    if d.closed {
        return ErrClosed
    }
    return d.office.be.PostWindowExtTextInputEvent(d.h, windowID, typ, text)
}

// ResizeWindow changes the size of a window.
func (d *Document) ResizeWindow(windowID uint64, w, h int) error {
    d.office.mu.Lock()
    defer d.office.mu.Unlock()
    if d.closed {
        return ErrClosed
    }
    if w <= 0 || h <= 0 {
        return ErrInvalidOption
    }
    return d.office.be.ResizeWindow(d.h, windowID, w, h)
}
```

- [ ] **Step 2: Implement window paint methods**

```go
// PaintWindow paints a window into the provided buffer. The buffer must
// have length 4*pxW*pxH. Returns premultiplied BGRA (same format as PaintTileRaw).
func (d *Document) PaintWindow(windowID uint64, buf []byte, pxW, pxH int) error {
    d.office.mu.Lock()
    defer d.office.mu.Unlock()
    if d.closed {
        return ErrClosed
    }
    return d.office.be.PaintWindow(d.h, windowID, buf, pxW, pxH)
}

// PaintWindowDPI paints a window with a DPI scale factor.
func (d *Document) PaintWindowDPI(windowID uint64, buf []byte, pxW, pxH int, dpiScale float64) error {
    d.office.mu.Lock()
    defer d.office.mu.Unlock()
    if d.closed {
        return ErrClosed
    }
    return d.office.be.PaintWindowDPI(d.h, windowID, buf, pxW, pxH, dpiScale)
}

// PaintWindowForView paints a window for a specific view ID.
func (d *Document) PaintWindowForView(windowID uint64, view ViewID, buf []byte, pxW, pxH int, dpiScale float64) error {
    d.office.mu.Lock()
    defer d.office.mu.Unlock()
    if d.closed {
        return ErrClosed
    }
    vh, ok := d.viewHandles[view]
    if !ok {
        return ErrInvalidOption
    }
    return d.office.be.PaintWindowForView(d.h, windowID, vh, buf, pxW, pxH, dpiScale)
}

// ResetWindow resets a window's internal state.
func (d *Document) ResetWindow(windowID uint64) error {
    d.office.mu.Lock()
    defer d.office.mu.Unlock()
    if d.closed {
        return ErrClosed
    }
    return d.office.be.ResetWindow(d.h, windowID)
}

// GetFontSubset retrieves a subset of a font as a byte slice (SFNT).
func (d *Document) GetFontSubset(fontName string) ([]byte, error) {
    d.office.mu.Lock()
    defer d.office.mu.Unlock()
    if d.closed {
        return nil, ErrClosed
    }
    return d.office.be.GetFontSubset(d.h, fontName)
}
```

- [ ] **Step 3: Write unit tests**

Create `lok/windows_test.go`:

```go
//go:build linux || darwin

package lok

import (
    "testing"
)

func TestPostWindowKeyEvent(t *testing.T) {
    fb := &fakeBackend{}
    withFakeBackend(t, fb)
    o, _ := New("/install")
    defer o.Close()
    doc, _ := o.Load("/tmp/x.odt")
    defer doc.Close()

    err := doc.PostWindowKeyEvent(123, KEY_PRESS, 'A', 65)
    if err != nil {
        t.Fatal(err)
    }
    if fb.lastWindowID != 123 {
        t.Errorf("lastWindowID=%d", fb.lastWindowID)
    }
}

func TestPaintWindow_InvalidBuffer(t *testing.T) {
    withFakeBackend(t, &fakeBackend{})
    o, _ := New("/install")
    defer o.Close()
    doc, _ := o.Load("/tmp/x.odt")
    defer doc.Close()

    buf := make([]byte, 100) // wrong size
    err := doc.PaintWindow(1, buf, 5, 5) // needs 4*5*5=100... actually ok
    // Actually 4*5*5 = 100, so this is valid. Let's use wrong size.
    buf2 := make([]byte, 99)
    err = doc.PaintWindow(1, buf2, 5, 5)
    if !errors.Is(err, ErrInvalidOption) {
        t.Errorf("want ErrInvalidOption for wrong buffer size, got %v", err)
    }
}

func TestGetFontSubset(t *testing.T) {
    fb := &fakeBackend{}
    withFakeBackend(t, fb)
    o, _ := New("/install")
    defer o.Close()
    doc, _ := o.Load("/tmp/x.odt")
    defer doc.Close()

    fb.lastWindowFontData = []byte("SFNT...")
    data, err := doc.GetFontSubset("Arial")
    if err != nil {
        t.Fatal(err)
    }
    if string(data) != "SFNT..." {
        t.Errorf("got %s", data)
    }
    if fb.lastWindowFontName != "Arial" {
        t.Errorf("fontName=%s", fb.lastWindowFontName)
    }
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./lok -run 'TestPostWindow|TestPaintWindow|TestGetFontSubset' -v`
Expected: PASS.

---

## Task 7: Public API — `lok/forms.go`

**Files:**
- `lok/forms.go`
- `lok/forms_test.go`

- [ ] **Step 1: Implement form event methods**

```go
// SendDialogEvent sends a dialog event identified by windowID.
// argsJSON is a JSON object whose structure depends on the event type.
func (d *Document) SendDialogEvent(windowID uint64, argsJSON string) error {
    d.office.mu.Lock()
    defer d.office.mu.Unlock()
    if d.closed {
        return ErrClosed
    }
    return d.office.be.SendDialogEvent(d.h, windowID, argsJSON)
}

// SendContentControlEvent sends an event for a content control.
func (d *Document) SendContentControlEvent(argsJSON string) error {
    d.office.mu.Lock()
    defer d.office.mu.Unlock()
    if d.closed {
        return ErrClosed
    }
    return d.office.be.SendContentControlEvent(d.h, argsJSON)
}

// SendFormFieldEvent sends an event for a form field.
func (d *Document) SendFormFieldEvent(argsJSON string) error {
    d.office.mu.Lock()
    defer d.office.mu.Unlock()
    if d.closed {
        return ErrClosed
    }
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

    args := `{"type":"dialog","action":"execute","data":{}}`
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

    args := `{"field":"name","action":"changed","value":"John"}`
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

- [ ] **Step 1: Add command values integration test**

Append to `lok/integration_test.go`:

```go
// TestIntegration_CommandValues tests GetCommandValues with a real LOK instance.
// Requires LOK_PATH to be set.
func TestIntegration_CommandValues(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test in short mode")
    }
    lokPath := os.Getenv("LOK_PATH")
    if lokPath == "" {
        t.Skip("LOK_PATH not set")
    }

    o, err := New(lokPath)
    if err != nil {
        t.Skipf("cannot create Office: %v (LO not installed?)", err)
    }
    defer o.Close()

    // Use a simple test document.
    docPath := "testdata/hello.odt"
    doc, err := o.Load(docPath)
    if err != nil {
        t.Skipf("cannot load %s: %v", docPath, err)
    }
    defer doc.Close()

    // Query a basic command.
    raw, err := doc.GetCommandValues(".uno:Save")
    if err != nil {
        t.Errorf("GetCommandValues(.uno:Save): %v", err)
    } else {
        t.Logf(".uno:Save command values: %s", raw)
        // Should be valid JSON.
        var m map[string]interface{}
        if err := json.Unmarshal(raw, &m); err != nil {
            t.Errorf("GetCommandValues returned invalid JSON: %v", err)
        }
    }

    // Query font names.
    raw, err = doc.GetCommandValues(".uno:FontName")
    if err != nil {
        t.Logf(".uno:FontName not available: %v", err) // may be unsupported
    } else {
        t.Logf(".uno:FontName: %s...", raw[:min(len(raw), 100)])
    }
}
```

- [ ] **Step 2: Add window paint integration test**

Append:

```go
// TestIntegration_WindowPaint tests PaintWindow with a real LOK instance.
func TestIntegration_WindowPaint(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test in short mode")
    }
    lokPath := os.Getenv("LOK_PATH")
    if lokPath == "" {
        t.Skip("LOK_PATH not set")
    }

    o, err := New(lokPath)
    if err != nil {
        t.Skipf("cannot create Office: %v", err)
    }
    defer o.Close()

    doc, err := o.Load("testdata/hello.odt")
    if err != nil {
        t.Skipf("cannot load document: %v", err)
    }
    defer doc.Close()

    // Create a second view to have a window-like surface.
    viewID, err := doc.CreateView()
    if err != nil {
        t.Skipf("cannot create view: %v", err) // may be unsupported
    }
    defer doc.DestroyView(viewID)

    // Try to paint into a window for that view.
    // Note: actual window IDs may not be available without UI events.
    // This test is best-effort: if PaintWindowForView is not supported,
    // we skip.
    buf := make([]byte, 4*200*200)
    err = doc.PaintWindowForView(1, viewID, buf, 200, 200, 1.0)
    if err != nil {
        // Check if it's unsupported.
        var lokErr *LOKError
        if errors.As(err, &lokErr) {
            t.Skipf("PaintWindowForView not supported: %v", err)
        }
        t.Errorf("PaintWindowForView: %v", err)
    } else {
        // Should have produced some non-trivial output.
        nonZero := 0
        for _, b := range buf {
            if b != 0 {
                nonZero++
            }
        }
        if nonZero == 0 {
            t.Error("PaintWindowForView produced all-zero buffer")
        }
    }
}
```

- [ ] **Step 3: Run integration tests (with LOK)**

Run: `LOK_PATH=/usr/lib/libreoffice/program go test -tags=lok_integration -run 'TestIntegration_Command|TestIntegration_Window' ./lok -v`
Expected: PASS or SKIP (if features not available).

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

Open `coverage.html` and verify that uncovered lines are either:
- Trivial error paths that are hard to trigger in unit tests, or
- cgo wrapper lines (excluded from gate).

If coverage < 90%, add tests for the uncovered functions.

- [ ] **Step 3: Run full test suite**

```bash
go test -race ./lok/...
```

Expected: All tests pass, no race conditions.

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

- Add Document.GetCommandValues and CompleteFunction
- Add window event APIs: PostWindow*Event, ResizeWindow
- Add window paint APIs: PaintWindow*, GetFontSubset
- Add dialog/content-control/form-field event helpers
- Extend internal/lokc with C shims for command/window functions
- Wire into backend seam; fakeBackend stubs for testing
- Unit tests for all new APIs
- Integration smoke tests for command values and window paint
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
- [ ] Integration tests pass when `LOK_PATH` is set.
- [ ] `lok` package coverage ≥ 90%.
- [ ] `internal/lokc` coverage ≥ 90% (excluding trivial wrappers).
- [ ] No new lint errors (`go vet`, `gofmt`).
- [ ] Public API matches design spec.
- [ ] Documentation (godoc) added for all new exported symbols.
- [ ] Examples in `cmd/` can be built (Phase 12) using new APIs.

## Notes

- Command names are case-sensitive and must match LOK's `.uno:*` constants.
- Window IDs are allocated by LOK and may be delivered via `EventTypeWindow`
  events; store them for later use in window-specific APIs.
- `PaintWindow*` methods follow the same pointer-safety contract as
  `PaintTileRaw`: buffer is pinned only for the synchronous C call.
- JSON returned by `GetCommandValues` is not parsed by `lok`; callers
  unmarshal into their own types as needed.
- All new methods on `Document` acquire the office mutex to satisfy LOK's
  not-free-threaded requirement.
