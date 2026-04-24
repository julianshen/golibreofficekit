# Phase 7 — Input Events Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the input surface of the LOK binding — `PostKeyEvent`, `PostMouseEvent`, `PostUnoCommand` — plus 11 typed UNO helpers (Bold, Italic, Underline, Undo, Redo, Copy, Cut, Paste, SelectAll, InsertPageBreak, InsertTable) and typed bitset enums for mouse buttons and keyboard modifiers.

**Architecture:** Same two-layer split as prior phases. `internal/lokc` gains three trivial void-returning cgo shims (outside coverage gate). `lok` gains two new files: `input.go` (3 core methods + enum types + named key-code constants + bitset `Has`/`String`) and `commands.go` (11 typed helpers). `MouseButton` and `Modifier` are typed `uint16` bitsets with helper methods. `PostMouseEvent` range-checks x/y against `int32` using a new helper that complements the Phase 6 `requireInt32Rect`.

**Tech Stack:** Go 1.23+, cgo, LibreOfficeKit 24.8 C ABI, UNO awt constants from `offapi/com/sun/star/awt/Key.idl` / `MouseButton.idl` / `KeyModifier.idl`.

**Spec:** `docs/superpowers/specs/2026-04-24-phase-7-input-events-design.md`.

### Deviations from spec

None. The spec was reviewed and polished in its own loop before this plan was drafted.

### Branching

`feat/input-events`, branched from `main` after the Phase 7 spec PR merges.

---

## Files

| Path | Role |
|------|------|
| `internal/lokc/input.go` (create) | 3 void cgo wrappers: PostKeyEvent, PostMouseEvent, PostUnoCommand |
| `internal/lokc/input_test.go` (create) | nil-handle + fake-handle no-op tests |
| `lok/input.go` (create) | 3 Document methods + KeyEventType / MouseEventType / MouseButton / Modifier types + keycode constants + requireInt32XY helper |
| `lok/input_test.go` (create) | Unit tests for the 3 core methods + bitset `Has`/`String` |
| `lok/commands.go` (create) | 11 typed UNO helpers on Document |
| `lok/commands_test.go` (create) | Unit tests for the 11 helpers |
| `lok/backend.go` (modify) | 3 new interface methods |
| `lok/real_backend.go` (modify) | 3 forwarders |
| `lok/office_test.go` (modify) | Fake capture fields + 3 fake methods |
| `lok/real_backend_test.go` (modify) | TestRealBackend_InputForwarding |
| `lok/integration_test.go` (modify) | Save-and-inspect subtests in TestIntegration_FullLifecycle |

---

## Task 0: Branch prep

- [ ] **Step 1: Sync main**

  ```bash
  git checkout main && git pull --ff-only && git status --short
  ```

  Expected: clean; main at the Phase 7 spec PR merge.

- [ ] **Step 2: Create branch**

  ```bash
  git checkout -b feat/input-events && git branch --show-current
  ```

  Expected: `feat/input-events`.

---

## Task 1: `internal/lokc` input wrappers (TDD)

**Files:**
- Create: `internal/lokc/input.go`
- Create: `internal/lokc/input_test.go`

### 1.1 Failing tests

- [ ] **Step 1: Create `internal/lokc/input_test.go`**

  ```go
  //go:build (linux && (amd64 || arm64)) || (darwin && (amd64 || arm64))

  package lokc

  import "testing"

  func TestDocumentInput_NilHandleAreNoOps(t *testing.T) {
  	var d DocumentHandle
  	DocumentPostKeyEvent(d, 0, 'a', 0)
  	DocumentPostMouseEvent(d, 0, 100, 100, 1, 1, 0)
  	DocumentPostUnoCommand(d, ".uno:Bold", "", false)
  }

  func TestDocumentInput_FakeHandle_SafeNoOps(t *testing.T) {
  	d := NewFakeDocumentHandle()
  	t.Cleanup(func() { FreeFakeDocumentHandle(d) })
  	DocumentPostKeyEvent(d, 0, 'a', 0)
  	DocumentPostMouseEvent(d, 0, 100, 100, 1, 1, 0)
  	DocumentPostUnoCommand(d, ".uno:Bold", "", false)
  }
  ```

- [ ] **Step 2: Run — red**

  ```bash
  cd /home/julianshen/prj/golibreofficekit && go test ./internal/lokc/... -run TestDocumentInput
  ```

  Expected: compile error `undefined: DocumentPostKeyEvent` (etc).

### 1.2 Implement

- [ ] **Step 3: Create `internal/lokc/input.go`**

  ```go
  //go:build linux || darwin

  package lokc

  /*
  #cgo CFLAGS: -I${SRCDIR}/../../third_party/lok -DLOK_USE_UNSTABLE_API
  #include <stdbool.h>
  #include <stdlib.h>
  #include "LibreOfficeKit/LibreOfficeKit.h"

  static void go_doc_post_key_event(LibreOfficeKitDocument* d, int type, int charCode, int keyCode) {
      if (d == NULL || d->pClass == NULL || d->pClass->postKeyEvent == NULL) return;
      d->pClass->postKeyEvent(d, type, charCode, keyCode);
  }
  static void go_doc_post_mouse_event(LibreOfficeKitDocument* d, int type, int x, int y,
      int count, int buttons, int mods) {
      if (d == NULL || d->pClass == NULL || d->pClass->postMouseEvent == NULL) return;
      d->pClass->postMouseEvent(d, type, x, y, count, buttons, mods);
  }
  static void go_doc_post_uno_command(LibreOfficeKitDocument* d, const char* cmd,
      const char* args, bool notifyWhenFinished) {
      if (d == NULL || d->pClass == NULL || d->pClass->postUnoCommand == NULL) return;
      d->pClass->postUnoCommand(d, cmd, args, notifyWhenFinished);
  }
  */
  import "C"

  import "unsafe"

  // DocumentPostKeyEvent forwards to pClass->postKeyEvent. typ is
  // a LOK_KEYEVENT_* value; charCode is a Unicode code point (0 for
  // non-printables); keyCode is a com::sun::star::awt::Key value
  // (0 for plain characters).
  func DocumentPostKeyEvent(d DocumentHandle, typ, charCode, keyCode int) {
  	if !d.IsValid() {
  		return
  	}
  	C.go_doc_post_key_event(d.p, C.int(typ), C.int(charCode), C.int(keyCode))
  }

  // DocumentPostMouseEvent forwards to pClass->postMouseEvent. typ is
  // LOK_MOUSEEVENT_*; x, y are twip coordinates (fit in C int); count
  // is click count; buttons and mods are OR-ed awt::MouseButton and
  // awt::KeyModifier bitsets.
  func DocumentPostMouseEvent(d DocumentHandle, typ, x, y, count, buttons, mods int) {
  	if !d.IsValid() {
  		return
  	}
  	C.go_doc_post_mouse_event(d.p, C.int(typ), C.int(x), C.int(y),
  		C.int(count), C.int(buttons), C.int(mods))
  }

  // DocumentPostUnoCommand forwards to pClass->postUnoCommand. args
  // may be empty; notifyWhenFinished requests a
  // LOK_CALLBACK_UNO_COMMAND_RESULT on completion (callback wiring
  // lives in a later phase).
  func DocumentPostUnoCommand(d DocumentHandle, cmd, args string, notifyWhenFinished bool) {
  	if !d.IsValid() {
  		return
  	}
  	ccmd := C.CString(cmd)
  	defer C.free(unsafe.Pointer(ccmd))
  	cargs := C.CString(args)
  	defer C.free(unsafe.Pointer(cargs))
  	C.go_doc_post_uno_command(d.p, ccmd, cargs, C.bool(notifyWhenFinished))
  }
  ```

