# Go Binding for LibreOfficeKit — Design Spec

Date: 2026-04-19
Status: Draft (rev 2) → Review
Owner: julianshen

## 1. Goal

Provide a Go package, `github.com/julianshen/golibreofficekit`, that binds the
LibreOfficeKit (LOK) C ABI so Go programs can load, render, edit, convert, and
inspect documents supported by LibreOffice (Writer, Calc, Impress, Draw) in
the same process.

"Full implementation" means every function declared on `LibreOfficeKit` and
`LibreOfficeKitDocument` in LibreOffice 24.8's public headers — enumerated in
the coverage matrix in §11 — is reachable from Go, with two exceptions:

- Windows platform support is deferred.
- The "advanced" tier (macros, signing, certificates) ships behind the
  `lok_advanced` build tag and has looser test guarantees.

## 2. Non-goals

- A drop-in replacement for the `soffice` CLI.
- Headless document *authoring* from scratch (use a template).
- A GUI component (LOKDocView/GTK out of scope).
- LibreOffice versions < 24.8.
- Windows. Linux and macOS only for first release; cgo preamble written so
  Windows can be added later.

## 3. Key decisions

Agreed with the user during brainstorming on 2026-04-19.

1. **Module path:** `github.com/julianshen/golibreofficekit`.
2. **Two packages.**
   - `internal/lokc` — thin cgo layer. One exported Go function per LOK C
     function. Strictly C-shaped: returns raw scalars, C strings, or
     integer handles. No JSON parsing, no Go conveniences.
   - `lok` — public, idiomatic Go API. All interpretation (JSON parsing,
     pixel conversion, typed enums, error wrapping) lives here.
3. **Runtime linking.** LOK is loaded with `dlopen(libsofficeapp.so,
   RTLD_LAZY|RTLD_LOCAL)` from the caller-supplied `program/` directory,
   mirroring LibreOffice's own `LibreOfficeKitInit.h`. No build-time
   `-llibreofficekit`. The init symbol is `libreofficekit_hook_2`
   (falls back to `libreofficekit_hook`).
4. **Concurrency.** LOK state is process-global: views mutate
   document-global state, and LOK's thread-safety is serialised per
   process. We therefore use **one `sync.Mutex` on `Office`** that guards
   every LOK entry point — `Document` and `View` borrow it. An
   `UnsafeOffice` escape hatch exposes raw handles for advanced callers
   who serialise themselves.
5. **Callbacks.** One exported C trampoline
   (`//export lokGoCallback`) is the only function LOK calls. It
   synchronously copies the `pPayload` C string into a Go byte slice,
   looks up the Go listener via an integer handle, and hands the copy to
   a dispatch goroutine so the LOK thread is never blocked by user code.
   `pPayload` is never retained beyond the trampoline body. Listeners
   are registered with `AddListener(cb) (cancel func())` returning a
   deregister closure. On `cancel` or `Document.Close` the handle is
   removed from the map; callbacks arriving after removal are dropped
   with a debug counter increment.
6. **Errors.** Typed sentinel errors where the condition is discrete
   (`ErrAlreadyInitialised`, `ErrInstallPathRequired`, `ErrDocClosed`);
   `*LOKError{Code, Detail}` for LOK-returned error strings. No panics
   across the cgo boundary. `errors.Is` / `errors.As` supported.
7. **Testing.** Unit tests use a fake `lokc` injected through an
   unexported `lokBackend` interface defined in `lok`. Integration tests
   live behind `//go:build lok_integration` and skip when `LOK_PATH` is
   unset. Pixel-conversion code is unit-tested standalone (not through
   the fake) with golden byte slices.
8. **Coverage.** `lok` package ≥ 90% enforced in CI. `internal/lokc`'s
   trivial single-statement wrappers are excluded; the dlopen loader
   and error-string helpers stay in the gate because they are real
   logic. Integration tests exercise the excluded wrappers end-to-end.
