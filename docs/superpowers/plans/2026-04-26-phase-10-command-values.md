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

- `lok/windows.go` — window events, paint, resize, font subset stubs.
- `lok/windows_test.go` — unit tests for window APIs.
- `lok/forms.go` — dialog/content-control/form-field event helpers.
- `lok/forms_test.go` — unit tests for form events.
- `internal/lokc/commands.c` — C shims for `getCommandValues`, `completeFunction`.
- `internal/lokc/commands.h` — declarations for above.
- `internal/lokc/windows.c` — C shims for window paint/resize/events.
- `internal/lokc/windows.h` — declarations for above.

Files modified:

- `lok/commands.go` — **append** `GetCommandValues`, `CompleteFunction`, typed helpers.
- `lok/commands_test.go` — **append** unit tests for command values.
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
PostWindowKeyEvent(d documentHandle, windowID uint32, typ int, charCode, keyCode int) error
PostWindowMouseEvent(d documentHandle, windowID uint32, typ int, x, y int64, count int, buttons, mods int) error
PostWindowGestureEvent(d documentHandle, windowID uint32, typ string, x, y, offset int64) error
PostWindowExtTextInputEvent(d documentHandle, windowID uint32, typ int, text string) error
ResizeWindow(d documentHandle, windowID uint32, w, h int) error
PaintWindow(d documentHandle, windowID uint32, buf []byte, x, y, pxW, pxH int) error
PaintWindowDPI(d documentHandle, windowID uint32, buf []byte, x, y, pxW, pxH int, dpiScale float64) error
PaintWindowForView(d documentHandle, windowID uint32, view viewHandle, buf []byte, x, y, pxW, pxH int, dpiScale float64) error
// NOTE: ResetWindow and GetFontSubset are NOT in LOK 24.8 — omitted.
```

- [ ] **Step 2: Extend fakeBackend**

In `lok/office_test.go`, add fields to `fakeBackend`:

```go
// Phase 10: command/window tracking.
lastCommand           string
lastCommandResult     string
getCommandValuesErr  error
lastWindowID          uint32
lastWindowX, lastWindowY int
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