- [ ] **Step 4: Run — green**

  ```bash
  cd /home/julianshen/prj/golibreofficekit && go test -race ./internal/lokc/... -run TestDocumentInput
  ```

  Expected: PASS (2 tests).

- [ ] **Step 5: Coverage gate**

  ```bash
  cd /home/julianshen/prj/golibreofficekit && make cover-gate
  ```

  Expected: ≥ 90.0%.

- [ ] **Step 6: Commit**

  ```bash
  cd /home/julianshen/prj/golibreofficekit
  git add internal/lokc/input.go internal/lokc/input_test.go
  git commit -m "$(cat <<'EOF'
  feat(lokc): add input-event cgo wrappers

  Three 1:1 void-returning vtable wrappers: PostKeyEvent,
  PostMouseEvent, PostUnoCommand. Live outside the coverage gate
  (Phase 5/6 precedent for trivial void forwarders). CString args
  for the UNO command are freed on return.

  Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
  EOF
  )"
  ```

---

## Task 2: `lok` backend seam + fakeBackend

**Files:**
- Modify: `lok/backend.go`
- Modify: `lok/real_backend.go`
- Modify: `lok/office_test.go`
- Modify: `lok/real_backend_test.go`

### Step 1: Extend `lok/backend.go`

- [ ] Append these 3 methods to the `backend` interface:

  ```go
  	DocumentPostKeyEvent(d documentHandle, typ, charCode, keyCode int)
  	DocumentPostMouseEvent(d documentHandle, typ, x, y, count, buttons, mods int)
  	DocumentPostUnoCommand(d documentHandle, cmd, args string, notifyWhenFinished bool)
  ```

### Step 2: Extend `lok/real_backend.go`

- [ ] Append 3 forwarders before the `init()` block (keep `init()` last):

  ```go
  func (realBackend) DocumentPostKeyEvent(d documentHandle, typ, charCode, keyCode int) {
  	lokc.DocumentPostKeyEvent(mustDoc(d).d, typ, charCode, keyCode)
  }
  func (realBackend) DocumentPostMouseEvent(d documentHandle, typ, x, y, count, buttons, mods int) {
  	lokc.DocumentPostMouseEvent(mustDoc(d).d, typ, x, y, count, buttons, mods)
  }
  func (realBackend) DocumentPostUnoCommand(d documentHandle, cmd, args string, notifyWhenFinished bool) {
  	lokc.DocumentPostUnoCommand(mustDoc(d).d, cmd, args, notifyWhenFinished)
  }
  ```

### Step 3: Extend `fakeBackend` in `lok/office_test.go`

- [ ] Append these fields to `fakeBackend` (after the render state block):

  ```go
  	// Input state.
  	lastKeyType     int
  	lastCharCode    int
  	lastKeyCode     int
  	lastMouseType   int
  	lastMouseX      int
  	lastMouseY      int
  	lastMouseCount  int
  	lastMouseButton int
  	lastMouseMods   int
  	lastUnoCmd      string
  	lastUnoArgs     string
  	lastUnoNotify   bool
  ```

- [ ] Append method implementations at the bottom of the file:

  ```go
  func (f *fakeBackend) DocumentPostKeyEvent(_ documentHandle, typ, charCode, keyCode int) {
  	f.lastKeyType = typ
  	f.lastCharCode = charCode
  	f.lastKeyCode = keyCode
  }
  func (f *fakeBackend) DocumentPostMouseEvent(_ documentHandle, typ, x, y, count, buttons, mods int) {
  	f.lastMouseType = typ
  	f.lastMouseX = x
  	f.lastMouseY = y
  	f.lastMouseCount = count
  	f.lastMouseButton = buttons
  	f.lastMouseMods = mods
  }
  func (f *fakeBackend) DocumentPostUnoCommand(_ documentHandle, cmd, args string, notify bool) {
  	f.lastUnoCmd = cmd
  	f.lastUnoArgs = args
  	f.lastUnoNotify = notify
  }
  ```

### Step 4: Extend `lok/real_backend_test.go`

- [ ] Add `TestRealBackend_InputForwarding` after `TestRealBackend_RenderForwarding`:

  ```go
  func TestRealBackend_InputForwarding(t *testing.T) {
  	rb := realBackend{}
  	fakeDocHandle := lokc.NewFakeDocumentHandle()
  	defer lokc.FreeFakeDocumentHandle(fakeDocHandle)
  	rdoc := realDocumentHandle{d: fakeDocHandle}

  	rb.DocumentPostKeyEvent(rdoc, 0, 'a', 0)
  	rb.DocumentPostMouseEvent(rdoc, 0, 100, 100, 1, 1, 0)
  	rb.DocumentPostUnoCommand(rdoc, ".uno:Bold", "", false)
  }
  ```

