# Phase 7 — Input Events: Design Spec

Date: 2026-04-24
Status: Draft → Review
Owner: julianshen
Parent spec: `2026-04-19-lok-binding-design.md` §Phase 7

## 1. Goal

Add the input surface of the LOK binding: post keyboard events, post
mouse events, dispatch arbitrary UNO commands, and expose typed
convenience wrappers for the most common UNO dispatches. "Done" means
a Go caller can type characters into a loaded document, click at a
twip coordinate, and invoke `.uno:Bold` without assembling JSON.

## 2. Non-goals

- Window-surface events (`postWindowKeyEvent`, `postWindowMouseEvent`,
  `postWindowGestureEvent`, `postWindowExtTextInputEvent`) — Phase 10.
- Asynchronous UNO-command completion callbacks
  (`LOK_CALLBACK_UNO_COMMAND_RESULT`) — Phase 9.
- Selection / clipboard APIs — Phase 8.
- A typed catalogue of every `.uno:*` command — the curated list is
  11 commands (see §5.2); callers needing more use `PostUnoCommand`
  directly.
- Full awt::Key coverage — we expose ~13 named constants for common
  non-printable keys. Callers needing exotic keys pass the raw `int`
  from LibreOffice's awt::Key UNO enum.

## 3. Key decisions

Agreed with the user during brainstorming on 2026-04-24.

1. **UNO helpers are thin method-per-command on `*Document`.** 11
   zero-arg dispatches (Bold, Italic, Underline, Undo, Redo, Copy,
   Cut, Paste, SelectAll, InsertPageBreak) + one arg-taking
   (`InsertTable(rows, cols int)`). Each is a one-liner over
   `PostUnoCommand`. (Answer: A during brainstorm.)
2. **`.uno:Save` is dropped from the curated list.** Phase 3 already
   ships `Document.Save()` calling `documentSave` — the binding's
   save-back semantics. Adding a UNO `Save()` wrapper would collide
   without adding surface. Callers who need `.uno:Save` specifically
   use `PostUnoCommand(".uno:Save", "", false)`.
3. **Typed bitsets for `MouseButton` and `Modifier`, with methods.**
   (Answer: C during brainstorm.) Methods:
   - `(b MouseButton) Has(other MouseButton) bool`
   - `(b MouseButton) String() string` — pipe-separated names
   - `(m Modifier) Has(other Modifier) bool`
   - `(m Modifier) String() string`
4. **Integration tests use save-and-inspect.** (Answer: B during
   brainstorm.) Post keyboard events, `SaveAs` to a temp file,
   open the resulting ODT as a ZIP, and assert `content.xml`
   contains the typed text. Couples the test to ODT's XML layout
   but catches real regressions.

## 4. Key codes and modifiers — values

LOK's `postKeyEvent(type, nCharCode, nKeyCode)` takes two integers:
- `nCharCode` — Unicode code point (0 for non-printable keys).
- `nKeyCode` — LibreOffice `com::sun::star::awt::Key` enum value (0
  for plain characters; non-zero for special keys + modifier bits).

Neither the LOK header nor `LibreOfficeKitEnums.h` defines these
values — they come from the UNO awt module. The binding exposes a
curated subset (§5.1) with values pulled directly from the
`offapi/com/sun/star/awt/Key.idl` IDL file in LibreOffice 24.8.

Similarly `postMouseEvent(type, x, y, count, nButtons, nModifier)`:
- `nButtons` — OR of `com::sun::star::awt::MouseButton`:
  - `LEFT = 1`, `RIGHT = 2`, `MIDDLE = 4`.
- `nModifier` — OR of `com::sun::star::awt::KeyModifier`:
  - `SHIFT = 1`, `MOD1 = 2` (Ctrl on Linux/Windows, Cmd on macOS),
    `MOD2 = 4` (Alt/Option), `MOD3 = 8`.

These values are stable across LO 24.8 and 25.x.

