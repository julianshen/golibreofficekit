# Phase 8 — Selection & clipboard (design)

**Branch:** `feat/selection-clipboard`
**Status:** approved in brainstorming 2026-04-24
**Predecessor:** Phase 7 (input events, merged in PR #17)

## 1. Goals

Expose LibreOfficeKit's selection and per-view clipboard surface as a
typed Go API on `*lok.Document`:

- Reading the current text selection with caller-chosen MIME type.
- Reading the selection *kind* (none / text / complex), with and without
  copying the selected text.
- Driving the text-selection handles (`START`, `END`, `RESET`) and
  graphic-selection handles (`START`, `END`) at caller-supplied twip
  coordinates.
- Resetting the current selection.
- Blocking UNO commands per view.
- Reading and writing the per-view clipboard as a slice of
  `(mime, bytes)` items, independent of the UNO `.uno:Copy`/`.uno:Paste`
  commands.

Scope decision: **one PR for all eight spec methods plus the
additional `GetSelectionKind` probe** (brainstorming answer A +
question 4b: yes). PR size is expected to land around the
recent-phases norm; the clipboard memory plumbing is the only
component that meaningfully increases the footprint.

Out of scope (deferred to later phases):

- Callbacks / listeners — needed for reliably waiting on
  `.uno:SelectAll` to finish before reading the selection in
  integration tests. Phase 9 (`feat/callbacks`). Phase 8 polls with a
  timeout, same workaround as Phase 7.
- Window text selection (`setWindowTextSelection`). Phase 10
  (`feat/command-values`).
- Paste helper (`pClass->paste`). Already implicitly planned for
  Phase 10 / the curated UNO helpers in Phase 7.

## 2. Architecture

The binding stays layered exactly as in Phases 3–7:

```
lok (public)                  — typed API, close-state, validation
  └─ backend interface        — seam; fakeBackend for unit tests
      └─ realBackend          — forwards to lokc
          └─ internal/lokc    — thin cgo wrappers + C helpers
              └─ LibreOfficeKit C ABI
```

Files added by this phase:

- `internal/lokc/selection.go` — cgo wrappers for the six selection /
  block C entry points. Per-call static C shims, no cross-method state.
- `internal/lokc/selection_test.go` — exercises the Go wrappers against
  the fake LOK handle (`pClass == NULL`) already in place for earlier
  phases. Covers the early-return guards.
- `internal/lokc/clipboard.go` — the two clipboard wrappers plus two C
  helpers (`go_doc_get_clipboard`, `go_doc_set_clipboard`) that own the
  triple-array memory juggling and the matching
  `go_doc_free_clipboard` call.
- `internal/lokc/clipboard_test.go` — exercises the Go side of the
  wrappers via a stubbed `pClass` that simulates `getClipboard`
  returning one item (see §6).
- `lok/selection.go` — typed enums + public `*Document` methods for
  the selection surface.
- `lok/clipboard.go` — `ClipboardItem`, MIME validator, and the two
  public clipboard methods.
- `lok/selection_test.go`, `lok/clipboard_test.go` — unit tests via
  the existing `fakeBackend`.

Files modified:

- `lok/backend.go` — nine new interface methods (see §3).
- `lok/real_backend.go` — nine forwarders into `lokc`.
- `lok/office_test.go` (or the equivalent that hosts `fakeBackend`) —
  fake-backend fields and the matching methods.
- `lok/real_backend_test.go` — smoke calls for the new `realBackend`
  methods against a loaded integration document (guarded by the
  `lok_integration` tag).
- `lok/integration_test.go` — the two happy-path round-trips described
  in §7.
- `lok/errors.go` — new sentinel `ErrUnsupported` for methods whose LOK
  function pointer is NULL (specifically `getSelectionTypeAndText`
  pre-7.4).

## 3. Public API

### 3.1 Types

```go
// SetTextSelectionType picks which text-selection handle is being
// moved. Mirrors LibreOfficeKitSetTextSelectionType.
type SetTextSelectionType int
const (
    SetTextSelectionStart SetTextSelectionType = iota // LOK_SETTEXTSELECTION_START
    SetTextSelectionEnd                               // LOK_SETTEXTSELECTION_END
    SetTextSelectionReset                             // LOK_SETTEXTSELECTION_RESET
)
func (SetTextSelectionType) String() string

// SetGraphicSelectionType picks which graphic-selection handle is
// being moved. Mirrors LibreOfficeKitSetGraphicSelectionType.
type SetGraphicSelectionType int
const (
    SetGraphicSelectionStart SetGraphicSelectionType = iota // LOK_SETGRAPHICSELECTION_START
    SetGraphicSelectionEnd                                  // LOK_SETGRAPHICSELECTION_END
)
func (SetGraphicSelectionType) String() string

// SelectionKind reports what kind of selection is currently active.
// Mirrors LibreOfficeKitSelectionType. LARGE_TEXT is folded into
// Complex — the LOK header notes LARGE_TEXT is "unused (same as
// LOK_SELTYPE_COMPLEX)".
type SelectionKind int
const (
    SelectionKindNone    SelectionKind = iota // LOK_SELTYPE_NONE
    SelectionKindText                         // LOK_SELTYPE_TEXT
    SelectionKindComplex                      // LOK_SELTYPE_COMPLEX / LARGE_TEXT
)
func (SelectionKind) String() string

// ClipboardItem is a single per-view clipboard entry. Data is nil
// when LOK had no data for the requested MimeType (GetClipboard
// preserves request order; unsupported MIME types come back as
// zero-Data entries).
type ClipboardItem struct {
    MimeType string
    Data     []byte
}
```

### 3.2 Selection methods

```go
// GetTextSelection copies the current text selection as mimeType.
// mimeType is typically "text/plain;charset=utf-8" or
// "text/html"; LOK may substitute a different, compatible mime,
// which is returned in usedMime.
func (*Document) GetTextSelection(mimeType string) (text, usedMime string, err error)

// GetSelectionTypeAndText reads the selection kind and text in a
// single LOK call. Requires LibreOffice >= 7.4; returns
// ErrUnsupported otherwise. Callers targeting older LO should use
// GetSelectionKind + GetTextSelection.
func (*Document) GetSelectionTypeAndText(mimeType string) (kind SelectionKind, text, usedMime string, err error)

// GetSelectionKind reports what kind of selection is currently
// active. Cheaper than GetSelectionTypeAndText because no text
// is copied; works on all LO versions we support.
func (*Document) GetSelectionKind() (SelectionKind, error)

// SetTextSelection drags the selection handle of kind typ to the
// document position (x, y) in twips.
func (*Document) SetTextSelection(typ SetTextSelectionType, x, y int64) error

// ResetSelection clears the current selection. Equivalent to
// SetTextSelection(SetTextSelectionReset, 0, 0), but goes through
// LOK's dedicated resetSelection entry point.
func (*Document) ResetSelection() error

// SetGraphicSelection drags a graphic-selection handle at (x, y)
// in twips. The handle at the given coordinate is selected; see
// the LOK header for the 3x3 grid convention.
func (*Document) SetGraphicSelection(typ SetGraphicSelectionType, x, y int64) error

// SetBlockedCommandList blocks the comma-separated set of UNO
// commands (csv) for the given view. Applies to .uno:<name> calls
// made after the block list is set.
func (*Document) SetBlockedCommandList(viewID int, csv string) error
```

### 3.3 Clipboard methods

```go
// GetClipboard reads the per-view clipboard. A nil mimeTypes slice
// asks LOK for every MIME type it offers natively. A non-nil slice
// requests those specific types; unavailable entries come back in
// order with Data == nil so the caller can correlate by index.
// An empty (non-nil) slice is treated the same as nil.
func (*Document) GetClipboard(mimeTypes []string) ([]ClipboardItem, error)

// SetClipboard writes items to the per-view clipboard, replacing
// the current contents. Each item's MimeType must be non-empty and
// may not contain a NUL byte.
func (*Document) SetClipboard(items []ClipboardItem) error
```

## 4. Data flow

### 4.1 `GetClipboard`

```
lok.Document.GetClipboard(mimeTypes)
  ├─ d.mu.Lock(); defer unlock
  ├─ guard: d.closed → ErrClosed
  ├─ validate each mimeType (non-empty, no NUL, <= 256 bytes)
  ├─ backend.DocumentGetClipboard(d.h, mimeTypes)   [interface]
  │     realBackend:
  │       ├─ build []*C.char + NULL terminator via C.CString (nil slice
  │       │  → pass NULL to C helper)
  │       ├─ defer-free each C.CString
  │       ├─ C.go_doc_get_clipboard(d, mimesPtr,
  │       │       &count, &outMimes, &outSizes, &outStreams)
  │       │     → returns int ok
  │       ├─ if ok == 0 → return nil, *LOKError
  │       ├─ for i in [0,count):
  │       │     mime := C.GoString(outMimes[i])
  │       │     data := nil                     // outStreams[i]==NULL
  │       │     if outStreams[i] != NULL:
  │       │         data = C.GoBytes(outStreams[i], C.int(outSizes[i]))
  │       │     items = append(items, ClipboardItem{mime, data})
  │       └─ C.go_doc_free_clipboard(count,
  │             outMimes, outSizes, outStreams)
  └─ return items, nil
```

The C helper owns both the call to `pClass->getClipboard` and the
matched free. Go never allocates through `unsafe` pointer arithmetic
into C storage; the `**char` arrays stay on the C heap for their
entire lifetime.

### 4.2 `SetClipboard`

Symmetric: the C helper allocates three parallel arrays (`char**`
mimes, `size_t*` sizes, `char**` streams), populates them from Go
via `C.CBytes` / `C.CString`, calls `pClass->setClipboard`, and
frees everything before returning. Go passes a contiguous payload
per item plus its length; the helper builds the pointer arrays on
the C side.

### 4.3 Selection getters

`GetTextSelection(mime)` is the straightforward LOK pattern: the C
function returns a `char*` we must free with `lokc`'s `errstr`
helper (extracts string + frees). `usedMime` comes back via a
`char**` out-parameter, also heap-allocated; same free. If
`pClass->getTextSelection` is itself NULL (shouldn't happen on any
supported LO), the wrapper returns `ErrUnsupported`.

`GetSelectionTypeAndText(mime)` uses `pClass->getSelectionTypeAndText`;
pre-7.4 that slot is NULL — we detect this at the lokc layer and
return `ErrUnsupported`.

`GetSelectionKind()` calls the plain `pClass->getSelectionType`
entry point; returns a raw int we cast to `SelectionKind`, folding
`LOK_SELTYPE_LARGE_TEXT` into `SelectionKindComplex` at the cast
site.

### 4.4 Selection setters

`SetTextSelection`, `SetGraphicSelection`, `ResetSelection`,
`SetBlockedCommandList` are fire-and-forget: a single LOK call with
no return. Arguments are plain ints and a CSV string. These take
`d.mu` for the duration of the call.

## 5. Error handling & invariants

- **Closed document.** Every public method returns `ErrClosed` if
  the document is already closed. Checked under `d.mu`.
- **Enum range.** `SetTextSelection` and `SetGraphicSelection`
  reject out-of-range `typ` values with `ErrInvalidOption`. The
  value is checked against the explicit iota ranges defined in
  §3.1; no silent coercion.
- **MIME validation.** `validateMime(s)` (in `lok/clipboard.go`):
  reject empty, longer than 256 bytes, or containing `\0`. Other
  structural checks (e.g. MIME grammar) are left to LOK; over-strict
  validation at the Go boundary would rule out legitimate LOK
  behaviour. The `GetClipboard` / `SetClipboard` paths and
  `GetTextSelection` / `GetSelectionTypeAndText` all run the same
  validator on their mime argument(s).
- **`ErrUnsupported`.** New sentinel in `lok/errors.go`. Returned
  when the relevant LOK function pointer is NULL on the loaded LO.
  Callers can check `errors.Is(err, lok.ErrUnsupported)` and fall
  back (for `GetSelectionTypeAndText`: `GetSelectionKind` +
  `GetTextSelection`).
- **Clipboard ordering.** `GetClipboard([]string{a, b, c})` returns
  exactly three items in that order; unavailable entries are
  `{MimeType: <originally requested>, Data: nil}` so callers can
  `switch items[i].MimeType`. `GetClipboard(nil)` returns LO's
  natively-offered list in LO's order.
- **Threading.** All new methods acquire `d.mu` for the duration of
  the LOK call, matching the Phase 7 contract. LOK is not
  free-threaded; serialising per document is a hard requirement, not
  a nicety.
- **No mutation after close.** The `d.closed` check under `d.mu`
  handles the race with `Close()` at the same layer as Phase 7.

## 6. Unit-test matrix

All `lok` unit tests run against `fakeBackend`. Target: ≥90% line
coverage for the new files, same as prior phases.

Table-driven where possible. Grouped by method:

| Method | Cases |
| --- | --- |
| `GetTextSelection` | happy path; closed → `ErrClosed`; empty mime → `ErrInvalidOption`; mime > 256 bytes → `ErrInvalidOption`; mime with NUL → `ErrInvalidOption`; fake returns `ErrUnsupported` → surfaced. |
| `GetSelectionTypeAndText` | happy path; `SelectionKindText` + `SelectionKindComplex` + `SelectionKindNone` returned correctly (LARGE_TEXT folds to Complex); closed; mime validation; `ErrUnsupported` path. |
| `GetSelectionKind` | returns each of the three kinds; closed; LARGE_TEXT input coerces to Complex. |
| `SetTextSelection` | happy path (args forwarded); closed; out-of-range typ; |
| `ResetSelection` | happy path; closed. |
| `SetGraphicSelection` | happy path; closed; out-of-range typ. |
| `SetBlockedCommandList` | happy path (args forwarded verbatim, including empty csv); closed. LOK's behaviour on empty csv is not asserted here — the unit test only verifies argument forwarding. |
| `GetClipboard` | nil mimes → backend receives nil; empty slice → backend receives nil; one-mime request preserves order; unavailable entry comes back nil-Data; invalid mime rejected; closed; backend error surfaced. |
| `SetClipboard` | happy path (single item); empty items slice is allowed (forwarded as `nInCount == 0`); invalid mime rejected; closed; backend error surfaced. |
| Enum `String()` | each variant + one out-of-range for each of the three new enums. |

Plus a round-trip on `fakeBackend`:
`SetClipboard(items)` → `GetClipboard(nil)` → deep-equal the items.
The fake stores the items in a field and returns them verbatim.

`internal/lokc` tests use the existing calloc'd fake-LOK handle with
`pClass == NULL` to exercise the early-return guards in every
wrapper. `internal/lokc/clipboard_test.go` additionally builds a
one-shot `pClass` in Go memory (heap-allocated `C.LibreOfficeKitDocumentClass`
zeroed out, then specific function pointers assigned to
`//export`ed Go stubs) and calls the wrapper against it, asserting
that `go_doc_get_clipboard` and `go_doc_free_clipboard` agree on the
count and that `GoBytes` returns the expected payload. This is the
same pattern used in earlier phases for non-trivial C helpers.

## 7. Integration tests (`lok_integration` tag)

All integration tests run sequentially under the shared singleton
`Office` + `-p 1 -tags=lok_integration`, in line with the existing
memory rule.

### 7.1 `TestSelectionRoundTrip`

1. Load `testdata/hello.odt` (Writer, contains the text "Hello").
2. Register a no-op document callback via the existing integration
   harness (see memory: Fedora + 24.8 silently drops posted input
   until a callback is hooked).
3. `PostUnoCommand(".uno:SelectAll", "", false)`.
4. Poll `GetSelectionKind()` for up to 2 s; fail with a descriptive
   error if it never leaves `SelectionKindNone`.
5. `text, usedMime, err := GetTextSelection("text/plain;charset=utf-8")`;
   assert `err == nil`, `usedMime` non-empty, `strings.Contains(text, "Hello")`.
6. `kind, text2, _, err := GetSelectionTypeAndText("text/plain;charset=utf-8")`.
   If `errors.Is(err, ErrUnsupported)` (LO < 7.4), `t.Logf` the
   skip reason and move on — the check is capability-gated by
   design, not silenced. Otherwise assert `err == nil`, `kind ==
   SelectionKindText`, `text2 == text`. The broader test does not
   use `t.Skip`; only this single assertion is capability-gated.
7. `ResetSelection()`; poll for `GetSelectionKind() == SelectionKindNone`.

### 7.2 `TestClipboardRoundTrip`

1. Same document as 7.1 (or a fresh load — whichever the harness
   makes cleaner).
2. Craft one `ClipboardItem{MimeType: "text/plain;charset=utf-8",
   Data: []byte("hi")}`; `SetClipboard([]ClipboardItem{...})`.
3. `GetClipboard(nil)`; assert the returned slice contains an entry
   with `MimeType == "text/plain;charset=utf-8"` (LOK may normalise
   the mime) and `string(Data) == "hi"`.
4. `GetClipboard([]string{"text/plain;charset=utf-8", "application/x-nothing"})`;
   assert `len == 2`, `items[0].Data != nil`, `items[1].Data == nil`,
   `items[1].MimeType == "application/x-nothing"`.

### 7.3 Smoke calls (no state assertions)

In the existing `realBackend` integration test, add calls to
`SetTextSelection(SetTextSelectionStart, 0, 0)`,
`SetGraphicSelection(SetGraphicSelectionEnd, 0, 0)`,
`SetBlockedCommandList(0, "")`, `ResetSelection()`. Asserts only
that they don't crash and that the document is still usable
afterwards (e.g. one more `GetSelectionKind()` call succeeds).
Meaningful state verification needs Phase 10 window-geometry to
compute non-trivial (x, y) values; calling out that deferral here
rather than skipping tests silently.

## 8. Coverage & risk notes

- **Coverage budget.** `realBackend`'s new forwarders and the
  `go_doc_*_clipboard` C helpers are not reached under `go test`
  without the `lok_integration` tag. We keep them as thin as
  possible so the Go logic being measured lives in
  `lok/selection.go`, `lok/clipboard.go`, and
  `internal/lokc/clipboard.go`'s Go side. Target total coverage:
  ≥90 %, matching the repo rule.
- **cgo pointer rules.** The only Go→C pointer transfer is for
  input strings (via `C.CString`) and input bytes (via `C.CBytes`
  inside the C helper). We do not pass Go slices or Go pointers
  into C storage. All returned C pointers are copied out with
  `C.GoString` / `C.GoBytes` before the helper frees them. This
  matches the Phase 6 `render.go` comments.
- **Mime normalisation.** LO can (and does) substitute mime types
  — e.g. return `text/plain;charset=utf-8` in response to a
  request for `text/plain`. Tests assert on substrings or on
  normalised keys, not on exact equality, to avoid flaking on
  version bumps.
- **`ErrUnsupported` vs.  `*LOKError`.** `ErrUnsupported` means
  "this LOK entry point is NULL on the loaded build"; it is a
  stable, documented outcome that callers check with `errors.Is`.
  `*LOKError` wraps the LOK-returned error string (non-zero return
  code paths in `getClipboard` / `setClipboard`). They are not
  interchangeable.

## 9. Deferred / open items

None surfaced during brainstorming. All eight spec methods plus
`GetSelectionKind` are in scope for this phase. Selection-handle
state assertions wait for window geometry in Phase 10; this is
called out in the integration tests (§7.3) rather than hidden.