### Step 5: Run + commit

- [ ] **Run:** `cd /home/julianshen/prj/golibreofficekit && make all && make cover-gate`
  Expected: green; coverage ≥ 90%.

- [ ] **Commit:**

  ```bash
  cd /home/julianshen/prj/golibreofficekit
  git add lok/backend.go lok/real_backend.go lok/office_test.go lok/real_backend_test.go
  git commit -m "$(cat <<'EOF'
  feat(lok): backend seam and fake for input events

  backend interface grows 3 input methods. realBackend forwards
  each to internal/lokc; fakeBackend captures the last event
  tuple for test assertion.

  Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
  EOF
  )"
  ```

---

## Task 3: Enum types + bitsets + keycode constants (TDD)

Why first: pure-Go types, no backend dependency. Lets Tasks 4–5 use them immediately. Written test-first to lock the `Has`/`String` contracts.

**Files:**
- Create: `lok/input.go` (types + constants only; methods arrive in Task 4)
- Create: `lok/input_test.go` (bitset tests)

### 3.1 Failing tests

- [ ] **Step 1: Create `lok/input_test.go`**

  ```go
  //go:build linux || darwin

  package lok

  import "testing"

  func TestMouseButton_Has(t *testing.T) {
  	set := MouseLeft | MouseRight
  	if !set.Has(MouseLeft) {
  		t.Error("expected MouseLeft in set")
  	}
  	if !set.Has(MouseRight) {
  		t.Error("expected MouseRight in set")
  	}
  	if set.Has(MouseMiddle) {
  		t.Error("did not expect MouseMiddle in set")
  	}
  	// Has(multi-bit) returns true only if ALL bits are set.
  	if !set.Has(MouseLeft | MouseRight) {
  		t.Error("expected set to contain MouseLeft|MouseRight")
  	}
  	if set.Has(MouseLeft | MouseMiddle) {
  		t.Error("did not expect set to contain MouseLeft|MouseMiddle")
  	}
  }

  func TestMouseButton_String(t *testing.T) {
  	cases := []struct {
  		in   MouseButton
  		want string
  	}{
  		{0, "(none)"},
  		{MouseLeft, "MouseLeft"},
  		{MouseLeft | MouseRight, "MouseLeft|MouseRight"},
  		{MouseLeft | MouseMiddle | MouseRight, "MouseLeft|MouseRight|MouseMiddle"},
  	}
  	for _, tc := range cases {
  		if got := tc.in.String(); got != tc.want {
  			t.Errorf("%d: got %q, want %q", tc.in, got, tc.want)
  		}
  	}
  }

  func TestModifier_Has(t *testing.T) {
  	set := ModShift | ModMod1
  	if !set.Has(ModShift) {
  		t.Error("expected ModShift in set")
  	}
  	if !set.Has(ModMod1) {
  		t.Error("expected ModMod1 in set")
  	}
  	if set.Has(ModMod2) {
  		t.Error("did not expect ModMod2 in set")
  	}
  	if !set.Has(ModShift | ModMod1) {
  		t.Error("expected set to contain ModShift|ModMod1")
  	}
  }

  func TestModifier_String(t *testing.T) {
  	cases := []struct {
  		in   Modifier
  		want string
  	}{
  		{0, "(none)"},
  		{ModShift, "ModShift"},
  		{ModShift | ModMod1, "ModShift|ModMod1"},
  		{ModShift | ModMod1 | ModMod2 | ModMod3, "ModShift|ModMod1|ModMod2|ModMod3"},
  	}
  	for _, tc := range cases {
  		if got := tc.in.String(); got != tc.want {
  			t.Errorf("%d: got %q, want %q", tc.in, got, tc.want)
  		}
  	}
  }
  ```

- [ ] **Step 2: Run — red**

  ```bash
  cd /home/julianshen/prj/golibreofficekit && go test ./lok/... -run 'TestMouseButton|TestModifier'
  ```

  Expected: compile errors for undefined identifiers.

### 3.2 Implement