9. **Deferred tier (`lok_advanced` build tag):** `runMacro`,
   `signDocument`, `insertCertificate`, `addCertificate`,
   `getSignatureState`. Tier-2 coverage: functions exist and handle
   errors, but end-to-end tests only run when `LOK_TEST_CERTS` is set.
   Also called out in §1. `trimMemory` is **not** deferred — it ships
   with `Office` in Phase 2.

## 4. Package layout

```
go.mod
go.sum
CLAUDE.md
README.md
Makefile
.github/workflows/ci.yml

third_party/lok/
  LibreOfficeKit.h
  LibreOfficeKitEnums.h
  LibreOfficeKitInit.h
  LICENSE               # upstream MPL-2.0 notice
  VERSION               # pinned LO version (e.g. 24.8.7.2)

internal/lokc/
  lokc.go                 # package doc + types
  office.go               # init, destroy, getError, getVersionInfo
  document.go             # documentLoad, close, save, getDocumentType ...
  render.go               # paintTile, initializeForRendering ...
  callback_trampoline.go  # //export lokGoCallback + handle registry
  dlopen_unix.go          # build tag linux,darwin
  errstr.go               # copy-and-free helper for C char*
  *_test.go               # loader + errstr unit tests (in coverage gate)

lok/
  lok.go                # package doc
  office.go             # Office, New, Close, Version, OptionalFeatures
  document.go           # Document, Load, Save, Type ...
  view.go               # View, CreateView, SetView ...
  render.go             # PaintTile → *image.NRGBA (+ PaintTileRaw)
  pixels.go             # premul BGRA → NRGBA (pure Go, unit-tested)
  input.go              # PostKeyEvent, PostMouseEvent, PostUnoCommand
  selection.go          # GetTextSelection, SetTextSelection ...
  clipboard.go          # GetClipboard, SetClipboard (per-view)
  events.go             # AddListener, Event, EventType
  commands.go           # GetCommandValues, typed helpers
  windows.go            # PostWindow{Key,Mouse,Gesture,ExtTextInput}Event, ResizeWindow
  forms.go              # SendFormFieldEvent, SendContentControlEvent, SendDialogEvent
  unsafe.go             # UnsafeOffice / UnsafeDocument escape hatches
  errors.go             # error types
  fake_test.go          # fakelok for unit tests
  *_test.go             # unit tests per file
  integration_test.go   # //go:build lok_integration

cmd/lok-render/         # example: render a doc to PNG
cmd/lok-convert/        # example: format conversion

ci/Dockerfile.lok       # LO 24.8 + Go image for integration CI

docs/
  superpowers/specs/    # design docs
```

## 5. Architecture

### 5.1 Boundary diagram

```
┌────────────────────────────────────────────────────────┐
│                    user Go program                     │
└────────────────────────────────────────────────────────┘
                          │  idiomatic Go API
                          ▼
┌────────────────────────────────────────────────────────┐
│  lok (public)                                          │
│   Office / Document / View / Event                     │
│   office-wide mutex, handle registry, error mapping,   │
│   pixel conversion (pure Go), JSON unmarshalling       │
└────────────────────────────────────────────────────────┘
                          │  narrow Go interface (lokBackend)
                          ▼
┌────────────────────────────────────────────────────────┐
│  internal/lokc (cgo)                                   │
│   1:1 wrappers, //export trampoline, dlopen loader     │
└────────────────────────────────────────────────────────┘
                          │  C ABI
                          ▼
┌────────────────────────────────────────────────────────┐
│  libsofficeapp.so  (loaded at runtime via dlopen)      │
└────────────────────────────────────────────────────────┘
```

### 5.2 Loader

`internal/lokc` uses `dlopen(RTLD_LAZY|RTLD_LOCAL)` to open
**`libsofficeapp.so`** inside the caller-supplied `program/` directory,
resolves `libreofficekit_hook_2` via `dlsym` (falling back to
`libreofficekit_hook`), and calls it with the install path to receive a
`LibreOfficeKit*`. Unload is a deliberate no-op — `dlclose` is not
called because LO's static initialisers cannot be re-run cleanly
within the same process, and the OS reclaims the mapping at exit.