## 5. Public API (`lok` package)

### 5.1 Core input (`lok/input.go`)

```go
// KeyEventType mirrors LOK_KEYEVENT_*.
type KeyEventType int
const (
    KeyEventInput KeyEventType = 0
    KeyEventUp    KeyEventType = 1
)

// MouseEventType mirrors LOK_MOUSEEVENT_*.
type MouseEventType int
const (
    MouseButtonDown MouseEventType = 0
    MouseButtonUp   MouseEventType = 1
    MouseMove       MouseEventType = 2
)

// MouseButton is a UNO awt::MouseButton bitset.
type MouseButton uint16
const (
    MouseLeft   MouseButton = 1
    MouseRight  MouseButton = 2
    MouseMiddle MouseButton = 4
)
func (b MouseButton) Has(other MouseButton) bool
func (b MouseButton) String() string

// Modifier is a UNO awt::KeyModifier bitset.
type Modifier uint16
const (
    ModShift Modifier = 1
    ModMod1  Modifier = 2 // Ctrl on Linux/Windows, Cmd on macOS
    ModMod2  Modifier = 4 // Alt/Option
    ModMod3  Modifier = 8
)
func (m Modifier) Has(other Modifier) bool
func (m Modifier) String() string

// Named key-code constants (a curated subset of awt::Key).
const (
    KeyCodeEnter     = 1280
    KeyCodeEsc       = 1281
    KeyCodeTab       = 1282
    KeyCodeBackspace = 1283
    KeyCodeDelete    = 1286
    KeyCodeUp        = 1024
    KeyCodeDown      = 1025
    KeyCodeLeft      = 1026
    KeyCodeRight     = 1027
    KeyCodeHome      = 1028
    KeyCodeEnd       = 1029
    KeyCodePageUp    = 1030
    KeyCodePageDown  = 1031
)

// PostKeyEvent posts a keyboard event to the currently active view.
// charCode is a Unicode code point (0 for non-printables); keyCode
// is an awt::Key value (0 for plain characters).
//
// The caller is responsible for pairing KeyEventInput with a
// matching KeyEventUp — LOK does not synthesize a release.
func (*Document) PostKeyEvent(typ KeyEventType, charCode, keyCode int) error

// PostMouseEvent posts a mouse event at twip coordinates (x, y).
// count is the click count (1 for single, 2 for double, etc.); use
// 0 for MouseMove. buttons and mods are OR-ed bitsets. x and y are
// int64 for parity with TwipRect; values outside int32 return
// *LOKError{Op:"PostMouseEvent"} without invoking LOK.
func (*Document) PostMouseEvent(typ MouseEventType, x, y int64, count int, buttons MouseButton, mods Modifier) error

// PostUnoCommand dispatches a .uno:* command. argsJSON is LOK's raw
// JSON args string (may be empty). notifyWhenFinished requests a
// LOK_CALLBACK_UNO_COMMAND_RESULT when the command completes — the
// callback is wired in Phase 9; setting true in Phase 7 is accepted
// but produces no visible effect until then.
func (*Document) PostUnoCommand(cmd, argsJSON string, notifyWhenFinished bool) error
```

### 5.2 Typed UNO helpers (`lok/commands.go`)

Each zero-arg helper is one line: `PostUnoCommand(".uno:<Name>", "", false)`.

```go
func (*Document) Bold() error
func (*Document) Italic() error
func (*Document) Underline() error
func (*Document) Undo() error
func (*Document) Redo() error
func (*Document) Copy() error
func (*Document) Cut() error
func (*Document) Paste() error
func (*Document) SelectAll() error
func (*Document) InsertPageBreak() error

// InsertTable dispatches .uno:InsertTable with rows/cols args.
// Builds `{"Columns":{"type":"long","value":N},"Rows":{"type":"long","value":M}}`
// internally; callers need not construct the JSON.
func (*Document) InsertTable(rows, cols int) error
```