- [ ] **Step 3: Create `lok/input.go`** (types + constants only; methods in Task 4):

  ```go
  //go:build linux || darwin

  package lok

  import "strings"

  // KeyEventType mirrors LOK_KEYEVENT_*.
  type KeyEventType int

  const (
  	KeyEventInput KeyEventType = 0 // LOK_KEYEVENT_KEYINPUT
  	KeyEventUp    KeyEventType = 1 // LOK_KEYEVENT_KEYUP
  )

  // MouseEventType mirrors LOK_MOUSEEVENT_*.
  type MouseEventType int

  const (
  	MouseButtonDown MouseEventType = 0 // LOK_MOUSEEVENT_MOUSEBUTTONDOWN
  	MouseButtonUp   MouseEventType = 1 // LOK_MOUSEEVENT_MOUSEBUTTONUP
  	MouseMove       MouseEventType = 2 // LOK_MOUSEEVENT_MOUSEMOVE
  )

  // MouseButton is a UNO awt::MouseButton bitset.
  type MouseButton uint16

  const (
  	MouseLeft   MouseButton = 1
  	MouseRight  MouseButton = 2
  	MouseMiddle MouseButton = 4
  )

  // Has reports whether all bits in other are set in b. b.Has(0) is
  // true by definition.
  func (b MouseButton) Has(other MouseButton) bool {
  	return b&other == other
  }

  // String renders a pipe-separated list of the set bits, or "(none)"
  // when no bits are set. Order: Left, Right, Middle.
  func (b MouseButton) String() string {
  	if b == 0 {
  		return "(none)"
  	}
  	var parts []string
  	if b.Has(MouseLeft) {
  		parts = append(parts, "MouseLeft")
  	}
  	if b.Has(MouseRight) {
  		parts = append(parts, "MouseRight")
  	}
  	if b.Has(MouseMiddle) {
  		parts = append(parts, "MouseMiddle")
  	}
  	return strings.Join(parts, "|")
  }

  // Modifier is a UNO awt::KeyModifier bitset.
  type Modifier uint16

  const (
  	ModShift Modifier = 1
  	ModMod1  Modifier = 2 // Ctrl on Linux/Windows, Cmd on macOS
  	ModMod2  Modifier = 4 // Alt / Option
  	ModMod3  Modifier = 8
  )

  // Has reports whether all bits in other are set in m. m.Has(0) is
  // true by definition.
  func (m Modifier) Has(other Modifier) bool {
  	return m&other == other
  }

  // String renders a pipe-separated list of the set bits, or "(none)"
  // when no bits are set. Order: Shift, Mod1, Mod2, Mod3.
  func (m Modifier) String() string {
  	if m == 0 {
  		return "(none)"
  	}
  	var parts []string
  	if m.Has(ModShift) {
  		parts = append(parts, "ModShift")
  	}
  	if m.Has(ModMod1) {
  		parts = append(parts, "ModMod1")
  	}
  	if m.Has(ModMod2) {
  		parts = append(parts, "ModMod2")
  	}
  	if m.Has(ModMod3) {
  		parts = append(parts, "ModMod3")
  	}
  	return strings.Join(parts, "|")
  }

  // Named key-code constants. A curated subset of
  // com::sun::star::awt::Key (IDL: offapi/com/sun/star/awt/Key.idl).
  // KeyCodeEnter maps to awt::Key::RETURN. Callers needing keys
  // outside this set pass the raw awt::Key int directly.
  const (
  	KeyCodeEnter     = 1280 // awt::Key::RETURN
  	KeyCodeEsc       = 1281 // awt::Key::ESCAPE
  	KeyCodeTab       = 1282 // awt::Key::TAB
  	KeyCodeBackspace = 1283 // awt::Key::BACKSPACE
  	KeyCodeDelete    = 1286 // awt::Key::DELETE
  	KeyCodeUp        = 1024 // awt::Key::UP
  	KeyCodeDown      = 1025 // awt::Key::DOWN
  	KeyCodeLeft      = 1026 // awt::Key::LEFT
  	KeyCodeRight     = 1027 // awt::Key::RIGHT
  	KeyCodeHome      = 1028 // awt::Key::HOME
  	KeyCodeEnd       = 1029 // awt::Key::END
  	KeyCodePageUp    = 1030 // awt::Key::PAGEUP
  	KeyCodePageDown  = 1031 // awt::Key::PAGEDOWN
  )
  ```

- [ ] **Step 4: Run — green**

  ```bash
  cd /home/julianshen/prj/golibreofficekit && go test -race ./lok/... -run 'TestMouseButton|TestModifier' -count=1
  ```

  Expected: 4 tests PASS (2 Has, 2 String).

- [ ] **Step 5: Commit**

  ```bash
  cd /home/julianshen/prj/golibreofficekit
  git add lok/input.go lok/input_test.go
  git commit -m "$(cat <<'EOF'
  feat(lok): input event types and bitset enums

  Introduces KeyEventType, MouseEventType, MouseButton, Modifier
  typed constants. MouseButton and Modifier are uint16 bitsets
  with Has(flag) bool and String() "pipe-separated-or-(none)"
  methods. Also ships the named awt::Key subset
  (KeyCodeEnter/Esc/Tab/Backspace/Delete + arrows + Home/End/
  PageUp/PageDown).

  Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
  EOF
  )"
  ```

---

## Task 4: `Document.PostKeyEvent` + `PostMouseEvent` + `PostUnoCommand` (TDD)

**Files:**
- Modify: `lok/input.go`
- Modify: `lok/input_test.go`

### 4.1 Failing tests

- [ ] **Step 1: Append to `lok/input_test.go`**

  Top of file: add `"errors"` import.

  Append tests:

  ```go
  func TestPostKeyEvent_Forwards(t *testing.T) {
  	fb := &fakeBackend{}
  	_, doc := loadFakeDoc(t, fb)
  	if err := doc.PostKeyEvent(KeyEventInput, 'G', 0); err != nil {
  		t.Fatal(err)
  	}
  	if fb.lastKeyType != int(KeyEventInput) || fb.lastCharCode != 'G' || fb.lastKeyCode != 0 {
  		t.Errorf("got (type=%d, char=%d, key=%d); want (0, 71, 0)",
  			fb.lastKeyType, fb.lastCharCode, fb.lastKeyCode)
  	}
  }

  func TestPostKeyEvent_UsesKeyCodeConstant(t *testing.T) {
  	fb := &fakeBackend{}
  	_, doc := loadFakeDoc(t, fb)
  	if err := doc.PostKeyEvent(KeyEventInput, 0, KeyCodeEnter); err != nil {
  		t.Fatal(err)
  	}
  	if fb.lastCharCode != 0 || fb.lastKeyCode != KeyCodeEnter {
  		t.Errorf("got (char=%d, key=%d); want (0, %d)",
  			fb.lastCharCode, fb.lastKeyCode, KeyCodeEnter)
  	}
  }

  func TestPostMouseEvent_Forwards(t *testing.T) {
  	fb := &fakeBackend{}
  	_, doc := loadFakeDoc(t, fb)
  	if err := doc.PostMouseEvent(MouseButtonDown, 720, 960, 1, MouseLeft, ModShift); err != nil {
  		t.Fatal(err)
  	}
  	if fb.lastMouseType != int(MouseButtonDown) ||
  		fb.lastMouseX != 720 || fb.lastMouseY != 960 ||
  		fb.lastMouseCount != 1 ||
  		fb.lastMouseButton != int(MouseLeft) ||
  		fb.lastMouseMods != int(ModShift) {
  		t.Errorf("fakeBackend state=%+v", fb)
  	}
  }

  func TestPostMouseEvent_RejectsOverflowX(t *testing.T) {
  	_, doc := loadFakeDoc(t, &fakeBackend{})
  	err := doc.PostMouseEvent(MouseMove, 1<<32+1, 0, 0, 0, 0)
  	var lokErr *LOKError
  	if !errors.As(err, &lokErr) || lokErr.Op != "PostMouseEvent" {
  		t.Errorf("want *LOKError{Op: PostMouseEvent}, got %T %v", err, err)
  	}
  }

  func TestPostMouseEvent_RejectsOverflowY(t *testing.T) {
  	_, doc := loadFakeDoc(t, &fakeBackend{})
  	err := doc.PostMouseEvent(MouseMove, 0, 1<<32+1, 0, 0, 0)
  	var lokErr *LOKError
  	if !errors.As(err, &lokErr) || lokErr.Op != "PostMouseEvent" {
  		t.Errorf("want *LOKError{Op: PostMouseEvent}, got %T %v", err, err)
  	}
  }

  func TestPostUnoCommand_Forwards(t *testing.T) {
  	fb := &fakeBackend{}
  	_, doc := loadFakeDoc(t, fb)
  	if err := doc.PostUnoCommand(".uno:Bold", "", false); err != nil {
  		t.Fatal(err)
  	}
  	if fb.lastUnoCmd != ".uno:Bold" || fb.lastUnoArgs != "" || fb.lastUnoNotify {
  		t.Errorf("fakeBackend state=%+v", fb)
  	}
  }

  func TestPostUnoCommand_ForwardsNotifyTrue(t *testing.T) {
  	fb := &fakeBackend{}
  	_, doc := loadFakeDoc(t, fb)
  	if err := doc.PostUnoCommand(".uno:Save", `{"x":1}`, true); err != nil {
  		t.Fatal(err)
  	}
  	if fb.lastUnoCmd != ".uno:Save" || fb.lastUnoArgs != `{"x":1}` || !fb.lastUnoNotify {
  		t.Errorf("fakeBackend state=%+v", fb)
  	}
  }
  ```