### 5.3 Callback trampoline

LOK's `LibreOfficeKitDocumentCallback` is
`void (*)(int nType, const char* pPayload, void* pData)` and may be
invoked from a LO thread that is not a Go-runtime thread. The exported
trampoline:

1. Reads `nType`, copies `pPayload` to a Go byte slice (`C.GoBytes`)
   **synchronously, before returning**. `pPayload` is never retained.
2. Casts `pData` to a `uint64` handle.
3. Looks up the handle in a package-level
   `sync.Map[uint64]chan<- Event`.
4. Sends the `Event{Type, Payload}` on a buffered channel. A worker
   goroutine started by `AddListener` drains the channel and invokes
   the user callback.

The LOK thread is therefore never blocked by user Go code and never
sees anything other than an integer handle. When a listener is
cancelled (or the `Document` closes), the handle is removed from the
map and the channel is closed; events that race in after removal hit
the `sync.Map` miss path, increment a `droppedCallbacks` counter, and
return. The counter is published via `expvar` under
`lok.dropped_callbacks` so operators can alert on unexpected drops.

### 5.4 Error mapping

LOK reports errors through `char* LibreOfficeKit::getError()` and
`char* LibreOfficeKitDocument::getError()`. Every wrapper that can fail
calls `getError` on a non-success return, copies the string with
`C.GoString`, frees it with `free` (or LOK's `freeError`), and returns
`*LOKError{Code, Detail}`. Known codes get sentinels
(`ErrLoadFailed`, `ErrSaveFailed`, ...) so callers can `errors.Is`.

### 5.5 Concurrency

- **Office-wide mutex.** LOK is a process-global singleton and view
  state is shared across all `Document`s derived from one `Office`; a
  per-document mutex is therefore insufficient. `Office` owns a
  `sync.Mutex`; `Document` and `View` lock it on every LOK entry
  point.
- A second mutex guards `lok.New` / `Office.Close` so a second `New`
  while one is live returns `ErrAlreadyInitialised`.
- `UnsafeOffice` / `UnsafeDocument` expose raw handles without locking
  for callers that want to batch calls under their own lock.
- Callback goroutines never touch LOK directly; if they need to they
  re-enter through the public API, which takes the office mutex.
- **Thread pinning.** Empirically LOK does not require the caller to
  be on any specific OS thread, but its internal GSettings/Cairo
  initialisation on first `libreofficekit_hook_2` can install
  thread-local state. We therefore call `runtime.LockOSThread` for
  the duration of `Office.New` and unlock before returning; subsequent
  calls on any goroutine are safe as long as the office mutex is held.
  The contract is re-stated in the `Office` godoc.

## 6. Implementation phases

Each phase is one feature branch, one PR, red→green→refactor TDD.
Phases are sized so each PR can be reviewed in a single sitting.

Coverage gate timing: `lok` coverage ≥ 90% applies **from Phase 2
onward** (Phases 0–1 predate the `lok` package). The `internal/lokc`
gate — dlopen loader, error-string helper, callback trampoline — is
active from Phase 1. Trivial single-statement cgo wrappers are
excluded throughout.

### Phase 0 — Module scaffold  `chore/scaffold`

- `go.mod` (Go 1.23+), `Makefile` (`build`, `test`, `test-integration`,
  `cover`, `lint`, `fmt`), `go vet` clean, README stub.
- `.github/workflows/ci.yml`: lint + `go test -race` job on
  linux/amd64. No integration job yet.
- `docs/superpowers/specs/` committed.

Acceptance: `make test` green (no production code yet — only a sanity
test); CI green.

### Phase 1 — dlopen loader  `chore/dlopen-loader`

- `internal/lokc` with `dlopen` of `libsofficeapp.so`, symbol resolution
  (`libreofficekit_hook_2` with `libreofficekit_hook` fallback), and
  error-string helper `errstr.go`.
- Unit tests: dlopen of a missing path returns a descriptive error; a
  fake `.so` built at test time (empty) fails symbol resolution
  gracefully. These tests stay in the coverage gate.
- No public `lok` API yet.

Acceptance: loader has ≥ 90% coverage on its unit tests; integration
test (behind tag) successfully `dlopen`s `libsofficeapp.so` from
`$LOK_PATH` and resolves the hook symbol. Actually invoking the hook
to obtain a `LibreOfficeKit*` is deferred to Phase 2.

### Phase 2 — Office lifecycle  `feat/office-lifecycle`

Public API:

```go
type Office struct { /* ... */ }

func New(installPath string) (*Office, error)
func (*Office) Close() error
func (*Office) VersionInfo() (VersionInfo, error)     // parses LO's JSON in lok, not lokc
func (*Office) SetOptionalFeatures(feat OptionalFeatures) error
func (*Office) SetAuthor(name string) error
func (*Office) TrimMemory(target int) error
func (*Office) DumpState() (string, error)
func (*Office) SetDocumentPassword(url, password string) error   // runtime re-prompt, before next Load
```

Tests: second `New` returns `ErrAlreadyInitialised`; `Close` is
idempotent; `VersionInfo` parses fixture JSON; fake injects error
strings that surface as `*LOKError`.

### Phase 3 — Document load / save  `feat/document-load-save`

```go
type Document struct { /* ... */ }
type DocumentType int   // Text, Spreadsheet, Presentation, Drawing, Other

func (*Office) Load(path string, opts ...LoadOption) (*Document, error)
func (*Office) LoadFromReader(r io.Reader, filter string, opts ...LoadOption) (*Document, error)
func (*Document) Type() DocumentType
func (*Document) Save() error
func (*Document) SaveAs(path, format, filterOpts string) error
func (*Document) Close() error
```

`LoadOption` covers password, read-only, lang, macro-security,
batch-mode, repair. `ctx context.Context` is **not** on Load/Save: LOK
is synchronous and not cancellable, and a `ctx` parameter that cannot
cancel is a lie. `LoadFromReader` streams the reader to a temp file
under `os.TempDir()`; the file is deleted in `Document.Close`, so its
lifetime equals the document's. Runtime password re-prompt lives on
`Office.SetDocumentPassword(url, password)` (Phase 2) — the LOK
method is office-scoped and takes a URL.

### Phase 4 — Views  `feat/views`

Views come before rendering because `SetClientVisibleArea` and almost
every paint scenario needs at least one view.

```go
type ViewID int
func (*Document) CreateView() (ViewID, error)
func (*Document) CreateViewWithOptions(opts string) (ViewID, error)
func (*Document) DestroyView(ViewID) error
func (*Document) SetView(ViewID) error
func (*Document) View() ViewID
func (*Document) Views() []ViewID
func (*Document) SetViewLanguage(ViewID, lang string) error
func (*Document) SetViewReadOnly(ViewID, ro bool) error
func (*Document) SetAccessibilityState(ViewID, enabled bool) error
```

### Phase 5 — Parts & sizing  `feat/parts-and-size`

```go
type TwipRect struct{ X, Y, W, H int64 }   // twips are 1/1440 inch

func (*Document) Parts() int
func (*Document) Part() int
func (*Document) SetPart(n int, allowDuplicate bool) error
func (*Document) PartName(n int) string
func (*Document) PartHash(n int) string
func (*Document) PartInfo(n int) (json.RawMessage, error)
func (*Document) DocumentSize() (widthTwips, heightTwips int64)
func (*Document) PartPageRectangles() []TwipRect
func (*Document) SetOutlineState(column, level int, hidden bool) error
```

### Phase 6 — Rendering  `feat/rendering`

```go
func (*Document) InitializeForRendering(args string) error
func (*Document) SetClientZoom(pxPerTwipX, pxPerTwipY, tileWidthTwips, tileHeightTwips int) error
func (*Document) SetClientVisibleArea(r TwipRect) error

// Zero-copy: fills the caller's buffer. len(buf) must == 4*pxW*pxH.
// Returns premultiplied BGRA (Cairo ARGB32, little-endian byte order B,G,R,A).
func (*Document) PaintTileRaw(buf []byte, pxW, pxH int, r TwipRect) error
func (*Document) PaintPartTileRaw(buf []byte, part, pxW, pxH int, r TwipRect) error

// Convenience: allocates, calls PaintTileRaw, unpremultiplies into NRGBA.
func (*Document) PaintTile(pxW, pxH int, r TwipRect) (*image.NRGBA, error)
func (*Document) PaintPartTile(part, pxW, pxH int, r TwipRect) (*image.NRGBA, error)

func (*Document) RenderSearchResult(query string) ([]byte, error)
func (*Document) RenderShapeSelection() ([]byte, error)
```

Pixel format details (important):

- LOK writes **premultiplied BGRA** (Cairo `ARGB32`, little-endian byte
  order B, G, R, A with alpha pre-multiplied into RGB).
- `PaintTileRaw` returns the raw premultiplied BGRA so callers that
  feed Cairo/Skia/WebRender avoid a round-trip.
- `PaintTile` unpremultiplies (`c/α` per channel, clamped) and swizzles
  to straight RGBA in an `*image.NRGBA` (NRGBA = non-premultiplied).
  The conversion is pure Go in `pixels.go` and unit-tested with golden
  byte slices — no cgo needed.

cgo pointer rules: `PaintTileRaw` passes the caller's buffer to a
**single, synchronous** `paintTile` cgo call. LOK does not retain the
pointer past that call. The Go runtime pins the backing array for the
duration of the call. **The pointer must not be stored by C or by our
code in any long-lived structure, and must not be handed to the
callback path.** Comments in `render.go` enforce this.

### Phase 7 — Input events  `feat/input-events`

```go
type MouseButtons uint16
type Modifiers   uint16
type MouseEventType int
type KeyEventType   int

func (*Document) PostKeyEvent(typ KeyEventType, charCode, keyCode int) error
func (*Document) PostMouseEvent(typ MouseEventType, x, y int64, count int, buttons MouseButtons, mods Modifiers) error
func (*Document) PostUnoCommand(cmd string, argsJSON string, notifyWhenFinished bool) error
```

Typed helpers for a curated set of common UNO commands (`Save`,
`Bold`, `Italic`, `Underline`, `Undo`, `Redo`, `Copy`, `Cut`, `Paste`,
`SelectAll`, `InsertPageBreak`, `InsertTable`).

### Phase 8 — Selection & clipboard  `feat/selection-clipboard`

```go
func (*Document) GetTextSelection(mimeType string) (text, usedMime string, err error)
func (*Document) GetSelectionTypeAndText(mimeType string) (SelectionType, string, string, error)
func (*Document) SetTextSelection(typ SelectionType, x, y int64) error
func (*Document) ResetSelection() error
func (*Document) SetGraphicSelection(typ int, x, y int64) error
func (*Document) SetBlockedCommandList(listID int, csv string) error

// Per-view clipboard (distinct from copy/paste via UNO).
func (*Document) GetClipboard(mimeTypes []string) ([]ClipboardItem, error)
func (*Document) SetClipboard(items []ClipboardItem) error
```

### Phase 9 — Callbacks / listeners  `feat/callbacks`

```go
type EventType int
type Event struct {
    Type    EventType
    Payload []byte   // synchronously copied from LOK; helpers parse common types
}

// Office-wide and per-document listeners. Returns a cancel closure.
func (*Office)    AddListener(cb func(Event)) (cancel func(), err error)
func (*Document)  AddListener(cb func(Event)) (cancel func(), err error)
```

Multiple listeners compose. Trampoline and handle registry live in
`internal/lokc`; `lok` owns the registered Go closures. Unit tests
invoke the `//export` function directly with synthetic handles to
validate dispatch, cancellation, post-close delivery, and the dropped
counter.

### Phase 10 — Command values & window events  `feat/command-values`

```go
func (*Document) GetCommandValues(command string) (json.RawMessage, error)
func (*Document) SendDialogEvent(windowID uint64, argsJSON string) error
func (*Document) SendContentControlEvent(argsJSON string) error
func (*Document) SendFormFieldEvent(argsJSON string) error
func (*Document) CompleteFunction(part int, name string) error

func (*Document) PostWindowKeyEvent(windowID uint64, typ KeyEventType, charCode, keyCode int) error
func (*Document) PostWindowMouseEvent(windowID uint64, typ MouseEventType, x, y int64, count int, buttons MouseButtons, mods Modifiers) error
func (*Document) PostWindowGestureEvent(windowID uint64, typ string, x, y, offset int64) error
func (*Document) PostWindowExtTextInputEvent(windowID uint64, typ int, text string) error
func (*Document) ResizeWindow(windowID uint64, w, h int) error

func (*Document) GetFontSubset(fontName string) ([]byte, error)

// Window paint family — used for sidebars, popups, dialogs that LOK
// draws into a separate window surface. Mirrors paintTile but for the
// window addressed by windowID.
func (*Document) PaintWindow(windowID uint64, buf []byte, pxW, pxH int) error
func (*Document) PaintWindowDPI(windowID uint64, buf []byte, pxW, pxH int, dpiScale float64) error
func (*Document) PaintWindowForView(windowID uint64, view ViewID, buf []byte, pxW, pxH int, dpiScale float64) error
func (*Document) ResetWindow(windowID uint64) error
```

Typed unmarshal helpers for the most common command payloads
(`.uno:CellCursor`, `.uno:ViewRowColumnHeaders`, etc.) live alongside
`GetCommandValues`.

### Phase 11 — Advanced (behind `lok_advanced`)  `feat/advanced`

```go
func (*Office)    RunMacro(url string) error
func (*Document)  Sign(certPem, keyPem []byte) error
func (*Office)    InsertCertificate(cert, privateKey []byte) error
func (*Office)    AddCertificate(cert []byte) error
func (*Document)  SignatureState() (SignatureState, error)
```

Tier-2 coverage: functions exist and surface errors cleanly; end-to-end
signing tests run only when `LOK_TEST_CERTS` is set (paths to a cert
and key). `TrimMemory` is **not** in this tier — it has no external
dependencies and ships in Phase 2 on `Office`.

### Phase 12 — Examples  `feat/examples`

- `cmd/lok-render`: `lok-render -in file.odt -out page-%02d.png`
  (phases 2–6).
- `cmd/lok-convert`: `lok-convert -in a.docx -out a.pdf` (phases 2–3).
- README with build/install/run instructions.

### Phase 13 — Integration CI  `feat/integration-ci`

- `ci/Dockerfile.lok`: `golang:1.23` + `libreoffice` + vendored headers.
- GitHub Actions integration job builds the image and runs
  `go test -tags=lok_integration -race ./...`.
- Unit-test job uploads coverage (integration job does not, to avoid
  skewing the percentage with LO-only code paths).
- Matrix across LO 24.8 today; LO 25.x added once that release ships.

## 7. Testing strategy

### 7.1 Unit tests (default `go test`)

- `lok` tests inject a fake backend via an unexported `lokBackend`
  interface (defined in `lok`, satisfied by `internal/lokc` in
  production).
- Every exported method has ≥ one happy-path and ≥ one error test.
- Callback trampoline tested by invoking the exported Go function
  directly from tests.
- Pixel conversion unit-tested standalone against golden byte arrays
  (not through the fake).
- `go test -race` is the default in CI.

### 7.2 Integration tests (`-tags=lok_integration`)

- Tiny fixtures in `testdata/` (`hello.odt`, `numbers.ods`, `deck.odp`,
  `drawing.odg`).
- Skip via `t.Skipf("LOK_PATH not set")` so `go test ./...` works
  without LO.
- Cover: load/save round-trip, type detection, paint a tile and assert
  the buffer is non-trivial, post a key and observe a callback, create
  and destroy a view, PDF export via `SaveAs`. Golden-image tests for
  the paint path (small tiles, tolerant comparison).

### 7.3 Coverage

- `lok` target ≥ 90%; CI fails below that.
- `internal/lokc` excludes only the trivial single-statement C
  wrappers. The dlopen loader, error-string helper, and callback
  trampoline stay in the gate.
- `go tool cover -html=coverage.out -o coverage.html` uploaded as a CI
  artifact from the unit-test job only.

## 8. Open questions

1. Do we want `image.RGBA` (premultiplied) in addition to `image.NRGBA`?
   `NRGBA` is the right default for Go (matches `image/png`), but a
   premultiplied path avoids two multiplications per pixel. Decide
   after benchmarking Phase 6.
2. LO 25 is expected late 2026. Pinning 24.8 headers now; when 25
   ships, add a second vendored header directory and select at build
   time with a tag.

## 9. Risks & mitigations

| Risk | Mitigation |
|------|------------|
| LO single-LOK-per-process surprises callers | Document in package doc + `ErrAlreadyInitialised`; integration test asserts |
| cgo pointer-rule violations in callbacks | Integer handles only; synchronous `C.GoBytes` copy; `cgocheck` in CI |
| Tile-buffer pointer stashed somewhere | Review checklist in `render.go`; not handed to callback path |
| Coverage gate blocks progress on tier-3 | Exclude only trivial wrappers; tier-3 behind build tag |
| LO version drift | Pin headers + `VERSION`; CI matrix once multiple LO releases supported |
| LOK thread-safety subtleties | Office-wide mutex; `UnsafeOffice` escape hatch clearly marked |
| Premul/straight pixel confusion | `PaintTileRaw` returns premul with loud doc; `PaintTile` returns straight NRGBA |

## 10. Acceptance

The binding is "done" when, on Linux with LO 24.8 installed:

1. A user can `go get github.com/julianshen/golibreofficekit` and open,
   render, edit, and save each of ODT, DOCX, XLSX, ODS, PPTX, ODP, PDF.
2. `go test ./...` passes without LO installed.
3. `go test -tags=lok_integration -race ./...` passes with LO
   installed.
4. `lok` coverage ≥ 90% on the default test run.
5. `cmd/lok-render` and `cmd/lok-convert` run against their README
   invocations.

## 11. Coverage matrix

Every public LOK 24.8 function must be listed here with the phase that
owns it. This table is the source of truth at PR time; the reviewer
will diff the header against it.

### LibreOfficeKit (office-level)

| LOK function            | Phase | Go symbol                      |
|-------------------------|-------|--------------------------------|
| documentLoad / …WithOptions | 3 | `Office.Load` / `LoadFromReader` |
| freeError               | n/a   | internal (`errstr.go`, §5.4)   |
| getError                | n/a   | internal (`errstr.go`, §5.4)   |
| getVersionInfo          | 2     | `Office.VersionInfo`           |
| setOptionalFeatures     | 2     | `Office.SetOptionalFeatures`   |
| setAuthor               | 2     | `Office.SetAuthor`             |
| registerCallback        | 9     | `Office.AddListener`           |
| getFilterTypes          | 3     | `Office.FilterTypes`           |
| dumpState               | 2     | `Office.DumpState`             |
| runMacro                | 11    | `Office.RunMacro`              |
| signDocument            | 11    | `Office.SignDocument`          |
| insertCertificate       | 11    | `Office.InsertCertificate`     |
| addCertificate          | 11    | `Office.AddCertificate`        |
| trimMemory              | 2     | `Office.TrimMemory`            |
| setDocumentPassword     | 2     | `Office.SetDocumentPassword(url, pwd)` |
| destroy                 | 2     | `Office.Close`                 |

### LibreOfficeKitDocument (per-document)

| LOK function                   | Phase | Go symbol                                |
|--------------------------------|-------|------------------------------------------|
| getDocumentType                | 3     | `Document.Type`                          |
| save / saveAs                  | 3     | `Document.Save` / `Document.SaveAs`      |
| destroy                        | 3     | `Document.Close`                         |
| getParts / getPart / setPart   | 5     | `Parts` / `Part` / `SetPart`             |
| getPartName / getPartHash      | 5     | `PartName` / `PartHash`                  |
| getPartInfo                    | 5     | `PartInfo`                               |
| getDocumentSize                | 5     | `DocumentSize`                           |
| getPartPageRectangles          | 5     | `PartPageRectangles`                     |
| setOutlineState                | 5     | `SetOutlineState`                        |
| initializeForRendering         | 6     | `InitializeForRendering`                 |
| setClientZoom                  | 6     | `SetClientZoom`                          |
| setClientVisibleArea           | 6     | `SetClientVisibleArea`                   |
| paintTile / paintPartTile      | 6     | `PaintTile(Raw)` / `PaintPartTile(Raw)`  |
| renderSearchResult             | 6     | `RenderSearchResult`                     |
| renderShapeSelection           | 6     | `RenderShapeSelection`                   |
| postKeyEvent                   | 7     | `PostKeyEvent`                           |
| postMouseEvent                 | 7     | `PostMouseEvent`                         |
| postUnoCommand                 | 7     | `PostUnoCommand`                         |
| getTextSelection               | 8     | `GetTextSelection`                       |
| getSelectionTypeAndText        | 8     | `GetSelectionTypeAndText`                |
| setTextSelection               | 8     | `SetTextSelection`                       |
| resetSelection                 | 8     | `ResetSelection`                         |
| setGraphicSelection            | 8     | `SetGraphicSelection`                    |
| setBlockedCommandList          | 8     | `SetBlockedCommandList`                  |
| getClipboard / setClipboard    | 8     | `GetClipboard` / `SetClipboard`          |
| createView / destroyView       | 4     | `CreateView(WithOptions)` / `DestroyView` |
| setView / getView / getViewIds | 4     | `SetView` / `View` / `Views`             |
| setViewLanguage                | 4     | `SetViewLanguage`                        |
| setViewReadOnly                | 4     | `SetViewReadOnly`                        |
| setAccessibilityState          | 4     | `SetAccessibilityState`                  |
| registerCallback               | 9     | `Document.AddListener`                   |
| getCommandValues               | 10    | `GetCommandValues`                       |
| sendDialogEvent                | 10    | `SendDialogEvent`                        |
| sendContentControlEvent        | 10    | `SendContentControlEvent`                |
| sendFormFieldEvent             | 10    | `SendFormFieldEvent`                     |
| completeFunction               | 10    | `CompleteFunction`                       |
| postWindowKeyEvent             | 10    | `PostWindowKeyEvent`                     |
| postWindowMouseEvent           | 10    | `PostWindowMouseEvent`                   |
| postWindowGestureEvent         | 10    | `PostWindowGestureEvent`                 |
| postWindowExtTextInputEvent    | 10    | `PostWindowExtTextInputEvent`            |
| resizeWindow                   | 10    | `ResizeWindow`                           |
| getFontSubset                  | 10    | `GetFontSubset`                          |
| paintWindow                    | 10    | `PaintWindow`                            |
| paintWindowDPI                 | 10    | `PaintWindowDPI`                         |
| paintWindowForView             | 10    | `PaintWindowForView`                     |
| resetWindow                    | 10    | `ResetWindow`                            |
| getSignatureState              | 11    | `SignatureState`                         |

Any LOK function that appears in the vendored header but not in this
table must be added (and assigned a phase) before merge.