## 6. Backend seam additions (`lok/backend.go`)

Three methods. The seam uses `int` for coordinates (LOK takes C `int`
= int32 on LP64); the public methods take `int64` where sensible and
range-check before handing off.

```go
DocumentPostKeyEvent(d documentHandle, typ, charCode, keyCode int)
DocumentPostMouseEvent(d documentHandle, typ, x, y, count, buttons, mods int)
DocumentPostUnoCommand(d documentHandle, cmd, args string, notifyWhenFinished bool)
```

## 7. Internal cgo layer (`internal/lokc/input.go`)

Three thin void-returning shims, NULL-guarded and `CString`-freed:

```c
static void go_doc_post_key_event(LibreOfficeKitDocument* d, int type, int charCode, int keyCode);
static void go_doc_post_mouse_event(LibreOfficeKitDocument* d, int type, int x, int y, int count, int buttons, int mods);
static void go_doc_post_uno_command(LibreOfficeKitDocument* d, const char* cmd, const char* args, bool notifyWhenFinished);
```

All three stay OUTSIDE the coverage gate — they're trivial void
forwarders, matching the Phase 5/6 pattern (`internal/lokc/render.go`
for the void shims).

## 8. Error handling

- Every public method uses `Document.guard()` for lock + closed check
  (returns `ErrClosed` on a closed doc or office).
- `PostMouseEvent` range-checks x/y against `[math.MinInt32, math.MaxInt32]`
  and returns `*LOKError{Op:"PostMouseEvent"}` on overflow — same
  pattern as `requireInt32Rect` from Phase 6.
- `PostKeyEvent` does NOT range-check; charCode and keyCode are
  already `int` (typically ≤ 64K). A future bug that passed
  `math.MaxInt64` would truncate silently — acceptable tradeoff given
  the narrow realistic range and the test coverage.
- `PostUnoCommand` does not validate that `cmd` starts with `.uno:` —
  that's LOK's responsibility, and caller-constructed commands are
  common enough that a prefix check would be gratuitous.
- Typed helpers have no arg validation beyond what `PostUnoCommand`
  offers. `InsertTable(rows, cols)` passes any int through — negative
  or zero values reach LOK, which will refuse them silently.

## 9. Testing strategy

### 9.1 Unit tests (`fakeBackend`, `go test`)

New fields on `fakeBackend`:
- `lastKeyType, lastCharCode, lastKeyCode int`
- `lastMouseType, lastMouseX, lastMouseY, lastMouseCount, lastMouseButtons, lastMouseMods int`
- `lastUnoCmd, lastUnoArgs string`
- `lastUnoNotify bool`

For each of the 3 core methods + 11 typed helpers:
- Happy-path assertion (correct values forwarded).
- Closed-doc → `ErrClosed` (table test `TestInputMethods_AfterCloseErrors`).

Additional unit tests:
- `MouseButton.Has` / `MouseButton.String` — table of combinations.
- `Modifier.Has` / `Modifier.String` — table of combinations.
- `PostMouseEvent` with x or y outside int32 → `*LOKError`.
- `PostUnoCommand` with `notifyWhenFinished=true` forwards the bool.
- `InsertTable(3, 4)` produces exactly
  `{"Columns":{"type":"long","value":4},"Rows":{"type":"long","value":3}}`.

### 9.2 Integration tests (`lok_integration`, save-and-inspect)

Placed in `TestIntegration_FullLifecycle` after the Phase 6 render
block and before `LoadFromReader(doc2)` — respects the two-docs +
DestroyView layout hazard.

Sequence:
1. `doc.SelectAll()` → `doc.Cut()` (clear the fixture).
2. Post-key sequence to type `"Go"`:
   - `PostKeyEvent(KeyEventInput, 'G', 0)` + `PostKeyEvent(KeyEventUp, 'G', 0)`.
   - Same for `'o'`.