- [ ] **Step 2: Run — red**

  ```bash
  cd /home/julianshen/prj/golibreofficekit && go test ./lok/... -run 'TestPost'
  ```

  Expected: undefined `PostKeyEvent`, `PostMouseEvent`, `PostUnoCommand`.

### 4.2 Implement

- [ ] **Step 3: Append to `lok/input.go`**

  Add imports at top: `"fmt"` and `"math"`.

  Append:

  ```go
  // PostKeyEvent posts a keyboard event to the currently active view.
  // charCode is a Unicode code point (0 for non-printables); keyCode
  // is an awt::Key value (0 for plain characters). The caller is
  // responsible for pairing KeyEventInput with a matching KeyEventUp —
  // LOK does not synthesize a release.
  func (d *Document) PostKeyEvent(typ KeyEventType, charCode, keyCode int) error {
  	unlock, err := d.guard()
  	if err != nil {
  		return err
  	}
  	defer unlock()
  	d.office.be.DocumentPostKeyEvent(d.h, int(typ), charCode, keyCode)
  	return nil
  }

  // PostMouseEvent posts a mouse event at twip coordinates (x, y).
  // count is the click count (1 for single, 2 for double, etc.); for
  // MouseMove, callers typically pass 0 but LOK accepts any value.
  // buttons and mods are OR-ed bitsets. Values of x or y outside
  // int32 return *LOKError{Op:"PostMouseEvent"} without invoking LOK.
  func (d *Document) PostMouseEvent(typ MouseEventType, x, y int64, count int, buttons MouseButton, mods Modifier) error {
  	if err := requireInt32XY("PostMouseEvent", x, y); err != nil {
  		return err
  	}
  	unlock, err := d.guard()
  	if err != nil {
  		return err
  	}
  	defer unlock()
  	d.office.be.DocumentPostMouseEvent(d.h, int(typ), int(x), int(y),
  		count, int(buttons), int(mods))
  	return nil
  }

  // PostUnoCommand dispatches a .uno:* command to the active view.
  // argsJSON is LOK's raw JSON args string (may be empty).
  // notifyWhenFinished requests a LOK_CALLBACK_UNO_COMMAND_RESULT —
  // the callback wiring lives in a later phase; passing true here
  // is accepted but produces no visible effect until then.
  func (d *Document) PostUnoCommand(cmd, argsJSON string, notifyWhenFinished bool) error {
  	unlock, err := d.guard()
  	if err != nil {
  		return err
  	}
  	defer unlock()
  	d.office.be.DocumentPostUnoCommand(d.h, cmd, argsJSON, notifyWhenFinished)
  	return nil
  }

  // requireInt32XY returns *LOKError if x or y exceeds int32 range.
  // LOK's postMouseEvent takes C int (32-bit on LP64). Complements
  // requireInt32Rect from render.go.
  func requireInt32XY(op string, x, y int64) error {
  	if x > math.MaxInt32 || x < math.MinInt32 ||
  		y > math.MaxInt32 || y < math.MinInt32 {
  		return &LOKError{Op: op, Detail: fmt.Sprintf("coord out of int32 range: x=%d, y=%d", x, y)}
  	}
  	return nil
  }
  ```

- [ ] **Step 4: Run — green**

  ```bash
  cd /home/julianshen/prj/golibreofficekit && go test -race ./lok/... -run 'TestPost' -count=1
  ```

  Expected: 7 tests PASS.

- [ ] **Step 5: Commit**

  ```bash
  cd /home/julianshen/prj/golibreofficekit
  git add lok/input.go lok/input_test.go
  git commit -m "$(cat <<'EOF'
  feat(lok): Document.PostKeyEvent/PostMouseEvent/PostUnoCommand

  Three core input methods, each via d.guard() for Lock + closed
  check. PostMouseEvent takes int64 x/y for parity with twip
  coords, range-checks to int32 via new requireInt32XY helper
  (complements Phase 6 requireInt32Rect); overflow surfaces as
  *LOKError without invoking LOK. PostUnoCommand forwards
  notifyWhenFinished as-is; callback wiring is Phase 9.

  Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
  EOF
  )"
  ```

---

## Task 5: Typed UNO helpers (TDD)

**Files:**
- Create: `lok/commands.go`
- Create: `lok/commands_test.go`

### 5.1 Failing tests