func (f *fakeBackend) CompleteFunction(_ documentHandle, part int, name string) error {
    f.lastCommand = "CompleteFunction:" + name
    return nil
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

The existing `lokc.go` already has a cgo preamble. Add the new headers:

```go
/*
#cgo CFLAGS: -I${SRCDIR}/../../third_party/lok -DLOK_USE_UNSTABLE_API
#include <stdlib.h>
#include "LibreOfficeKit/LibreOfficeKit.h"
#include "callback.h"
#include "commands.h"   // new
#include "windows.h"    // new
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
int loke_post_window_key_event(void* doc, uint32_t window_id, int type, int char_code, int key_code);
int loke_post_window_mouse_event(void* doc, uint32_t window_id, int type, int64_t x, int64_t y, int count, int buttons, int mods);
int loke_post_window_gesture_event(void* doc, uint32_t window_id, const char* typ, int64_t x, int64_t y, int64_t offset);
int loke_post_window_ext_text_input_event(void* doc, uint32_t window_id, int typ, const char* text);
int loke_resize_window(void* doc, uint32_t window_id, int w, int h);

// Window paint — x, y are top-left of source rect in twips
int loke_paint_window(void* doc, uint32_t window_id, void* buf, int x, int y, int px_w, int px_h);
int loke_paint_window_dpi(void* doc, uint32_t window_id, void* buf, int x, int y, int px_w, int px_h, double dpi_scale);
int loke_paint_window_for_view(void* doc, uint32_t window_id, int view_id, void* buf, int x, int y, int px_w, int px_h, double dpi_scale);

// Font subset — NOT in LOK 24.8, stubbed for future
int loke_get_font_subset(void* doc, const char* font_name, char** out, size_t* out_len);

// Reset window — NOT in LOK 24.8, stubbed for future
int loke_reset_window(void* doc, uint32_t window_id);

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

int loke_post_window_key_event(void* doc, uint32_t window_id, int type, int char_code, int key_code) {
    if (!doc) return 0;
    LibreOfficeKitDocument* d = (LibreOfficeKitDocument*)doc;
    if (!d->pClass || !d->pClass->postWindowKeyEvent) return 0;
    return d->pClass->postWindowKeyEvent(d, window_id, type, char_code, key_code), 1;
}

int loke_post_window_mouse_event(void* doc, uint32_t window_id, int type, int64_t x, int64_t y, int count, int buttons, int mods) {
    if (!doc) return 0;
    LibreOfficeKitDocument* d = (LibreOfficeKitDocument*)doc;
    if (!d->pClass || !d->pClass->postWindowMouseEvent) return 0;
    return d->pClass->postWindowMouseEvent(d, window_id, type, x, y, count, buttons, mods), 1;
}

int loke_post_window_gesture_event(void* doc, uint32_t window_id, const char* typ, int64_t x, int64_t y, int64_t offset) {
    if (!doc || !typ) return 0;
    LibreOfficeKitDocument* d = (LibreOfficeKitDocument*)doc;
    if (!d->pClass || !d->pClass->postWindowGestureEvent) return 0;
    return d->pClass->postWindowGestureEvent(d, window_id, typ, x, y, offset), 1;
}

int loke_post_window_ext_text_input_event(void* doc, uint32_t window_id, int typ, const char* text) {
    if (!doc || !text) return 0;
    LibreOfficeKitDocument* d = (LibreOfficeKitDocument*)doc;
    if (!d->pClass || !d->pClass->postWindowExtTextInputEvent) return 0;
    return d->pClass->postWindowExtTextInputEvent(d, window_id, typ, text), 1;
}

int loke_resize_window(void* doc, uint32_t window_id, int w, int h) {
    if (!doc) return 0;
    LibreOfficeKitDocument* d = (LibreOfficeKitDocument*)doc;
    if (!d->pClass || !d->pClass->resizeWindow) return 0;
    return d->pClass->resizeWindow(d, window_id, w, h), 1;
}

int loke_paint_window(void* doc, uint32_t window_id, void* buf, int x, int y, int px_w, int px_h) {
    if (!doc || !buf) return 0;
    LibreOfficeKitDocument* d = (LibreOfficeKitDocument*)doc;
    if (!d->pClass || !d->pClass->paintWindow) return 0;
    return d->pClass->paintWindow(d, window_id, (unsigned char*)buf, x, y, px_w, px_h), 1;
}

int loke_paint_window_dpi(void* doc, uint32_t window_id, void* buf, int x, int y, int px_w, int px_h, double dpi_scale) {
    if (!doc || !buf) return 0;
    LibreOfficeKitDocument* d = (LibreOfficeKitDocument*)doc;
    if (!d->pClass || !d->pClass->paintWindowDPI) return 0;
    return d->pClass->paintWindowDPI(d, window_id, (unsigned char*)buf, x, y, px_w, px_h, dpi_scale), 1;
}

int loke_paint_window_for_view(void* doc, uint32_t window_id, int view_id, void* buf, int x, int y, int px_w, int px_h, double dpi_scale) {
    if (!doc || !buf) return 0;
    LibreOfficeKitDocument* d = (LibreOfficeKitDocument*)doc;
    if (!d->pClass || !d->pClass->paintWindowForView) return 0;
    return d->pClass->paintWindowForView(d, window_id, (unsigned char*)buf, x, y, px_w, px_h, dpi_scale, view_id), 1;
}

int loke_get_font_subset(void* doc, const char* font_name, char** out, size_t* out_len) {
    (void)doc; (void)font_name; (void)out; (void)out_len;
    return 0;  // NOT IMPLEMENTED in LOK 24.8
}

int loke_reset_window(void* doc, uint32_t window_id) {
    (void)doc; (void)window_id;
    return 0;  // NOT IMPLEMENTED in LOK 24.8
}
```

- [ ] **Step 3: Build**

Run: `go build ./internal/lokc`
Expected: compiles cleanly.

---

## Task 4: realBackend forwarders

**Files:**
- `lok/real_backend.go`
- `internal/lokc/callback.go` (add DispatchHandle helpers — already present from Phase 9)

- [ ] **Step 1: Add forwarders in `lok/real_backend.go`**

Before the `var _ backend = realBackend{}` line, add:

```go
// --- Command & window operations (Phase 10) ---

func (realBackend) GetCommandValues(d documentHandle, command string) (string, error) {
    var out *C.char
    var outLen C.size_t
    ok := C.loke_get_command_values(mustDoc(d).d, C.CString(command), &out, &outLen)
    if ok == 0 {
        // getCommandValues returns NULL for unknown commands or errors.
        // LOK has no document-level getError, so we return a generic error.
        return "", ErrUnsupported
    }
    defer C.free(unsafe.Pointer(out))
    return C.GoStringN(out, C.int(outLen)), nil
}

func (realBackend) CompleteFunction(d documentHandle, part int, name string) error {
    ok := C.loke_complete_function(mustDoc(d).d, C.int(part), C.CString(name))
    if ok == 0 {
        // completeFunction is void; 0 means vtable slot missing.
        return ErrUnsupported
    }
    return nil
}

func (realBackend) SendDialogEvent(d documentHandle, windowID uint64, argsJSON string) error {
    cjson := C.CString(argsJSON)
    defer C.free(unsafe.Pointer(cjson))
    // sendDialogEvent on Document takes uint64 window ID.
    // We need a C shim that calls the Document vtable slot.
    ok := C.loke_doc_send_dialog_event(mustDoc(d).d, C.uint64_t(windowID), cjson)
    if ok == 0 {
        return ErrUnsupported
    }
    return nil
}

func (realBackend) SendContentControlEvent(d documentHandle, argsJSON string) error {
    cjson := C.CString(argsJSON)
    defer C.free(unsafe.Pointer(cjson))
    ok := C.loke_doc_send_content_control_event(mustDoc(d).d, cjson)
    if ok == 0 {
        return ErrUnsupported
    }
    return nil
}

func (realBackend) SendFormFieldEvent(d documentHandle, argsJSON string) error {
    cjson := C.CString(argsJSON)
    defer C.free(unsafe.Pointer(cjson))
    ok := C.loke_doc_send_form_field_event(mustDoc(d).d, cjson)
    if ok == 0 {
        return ErrUnsupported
    }
    return nil
}

func (realBackend) PostWindowKeyEvent(d documentHandle, windowID uint32, typ, charCode, keyCode int) error {
    ok := C.loke_post_window_key_event(mustDoc(d).d, C.uint32_t(windowID), C.int(typ), C.int(charCode), C.int(keyCode))
    if ok == 0 {
        return ErrUnsupported
    }
    return nil
}

func (realBackend) PostWindowMouseEvent(d documentHandle, windowID uint32, typ int, x, y int64, count int, buttons, mods int) error {
    ok := C.loke_post_window_mouse_event(mustDoc(d).d, C.uint32_t(windowID), C.int(typ), C.int64_t(x), C.int64_t(y), C.int(count), C.int(buttons), C.int(mods))
    if ok == 0 {
        return ErrUnsupported
    }
    return nil
}

func (realBackend) PostWindowGestureEvent(d documentHandle, windowID uint32, typ string, x, y, offset int64) error {
    ctyp := C.CString(typ)
    defer C.free(unsafe.Pointer(ctyp))
    ok := C.loke_post_window_gesture_event(mustDoc(d).d, C.uint32_t(windowID), ctyp, C.int64_t(x), C.int64_t(y), C.int64_t(offset))
    if ok == 0 {
        return ErrUnsupported
    }
    return nil
}

func (realBackend) PostWindowExtTextInputEvent(d documentHandle, windowID uint32, typ int, text string) error {
    ctext := C.CString(text)
    defer C.free(unsafe.Pointer(ctext))
    ok := C.loke_post_window_ext_text_input_event(mustDoc(d).d, C.uint32_t(windowID), C.int(typ), ctext)
    if ok == 0 {
        return ErrUnsupported
    }
    return nil
}

func (realBackend) ResizeWindow(d documentHandle, windowID uint32, w, h int) error {
    ok := C.loke_resize_window(mustDoc(d).d, C.uint32_t(windowID), C.int(w), C.int(h))
    if ok == 0 {
        return ErrUnsupported
    }
    return nil
}

func (realBackend) PaintWindow(d documentHandle, windowID uint32, buf []byte, x, y, pxW, pxH int) error {
    if err := checkPaintBuf("PaintWindow", buf, pxW, pxH); err != nil {
        return err
    }
    ok := C.loke_paint_window(mustDoc(d).d, C.uint32_t(windowID), unsafe.Pointer(&buf[0]), C.int(x), C.int(y), C.int(pxW), C.int(pxH))
    if ok == 0 {
        return ErrUnsupported
    }
    return nil
}

func (realBackend) PaintWindowDPI(d documentHandle, windowID uint32, buf []byte, x, y, pxW, pxH int, dpiScale float64) error {
    if err := checkPaintBuf("PaintWindowDPI", buf, pxW, pxH); err != nil {
        return err
    }
    ok := C.loke_paint_window_dpi(mustDoc(d).d, C.uint32_t(windowID), unsafe.Pointer(&buf[0]), C.int(x), C.int(y), C.int(pxW), C.int(pxH), C.double(dpiScale))
    if ok == 0 {
        return ErrUnsupported
    }
    return nil
}

func (realBackend) PaintWindowForView(d documentHandle, windowID uint32, view viewHandle, buf []byte, x, y, pxW, pxH int, dpiScale float64) error {
    if err := checkPaintBuf("PaintWindowForView", buf, pxW, pxH); err != nil {
        return err
    }
    // viewHandle is internal; convert to int (ViewID) for LOK.
    ok := C.loke_paint_window_for_view(mustDoc(d).d, C.uint32_t(windowID), C.int(view), unsafe.Pointer(&buf[0]), C.int(x), C.int(y), C.int(pxW), C.int(pxH), C.double(dpiScale))
    if ok == 0 {
        return ErrUnsupported
    }
    return nil
}
```

Also add to `commands.h` the Document-level sendDialogEvent, sendContentControlEvent, sendFormFieldEvent:

```c
// In commands.h, add:
int loke_doc_send_dialog_event(void* doc, uint64_t window_id, const char* args_json);
int loke_doc_send_content_control_event(void* doc, const char* args_json);
int loke_doc_send_form_field_event(void* doc, const char* args_json);
```

And in `commands.c`:

```c
int loke_doc_send_dialog_event(void* doc, uint64_t window_id, const char* args_json) {
    if (!doc || !args_json) return 0;
    LibreOfficeKitDocument* d = (LibreOfficeKitDocument*)doc;
    if (!d->pClass || !d->pClass->sendDialogEvent) return 0;
    d->pClass->sendDialogEvent(d, window_id, args_json);
    return 1;
}
int loke_doc_send_content_control_event(void* doc, const char* args_json) {
    if (!doc || !args_json) return 0;
    LibreOfficeKitDocument* d = (LibreOfficeKitDocument*)doc;
    if (!d->pClass || !d->pClass->sendContentControlEvent) return 0;
    d->pClass->sendContentControlEvent(d, args_json);
    return 1;
}
int loke_doc_send_form_field_event(void* doc, const char* args_json) {
    if (!doc || !args_json) return 0;
    LibreOfficeKitDocument* d = (LibreOfficeKitDocument*)doc;
    if (!d->pClass || !d->pClass->sendFormFieldEvent) return 0;
    d->pClass->sendFormFieldEvent(d, args_json);
    return 1;
}
```

- [ ] **Step 2: Add `checkPaintBuf` import**

`real_backend.go` needs access to `checkPaintBuf`. Since it's in the same package (`lok`), it can call it directly. No change needed.

- [ ] **Step 3: Build**

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

## Task 6: Public API — `lok/windows.go`

**Files:**
- `lok/windows.go`
- `lok/windows_test.go`

- [ ] **Step 1: Implement window event methods**

Create `lok/windows.go`:

```go
//go:build linux || darwin

package lok

import (
    "errors"
)

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

// PostWindowGestureEvent posts a gesture event (pan/zoom) to a window.
func (d *Document) PostWindowGestureEvent(windowID uint32, typ string, x, y, offset int64) error {
    unlock, err := d.guard()
    if err != nil {
        return err
    }
    defer unlock()
    return d.office.be.PostWindowGestureEvent(d.h, windowID, typ, x, y, offset)
}

// PostWindowExtTextInputEvent posts extended text input to a window.
func (d *Document) PostWindowExtTextInputEvent(windowID uint32, typ int, text string) error {
    unlock, err := d.guard()
    if err != nil {
        return err
    }
    defer unlock()
    return d.office.be.PostWindowExtTextInputEvent(d.h, windowID, typ, text)
}

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

- [ ] **Step 2: Implement window paint methods**

Append to `lok/windows.go`:

```go
import "github.com/julianshen/golibreofficekit/internal/lokc"

// PaintWindow paints a window into the provided buffer. The buffer must
// have length 4*pxW*pxH. Returns premultiplied BGRA (same format as PaintTileRaw).
// x, y specify the top-left corner of the source rectangle in twips.
func (d *Document) PaintWindow(windowID uint32, buf []byte, x, y, pxW, pxH int) error {
    unlock, err := d.guard()
    if err != nil {
        return err
    }
    defer unlock()
    return d.office.be.PaintWindow(d.h, windowID, buf, x, y, pxW, pxH)
}

// PaintWindowDPI paints a window with a DPI scale factor.
func (d *Document) PaintWindowDPI(windowID uint32, buf []byte, x, y, pxW, pxH int, dpiScale float64) error {
    unlock, err := d.guard()
    if err != nil {
        return err
    }
    defer unlock()
    return d.office.be.PaintWindowDPI(d.h, windowID, buf, x, y, pxW, pxH, dpiScale)
}

// PaintWindowForView paints a window for a specific view ID.
func (d *Document) PaintWindowForView(windowID uint32, view ViewID, buf []byte, x, y, pxW, pxH int, dpiScale float64) error {
    unlock, err := d.guard()
    if err != nil {
        return err
    }
    defer unlock()
    vh, ok := d.viewHandles[view]
    if !ok {
        return ErrInvalidOption
    }
    return d.office.be.PaintWindowForView(d.h, windowID, vh, buf, x, y, pxW, pxH, dpiScale)
}

// ResetWindow resets a window's internal state.
// NOTE: Not available in LOK 24.8. Always returns ErrUnsupported.
func (d *Document) ResetWindow(windowID uint32) error {
    return ErrUnsupported
}

// GetFontSubset retrieves a subset of a font as a byte slice (SFNT).
// NOTE: Not available in LOK 24.8. Always returns ErrUnsupported.
func (d *Document) GetFontSubset(fontName string) ([]byte, error) {
    return nil, ErrUnsupported
}
```

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

    err := doc.PostWindowKeyEvent(123, KeyEventInput, 'A', 65)
    if err != nil {
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

    err := doc.PostWindowMouseEvent(456, MouseButtonDown, 100, 200, 1, MouseLeft, ModShift)
    if err != nil {
        t.Fatal(err)
    }
}

func TestResizeWindow_InvalidSize(t *testing.T) {
    withFakeBackend(t, &fakeBackend{})
    o, _ := New("/install")
    defer o.Close()
    doc, _ := o.Load("/tmp/x.odt")
    defer doc.Close()

    err := doc.ResizeWindow(1, -10, 100)
    if !errors.Is(err, ErrInvalidOption) {
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
    err := doc.PaintWindow(1, buf, 0, 0, 100, 100)
    if !errors.Is(err, ErrClosed) {
        t.Errorf("want ErrClosed, got %v", err)
    }
}

func TestGetFontSubset_Unavailable(t *testing.T) {
    withFakeBackend(t, &fakeBackend{})
    o, _ := New("/install")
    defer o.Close()
    doc, _ := o.Load("/tmp/x.odt")
    defer doc.Close()

    _, err := doc.GetFontSubset("Arial")
    if !errors.Is(err, ErrUnsupported) {
        t.Errorf("want ErrUnsupported, got %v", err)
    }
}

func TestResetWindow_Unavailable(t *testing.T) {
    withFakeBackend(t, &fakeBackend{})
    o, _ := New("/install")
    defer o.Close()
    doc, _ := o.Load("/tmp/x.odt")
    defer doc.Close()

    err := doc.ResetWindow(1)
    if !errors.Is(err, ErrUnsupported) {
        t.Errorf("want ErrUnsupported, got %v", err)
    }
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./lok -run 'TestPostWindow|TestResizeWindow|TestPaintWindow|TestGetFontSubset|TestResetWindow' -v`
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

- [ ] **Step 1: Add command values integration test**

Append to `lok/integration_test.go` (reusing existing package-level Office):

```go
// TestIntegration_CommandValues tests GetCommandValues with a real LOK instance.
// Requires LOK_PATH to be set and reuses the package-level Office.
func TestIntegration_CommandValues(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test in short mode")
    }
    if lokPath == "" {
        t.Skip("LOK_PATH not set")
    }
    // Use the package-level Office created in TestIntegration_FullLifecycle
    // (see init or testMain). For simplicity, we create a doc here.
    docPath := "testdata/hello.odt"
    doc, err := testOffice.Load(docPath)
    if err != nil {
        t.Skipf("cannot load %s: %v", docPath, err)
    }
    defer doc.Close()

    raw, err := doc.GetCommandValues(".uno:Save")
    if err != nil {
        t.Errorf("GetCommandValues(.uno:Save): %v", err)
    } else {
        t.Logf(".uno:Save command values: %s", raw)
        var m map[string]interface{}
        if err := json.Unmarshal(raw, &m); err != nil {
            t.Errorf("GetCommandValues returned invalid JSON: %v", err)
        }
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
    if lokPath == "" {
        t.Skip("LOK_PATH not set")
    }

    doc, err := testOffice.Load("testdata/hello.odt")
    if err != nil {
        t.Skipf("cannot load document: %v", err)
    }
    defer doc.Close()

    // Create a second view.
    viewID, err := doc.CreateView()
    if err != nil {
        t.Skipf("cannot create view: %v", err)
    }
    defer doc.DestroyView(viewID)

    // Try PaintWindowForView — may be unsupported, that's OK.
    buf := make([]byte, 4*200*200)
    err = doc.PaintWindowForView(1, viewID, buf, 0, 0, 200, 200, 1.0)
    if err != nil {
        var lokErr *LOKError
        if errors.As(err, &lokErr) {
            t.Skipf("PaintWindowForView not supported: %v", err)
        }
        t.Errorf("PaintWindowForView: %v", err)
    } else {
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

- [ ] **Step 3: Ensure package-level Office is available**

The existing `integration_test.go` already has a `testOffice` pattern. Follow it.

- [ ] **Step 4: Run integration tests**

```bash
GODEBUG=asyncpreemptoff=1 LOK_PATH=/usr/lib/libreoffice/program \
  go test -tags=lok_integration -run 'TestIntegration_Command|TestIntegration_Window' ./lok -v
```

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

Verify uncovered lines are trivial or cgo wrappers.

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
- Add window paint APIs: PaintWindow*, GetFontSubset (stubbed)
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
- Window IDs are `uint32` (LOK uses `unsigned`). `SendDialogEvent` takes
  `uint64` to match LOK's `unsigned long long`.
- `PaintWindow*` methods follow the same pointer-safety contract as
  `PaintTileRaw`: buffer is pinned only for the synchronous C call.
- JSON returned by `GetCommandValues` is not parsed by `lok`; callers
  unmarshal into their own types as needed.
- All new methods on `Document` use `guard()` to acquire the office mutex
  and check both `d.closed` and `d.office.closed`.
- `ResetWindow` and `GetFontSubset` are not available in LOK 24.8 and
  return `ErrUnsupported`.
- `CompleteFunction` is a no-op for non-Calc documents (LOK silently
  ignores it).