3. `SaveAs` to `filepath.Join(outDir, "typed.odt")`.
4. Open the resulting `.odt` as a ZIP (`archive/zip`), read `content.xml`,
   assert the bytes contain `"Go"`.
5. `doc.Undo()` three times (Paste + 'o' + 'G'), `SaveAs` to
   `"undone.odt"`, assert `content.xml` does NOT contain the injected
   text. (Three undos cover the worst case; LO coalesces
   character-typing into a single undo step in practice, but the test
   is robust either way.)
6. Post a single mouse click at `(720, 720)` (1/2 inch in twips) with
   `MouseLeft` button, count=1 — just verifies no crash. With no
   callbacks yet, we can't assert cursor movement at this layer;
   that's Phase 9's job.

The test does NOT attempt to verify PostUnoCommand with
`notifyWhenFinished=true` via a callback — that's explicitly Phase 9.

### 9.3 Coverage

`lok` package ≥ 90% (enforced since Phase 2). The three core methods,
11 helpers, and two bitset methods add ~150 LOC in `lok/`, all
unit-testable via the fake. The `internal/lokc/input.go` wrappers are
3 void forwarders, excluded from the coverage gate per precedent.

## 10. File plan

Create:
- `lok/input.go` — 3 core methods + enum types + key-code constants + bitset methods.
- `lok/input_test.go` — unit tests for the core methods and the bitsets.
- `lok/commands.go` — 11 typed UNO helpers.
- `lok/commands_test.go` — unit tests for the helpers (including the InsertTable JSON).
- `internal/lokc/input.go` — 3 cgo shims.
- `internal/lokc/input_test.go` — nil-handle + fake-handle no-op tests.

Modify:
- `lok/backend.go` — 3 new interface methods.
- `lok/real_backend.go` — 3 forwarders.
- `lok/office_test.go` — 10 new capture fields + 3 fake methods.
- `lok/real_backend_test.go` — `TestRealBackend_InputForwarding`.
- `lok/integration_test.go` — new save-and-inspect subtests.

## 11. Out-of-band concerns

**View target.** `postKeyEvent` / `postMouseEvent` / `postUnoCommand`
post to the *currently active view*. Callers who want to target a
specific view call `SetView(id)` first. The binding does not
snapshot-and-restore the view around input calls — callers see the
contract they set up.

**UNO command visibility.** Most `.uno:*` commands target the
currently selected text or cursor position. Typed helpers like
`Bold()` that toggle state assume the caller has an active selection
or the cursor is on the intended content; a bare `Bold()` on an empty
selection still dispatches, but LO may silently ignore.

**`notifyWhenFinished` without callbacks.** Passing `true` when
Phase 9 isn't wired means the event fires with no Go-side observer.
That's a no-op, not an error.

## 12. Risks

| Risk | Mitigation |
|------|------------|
| awt::Key / KeyModifier values drift across LO versions | Values unchanged since LO 6.x; pin to 24.8 and widen the CI matrix once 25.x ships |
| `.uno:InsertTable` JSON format change | JSON construction is a one-liner; unit test asserts the exact output string so a format drift breaks the test, not production |
| Save-and-inspect test couples to ODT XML layout | Reads `content.xml` as a ZIP entry and substring-searches — robust to whitespace / namespace changes |
| `Undo` count in integration test | Three undos covers one-per-character worst case; LO typically coalesces but the test tolerates both |
| `PostKeyEvent` without prior `SetView` | LOK accepts; events reach whatever view is current at load time (view 0). Callers who multi-view must SetView first — documented on `Document.View()` in Phase 4 |

## 13. Acceptance

- `make test` green; `lok` coverage ≥ 90%.
- `make test-integration` green — types "Go" into hello.odt, saves,
  and the saved ODT contains "Go" in content.xml.
- Every LOK function in Phase 7 of the parent spec's §11 coverage
  matrix maps to a Go symbol on `lok.Document`.