- [ ] **Step 1: Create `lok/commands_test.go`**

  ```go
  //go:build linux || darwin

  package lok

  import "testing"

  func TestTypedHelpers_DispatchExpectedCommand(t *testing.T) {
  	cases := []struct {
  		name string
  		call func(*Document) error
  		want string
  	}{
  		{"Bold", func(d *Document) error { return d.Bold() }, ".uno:Bold"},
  		{"Italic", func(d *Document) error { return d.Italic() }, ".uno:Italic"},
  		{"Underline", func(d *Document) error { return d.Underline() }, ".uno:Underline"},
  		{"Undo", func(d *Document) error { return d.Undo() }, ".uno:Undo"},
  		{"Redo", func(d *Document) error { return d.Redo() }, ".uno:Redo"},
  		{"Copy", func(d *Document) error { return d.Copy() }, ".uno:Copy"},
  		{"Cut", func(d *Document) error { return d.Cut() }, ".uno:Cut"},
  		{"Paste", func(d *Document) error { return d.Paste() }, ".uno:Paste"},
  		{"SelectAll", func(d *Document) error { return d.SelectAll() }, ".uno:SelectAll"},
  		{"InsertPageBreak", func(d *Document) error { return d.InsertPageBreak() }, ".uno:InsertPageBreak"},
  	}
  	for _, tc := range cases {
  		t.Run(tc.name, func(t *testing.T) {
  			fb := &fakeBackend{}
  			_, doc := loadFakeDoc(t, fb)
  			if err := tc.call(doc); err != nil {
  				t.Fatal(err)
  			}
  			if fb.lastUnoCmd != tc.want {
  				t.Errorf("cmd=%q, want %q", fb.lastUnoCmd, tc.want)
  			}
  			if fb.lastUnoArgs != "" {
  				t.Errorf("args=%q, want empty", fb.lastUnoArgs)
  			}
  			if fb.lastUnoNotify {
  				t.Error("notify=true, want false")
  			}
  		})
  	}
  }

  func TestInsertTable_BuildsExpectedJSON(t *testing.T) {
  	fb := &fakeBackend{}
  	_, doc := loadFakeDoc(t, fb)
  	if err := doc.InsertTable(3, 4); err != nil {
  		t.Fatal(err)
  	}
  	if fb.lastUnoCmd != ".uno:InsertTable" {
  		t.Errorf("cmd=%q", fb.lastUnoCmd)
  	}
  	want := `{"Columns":{"type":"long","value":4},"Rows":{"type":"long","value":3}}`
  	if fb.lastUnoArgs != want {
  		t.Errorf("args=%q, want %q", fb.lastUnoArgs, want)
  	}
  	if fb.lastUnoNotify {
  		t.Error("notify=true, want false")
  	}
  }
  ```

- [ ] **Step 2: Run — red**

  ```bash
  cd /home/julianshen/prj/golibreofficekit && go test ./lok/... -run 'TestTypedHelpers|TestInsertTable'
  ```

  Expected: undefined `Bold`, `Italic`, etc.

### 5.2 Implement

- [ ] **Step 3: Create `lok/commands.go`**

  ```go
  //go:build linux || darwin

  package lok

  import "fmt"

  // Bold toggles bold on the current selection. Equivalent to
  // PostUnoCommand(".uno:Bold", "", false).
  func (d *Document) Bold() error { return d.PostUnoCommand(".uno:Bold", "", false) }

  // Italic toggles italic on the current selection.
  func (d *Document) Italic() error { return d.PostUnoCommand(".uno:Italic", "", false) }

  // Underline toggles underline on the current selection.
  func (d *Document) Underline() error { return d.PostUnoCommand(".uno:Underline", "", false) }

  // Undo reverses the most recent editing action.
  func (d *Document) Undo() error { return d.PostUnoCommand(".uno:Undo", "", false) }

  // Redo re-applies the most recently undone action.
  func (d *Document) Redo() error { return d.PostUnoCommand(".uno:Redo", "", false) }

  // Copy copies the current selection to the system clipboard.
  func (d *Document) Copy() error { return d.PostUnoCommand(".uno:Copy", "", false) }

  // Cut removes the current selection and places it on the clipboard.
  func (d *Document) Cut() error { return d.PostUnoCommand(".uno:Cut", "", false) }

  // Paste inserts the clipboard content at the caret.
  func (d *Document) Paste() error { return d.PostUnoCommand(".uno:Paste", "", false) }

  // SelectAll selects the entire document content.
  func (d *Document) SelectAll() error { return d.PostUnoCommand(".uno:SelectAll", "", false) }

  // InsertPageBreak inserts a page break at the caret.
  func (d *Document) InsertPageBreak() error { return d.PostUnoCommand(".uno:InsertPageBreak", "", false) }

  // InsertTable inserts a table with the given row and column counts
  // at the caret. Builds LOK's awt::Any JSON args internally.
  func (d *Document) InsertTable(rows, cols int) error {
  	args := fmt.Sprintf(`{"Columns":{"type":"long","value":%d},"Rows":{"type":"long","value":%d}}`, cols, rows)
  	return d.PostUnoCommand(".uno:InsertTable", args, false)
  }
  ```

- [ ] **Step 4: Run — green**

  ```bash
  cd /home/julianshen/prj/golibreofficekit && go test -race ./lok/... -run 'TestTypedHelpers|TestInsertTable' -count=1
  ```

  Expected: 11 tests PASS (10 table + 1 InsertTable).

- [ ] **Step 5: Commit**

  ```bash
  cd /home/julianshen/prj/golibreofficekit
  git add lok/commands.go lok/commands_test.go
  git commit -m "$(cat <<'EOF'
  feat(lok): typed UNO helpers for common commands

  Eleven one-liner methods on *Document: Bold, Italic, Underline,
  Undo, Redo, Copy, Cut, Paste, SelectAll, InsertPageBreak,
  InsertTable. Each wraps PostUnoCommand with a fixed .uno:*
  name and empty args (notifyWhenFinished=false).

  InsertTable builds LOK's awt::Any JSON for Rows/Columns
  internally; caller supplies row/column counts.

  Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
  EOF
  )"
  ```

---

## Task 6: After-close table

**Files:**
- Modify: `lok/input_test.go`

### Step 1: Append table test

- [ ] **Step 1: Append to `lok/input_test.go`**

  ```go
  func TestInputMethods_AfterCloseErrors(t *testing.T) {
  	cases := []struct {
  		name string
  		call func(*Document) error
  	}{
  		{"PostKeyEvent", func(d *Document) error { return d.PostKeyEvent(KeyEventInput, 'a', 0) }},
  		{"PostMouseEvent", func(d *Document) error {
  			return d.PostMouseEvent(MouseButtonDown, 0, 0, 1, MouseLeft, 0)
  		}},
  		{"PostUnoCommand", func(d *Document) error { return d.PostUnoCommand(".uno:Bold", "", false) }},
  		{"Bold", func(d *Document) error { return d.Bold() }},
  		{"Italic", func(d *Document) error { return d.Italic() }},
  		{"Underline", func(d *Document) error { return d.Underline() }},
  		{"Undo", func(d *Document) error { return d.Undo() }},
  		{"Redo", func(d *Document) error { return d.Redo() }},
  		{"Copy", func(d *Document) error { return d.Copy() }},
  		{"Cut", func(d *Document) error { return d.Cut() }},
  		{"Paste", func(d *Document) error { return d.Paste() }},
  		{"SelectAll", func(d *Document) error { return d.SelectAll() }},
  		{"InsertPageBreak", func(d *Document) error { return d.InsertPageBreak() }},
  		{"InsertTable", func(d *Document) error { return d.InsertTable(1, 1) }},
  	}
  	for _, tc := range cases {
  		t.Run(tc.name, func(t *testing.T) {
  			_, doc := loadFakeDoc(t, &fakeBackend{})
  			doc.Close()
  			if err := tc.call(doc); !errors.Is(err, ErrClosed) {
  				t.Errorf("want ErrClosed, got %v", err)
  			}
  		})
  	}
  }
  ```

- [ ] **Step 2: Run — green**

  ```bash
  cd /home/julianshen/prj/golibreofficekit && go test -race ./lok/... -run TestInputMethods_AfterCloseErrors -count=1
  ```

  Expected: 14 subtests PASS.

- [ ] **Step 3: Coverage gate**

  ```bash
  cd /home/julianshen/prj/golibreofficekit && make cover-gate
  ```

  Expected: ≥ 90%.

- [ ] **Step 4: Commit**

  ```bash
  cd /home/julianshen/prj/golibreofficekit
  git add lok/input_test.go
  git commit -m "$(cat <<'EOF'
  test(lok): after-close table test for all Phase 7 methods

  Every new public method on Document — 3 core input methods plus
  11 typed UNO helpers — returns ErrClosed when invoked after
  Close(). Mirrors Phase 5's TestPartMethods_AfterCloseErrors
  shape.

  Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
  EOF
  )"
  ```

---

## Task 7: Integration tests (save-and-inspect)

**Files:**
- Modify: `lok/integration_test.go`

Constraint recap (`memory/feedback_lok_singleton_per_process.md`): all subtests share ONE `New`/`Close` pair in `TestIntegration_FullLifecycle`; `LoadFromReader(doc2)` stays last; render block lives between PartPageRectangles and LoadFromReader. The new input block goes AFTER the render block and BEFORE LoadFromReader.

### Step 1: Add imports

- [ ] **Step 1: Modify `lok/integration_test.go` imports**

  Add `"archive/zip"` (alphabetical position at the top, before `"bytes"`).

### Step 2: Insert the input subtests

- [ ] **Step 2: Locate the comment `// LoadFromReader deliberately comes last.`**

  Insert the following IMMEDIATELY BEFORE that comment (i.e. the new block becomes the last thing before LoadFromReader):

  ```go
  	// Input round-trip on doc — save-and-inspect.

  	// Clear the fixture: SelectAll + Cut. (Delete would also work;
  	// Cut is used to exercise clipboard pathways implicitly.)
  	if err := doc.SelectAll(); err != nil {
  		t.Errorf("SelectAll: %v", err)
  	}
  	if err := doc.Cut(); err != nil {
  		t.Errorf("Cut: %v", err)
  	}

  	// Type "Go" via paired KeyInput/KeyUp events.
  	typed := []rune{'G', 'o'}
  	for _, r := range typed {
  		if err := doc.PostKeyEvent(KeyEventInput, int(r), 0); err != nil {
  			t.Errorf("PostKeyEvent(%q, down): %v", r, err)
  		}
  		if err := doc.PostKeyEvent(KeyEventUp, int(r), 0); err != nil {
  			t.Errorf("PostKeyEvent(%q, up): %v", r, err)
  		}
  	}

  	// Save to a distinct file and inspect content.xml for "Go".
  	typedPath := filepath.Join(outDir, "typed.odt")
  	if err := doc.SaveAs(typedPath, "odt", ""); err != nil {
  		t.Fatalf("SaveAs typed.odt: %v", err)
  	}
  	if body := readODTContent(t, typedPath); !strings.Contains(body, "Go") {
  		t.Errorf("content.xml does not contain \"Go\"; body=%q (truncated)", truncate(body, 400))
  	}

  	// Undo three times (covers one-per-character worst case); LO
  	// typically coalesces into a single undo but the loop is robust
  	// either way.
  	for i := 0; i < 3; i++ {
  		if err := doc.Undo(); err != nil {
  			t.Errorf("Undo %d: %v", i, err)
  		}
  	}
  	undonePath := filepath.Join(outDir, "undone.odt")
  	if err := doc.SaveAs(undonePath, "odt", ""); err != nil {
  		t.Fatalf("SaveAs undone.odt: %v", err)
  	}
  	if body := readODTContent(t, undonePath); strings.Contains(body, "Go") {
  		t.Errorf("after Undo, content.xml still contains \"Go\"; body=%q", truncate(body, 400))
  	}

  	// Single mouse click — no callbacks yet (Phase 9), so this just
  	// verifies the path doesn't crash. A left-button click at
  	// (720, 720) twips = (0.5in, 0.5in).
  	if err := doc.PostMouseEvent(MouseButtonDown, 720, 720, 1, MouseLeft, 0); err != nil {
  		t.Errorf("PostMouseEvent down: %v", err)
  	}
  	if err := doc.PostMouseEvent(MouseButtonUp, 720, 720, 1, MouseLeft, 0); err != nil {
  		t.Errorf("PostMouseEvent up: %v", err)
  	}

  	// PostUnoCommand: exercise the bare path with a harmless command.
  	if err := doc.PostUnoCommand(".uno:Deselect", "", false); err != nil {
  		t.Errorf("PostUnoCommand .uno:Deselect: %v", err)
  	}
  ```

### Step 3: Add helper functions

- [ ] **Step 3: Append to `lok/integration_test.go` (after `TestIntegration_FullLifecycle`)**

  ```go
  // readODTContent opens path as a ZIP, extracts content.xml, and
  // returns it as a string. Fails the test on any IO or zip error.
  func readODTContent(t *testing.T, path string) string {
  	t.Helper()
  	zr, err := zip.OpenReader(path)
  	if err != nil {
  		t.Fatalf("zip.OpenReader(%s): %v", path, err)
  	}
  	defer zr.Close()
  	for _, f := range zr.File {
  		if f.Name != "content.xml" {
  			continue
  		}
  		rc, err := f.Open()
  		if err != nil {
  			t.Fatalf("open content.xml: %v", err)
  		}
  		defer rc.Close()
  		body, err := io.ReadAll(rc)
  		if err != nil {
  			t.Fatalf("read content.xml: %v", err)
  		}
  		return string(body)
  	}
  	t.Fatalf("content.xml not found in %s", path)
  	return ""
  }

  // truncate shortens s to at most n runes for error-message compactness.
  func truncate(s string, n int) string {
  	if len(s) <= n {
  		return s
  	}
  	return s[:n] + "…"
  }
  ```

  Also add `"io"` to the imports at the top of the file. Existing imports are `bytes`, `errors`, `os`, `path/filepath`, `strings`, `testing` — after Step 1 you've already prepended `archive/zip`. The alphabetical slot for `io` is BETWEEN `errors` and `os`. Final order: `archive/zip`, `bytes`, `errors`, `io`, `os`, `path/filepath`, `strings`, `testing`.

### Step 4: Run

- [ ] **Step 4: Verify compile and run integration**

  ```bash
  cd /home/julianshen/prj/golibreofficekit && go build -tags=lok_integration ./lok/...
  ```

  Expected: silent success.

  ```bash
  cd /home/julianshen/prj/golibreofficekit && LOK_PATH=/usr/lib64/libreoffice/program make test-integration
  ```

  Expected: all green. Try `/usr/lib/libreoffice/program` on Debian/Ubuntu if the Fedora path is absent.

  If the "Go" assertion fails, inspect `/tmp/...typed.odt/content.xml` manually — LO may have wrapped the text in unexpected namespaces. The substring match is intentionally lax; adjust only if LO 24.8 output genuinely doesn't contain the literal "Go" anywhere in content.xml (highly unlikely).

  If Undo doesn't fully revert with three calls, LO's coalescing may group differently. Increase the loop count to 5 and re-verify — document the change in the commit if needed.

### Step 5: Commit

- [ ] **Step 5: Commit**

  ```bash
  cd /home/julianshen/prj/golibreofficekit
  git add lok/integration_test.go
  git commit -m "$(cat <<'EOF'
  test(lok): integration coverage for Phase 7 input events

  Save-and-inspect flow: SelectAll+Cut → PostKeyEvent "Go" →
  SaveAs typed.odt → assert content.xml contains "Go" →
  Undo×3 → SaveAs undone.odt → assert content.xml no longer
  contains "Go". Also exercises a PostMouseEvent click pair and
  PostUnoCommand(.uno:Deselect) for crash-resistance coverage
  (no callbacks yet — Phase 9).

  Subtests placed AFTER the Phase 6 render block and BEFORE
  LoadFromReader(doc2), respecting the two-docs layout hazard.

  Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
  EOF
  )"
  ```

---

## Task 8: PR

- [ ] **Step 1: Final verification**

  ```bash
  cd /home/julianshen/prj/golibreofficekit && make all && make cover-gate && LOK_PATH=/usr/lib64/libreoffice/program make test-integration
  ```

  Expected: all green, coverage ≥ 90%.

- [ ] **Step 2: Push + PR**

  ```bash
  git push -u origin feat/input-events
  gh pr create --title "feat(lok): Phase 7 — Input events (keyboard, mouse, UNO dispatch)" --body "$(cat <<'EOF'
  ## Summary
  - Adds the input surface per `docs/superpowers/specs/2026-04-24-phase-7-input-events-design.md`.
  - 3 core methods: `PostKeyEvent`, `PostMouseEvent`, `PostUnoCommand`.
  - 11 typed UNO helpers: Bold, Italic, Underline, Undo, Redo, Copy, Cut, Paste, SelectAll, InsertPageBreak, InsertTable.
  - Typed bitset enums `MouseButton` / `Modifier` with `Has(flag)` and `String()`.
  - Named awt::Key constants (Enter/Esc/Tab/Backspace/Delete + arrows + Home/End/PageUp/PageDown).
  - `PostMouseEvent` range-checks x/y against int32 via new `requireInt32XY` (complements Phase 6's `requireInt32Rect`).
  - Integration save-and-inspect: types "Go", saves, asserts content.xml contains "Go"; undoes and asserts it's gone.

  ## Test plan
  - [ ] `make test` — unit tests ≥ 90% coverage gate.
  - [ ] `make cover-gate` — explicit gate passes.
  - [ ] `make test-integration` — real LO save-and-inspect round-trip.

  🤖 Generated with [Claude Code](https://claude.com/claude-code)
  EOF
  )"
  ```

  Expected: PR URL returned.

---

## Out of scope (deferred)

- Window-surface events (`postWindow*`) — Phase 10.
- `LOK_CALLBACK_UNO_COMMAND_RESULT` wiring — Phase 9.
- Selection / clipboard APIs (`getTextSelection`, `setTextSelection`, `getClipboard`, etc.) — Phase 8.
- Full awt::Key catalogue — 13 named; callers pass raw ints for the rest.
- Arg validation on typed helpers (e.g. negative rows/cols) — LO's responsibility.
