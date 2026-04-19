# Go Binding for LibreOfficeKit — Design Spec

Date: 2026-04-19
Status: Draft → Review
Owner: julianshen

## 1. Goal

Provide a Go package, `github.com/julianshen/golibreofficekit`, that binds the
LibreOfficeKit (LOK) C ABI so Go programs can load, render, edit, convert, and
inspect documents supported by LibreOffice (Writer, Calc, Impress, Draw) in
the same process.

"Full implementation" means every function declared on `LibreOfficeKit` and
`LibreOfficeKitDocument` in LibreOffice 24.8's public headers is reachable
from Go, with the exceptions called out in §3.

## 2. Non-goals

- A drop-in replacement for the `soffice` CLI. This binding exposes the
  in-process API only.
- Headless document *authoring* from scratch (use a template document).
- A GUI component. (LOKDocView is a GTK widget; out of scope.)
- Supporting LibreOffice versions older than 24.8. The binding targets the
  current LTS and will follow new releases; back-compat shims are out of
  scope unless a user requests them.
- Windows support in the first release. Linux and macOS only. The cgo
  preamble is written so Windows can be added later.

## 3. Key decisions

These were agreed with the user during brainstorming on 2026-04-19.

1. **Module path:** `github.com/julianshen/golibreofficekit`.
2. **Two packages.**
   - `internal/lokc` — thin cgo layer. One exported Go function per LOK C
     function. No interpretation, no Go conveniences.
   - `lok` — public, idiomatic Go API: `Office`, `Document`, `View` structs;
     methods; `context.Context` on cancellable operations; typed errors;
     `io.Reader`/`io.Writer` at package edges.
3. **Runtime linking.** LOK is loaded with `dlopen` of
   `libreofficekit.so` / `libreofficekitgtk.so` (whichever exports
   `libreofficekit_hook`) via the same code path used in LibreOffice's own
   `LibreOfficeKitInit.h`. No build-time `-llibreofficekit`. Install path
   is supplied by the caller; helpers probe common defaults.
4. **Concurrency.** One `sync.Mutex` per `Document`; a package-level mutex
   guards `Office` lifecycle. The contract — "serialise access to a given
   document, and only one `Office` per process" — is documented loudly. An
   `UnsafeDocument` type exposes the underlying handle for callers who
   want to manage locking themselves.
5. **Callbacks.** One exported C trampoline
   (`//export lokGoCallback`) receives every event and dispatches through
   an integer-keyed handle table to the Go closure registered with
   `Document.OnEvent`. Go pointers are never stored in C memory.
6. **Errors.** Typed sentinel errors where the condition is discrete
   (`ErrAlreadyInitialised`, `ErrDocClosed`); `*LOKError` with `Code` and
   `Detail` fields for LOK-returned error strings. No panics across the
   cgo boundary.
7. **Testing.** Unit tests use a faked `lokc` interface (defined in `lok`,
   satisfied by real `lokc` in production and a `fakelok` in tests).
   Integration tests live behind `//go:build lok_integration` and skip
   when `LOK_PATH` is unset.
8. **Coverage.** `lok` package ≥ 90% enforced in CI
   (`go test -covermode=atomic -coverprofile=coverage.out ./lok/...
   && go tool cover -func=coverage.out | tail -n 1`). `internal/lokc` is
   excluded from the bar because it is mostly cgo plumbing; integration
   tests still exercise it end-to-end.
9. **Functions intentionally deferred** (tier 3, behind build tag
   `lok_advanced`): `runMacro`, `signDocument`, `insertCertificate`,
   `addCertificate`, `getSignatureState`. They are rarely used, hard to
   test without additional infrastructure (signing keys, Basic macros),
   and can be added later without API changes.

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
  lokc.go               # package doc + types
  office.go             # lok_init, destroy, getError, getVersionInfo
  document.go           # documentLoad, close, save, getDocumentType ...
  render.go             # paintTile, initializeForRendering ...
  events.go             # registerCallback, trampoline
  dlopen_unix.go        # build tag linux,darwin
  dlopen_unix_test.go

lok/
  lok.go                # package doc
  office.go             # Office, New, Close
  document.go           # Document, Load, Save, Type ...
  view.go               # View, CreateView, SetView ...
  render.go             # PaintTile → image.RGBA
  input.go              # PostKeyEvent, PostMouseEvent, PostUnoCommand
  selection.go          # GetTextSelection, SetTextSelection ...
  events.go             # OnEvent, Event, EventType
  commands.go           # GetCommandValues, typed helpers
  unsafe.go             # UnsafeDocument escape hatch
  errors.go             # error types
  fake_test.go          # fakelok for unit tests
  *_test.go             # unit tests per file
  integration_test.go   # //go:build lok_integration

cmd/lok-render/         # example: render a doc to PNG
cmd/lok-convert/        # example: format conversion

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
│   mutex per Document, handle table, error mapping      │
└────────────────────────────────────────────────────────┘
                          │  narrow Go interface
                          ▼
┌────────────────────────────────────────────────────────┐
│  internal/lokc (cgo)                                   │
│   1:1 wrappers, //export trampoline, dlopen loader     │
└────────────────────────────────────────────────────────┘
                          │  C ABI
                          ▼
┌────────────────────────────────────────────────────────┐
│  libreofficekit.so  (loaded at runtime via dlopen)     │
└────────────────────────────────────────────────────────┘
```

### 5.2 Loader

`internal/lokc` uses `dlopen(RTLD_LAZY|RTLD_LOCAL)` to open `libsofficeapp.so`
inside the caller-supplied `program/` directory, resolves the
`libreofficekit_hook` (or `libreofficekit_hook_2`) symbol via `dlsym`, calls
it with the install path, and stores the returned `LibreOfficeKit*`
pointer. Unloading is a no-op: LO does not support unloading cleanly, and
the process will exit anyway.

### 5.3 Callback trampoline

LOK's `LibreOfficeKitDocumentCallback` is
`void (*)(int nType, const char* pPayload, void* pData)`. We pass a small
integer as `pData` — the key into a package-level
`map[uint64]func(Event)`. The exported trampoline looks up the closure,
copies `pPayload` to a Go string, and invokes it on a goroutine so the
LOK thread is not blocked by user code. The handle is freed when the
`Document` is closed.

### 5.4 Error mapping

LOK reports errors through `char* LibreOfficeKit::getError()`. Every
wrapper that can fail calls `getError` on a non-success return, copies and
frees the string, and returns a `*LOKError{Code, Detail}`. Well-known
codes get their own sentinel (`ErrLoadFailed` etc.) so callers can
`errors.Is`.

### 5.5 Concurrency

- `package lok` holds a `sync.Mutex` that is locked in `New` and
  `(*Office).Close`. Attempting a second `New` returns
  `ErrAlreadyInitialised`.
- Each `*Document` has its own `sync.Mutex` locked around every LOK call.
  Callers who need finer control use `UnsafeDocument` which exposes the
  raw handle and omits the mutex.
- Callback goroutines never touch document state directly; they re-enter
  through the public API if they need to.

## 6. Implementation phases

Each phase is one feature branch, one PR, red→green→refactor TDD, ≥ 90%
coverage on the `lok` package at all times.

### Phase 0 — Scaffold  `chore/scaffold`

- `go.mod` (Go 1.23+).
- Vendor `LibreOfficeKit.h`, `LibreOfficeKitEnums.h`,
  `LibreOfficeKitInit.h` under `third_party/lok/`, with upstream
  `LICENSE` (MPL-2.0) and a `VERSION` file pinning the source tag.
- `internal/lokc` skeleton with dlopen loader and a single round-trip
  function (`getVersionInfo`) so the cgo preamble is exercised.
- `lok` skeleton with a failing test that asserts `lok.New("")` returns
  `ErrInstallPathRequired`. (TDD canary.)
- `Makefile` targets: `build`, `test`, `test-integration`, `cover`,
  `lint`, `fmt`.
- GitHub Actions CI: matrix over linux/amd64 and linux/arm64, Go 1.23
  and tip, runs `go vet`, `go test -race`, and the coverage gate. An
  integration job runs inside a Docker image that installs
  LibreOffice 24.8 and exports `LOK_PATH`.
- README stub linking to CLAUDE.md and the spec.

Acceptance: `make test` green, coverage ≥ 90%, `make test-integration`
green in CI.

### Phase 1 — Office lifecycle  `feat/office-lifecycle`

Public API:

```go
type Office struct { /* ... */ }

func New(installPath string) (*Office, error)
func (*Office) Close() error
func (*Office) VersionInfo() (VersionInfo, error)
func (*Office) SetOptionalFeatures(feat OptionalFeatures) error
func (*Office) RegisterCallback(cb func(Event)) error
func (*Office) DumpState() (string, error)
```

`VersionInfo` parses LO's JSON version payload.
`OptionalFeatures` is a bitmask mirroring the upstream enum.

Tests cover: second `New` fails with `ErrAlreadyInitialised`; `Close`
after `Close` is a no-op; fake `lokc` injects error strings and asserts
they surface as `*LOKError`.

### Phase 2 — Document load / save  `feat/document-load-save`

```go
type Document struct { /* ... */ }
type DocumentType int   // Text, Spreadsheet, Presentation, Drawing, Other

func (*Office) Load(ctx context.Context, path string, opts ...LoadOption) (*Document, error)
func (*Document) Type() DocumentType
func (*Document) Save(ctx context.Context) error
func (*Document) SaveAs(ctx context.Context, path, format string, opts string) error
func (*Document) Close() error
```

`LoadOption` covers password, read-only, lang, macro-security. `opts`
string on `SaveAs` is LOK's filter-options string.

### Phase 3 — Parts & sizing  `feat/parts-and-size`

```go
func (*Document) Parts() int
func (*Document) Part() int
func (*Document) SetPart(n int) error
func (*Document) PartName(n int) string
func (*Document) PartHash(n int) string
func (*Document) DocumentSize() (widthTwips, heightTwips int64)
func (*Document) PartPageRectangles() []image.Rectangle   // twips
```

### Phase 4 — Rendering  `feat/rendering`

```go
func (*Document) InitializeForRendering(args string) error
func (*Document) SetClientZoom(pxPerTwipX, pxPerTwipY, tileWidthTwips, tileHeightTwips int) error
func (*Document) SetClientVisibleArea(r image.Rectangle /* twips */) error
func (*Document) PaintTile(pxW, pxH int, twipsX, twipsY, twipsW, twipsH int64) (*image.RGBA, error)
func (*Document) PaintPartTile(part int, pxW, pxH int, twipsX, twipsY, twipsW, twipsH int64) (*image.RGBA, error)
```

LOK writes BGRA; we convert to RGBA (or expose both via option) and
return `*image.RGBA`. Buffer is allocated in Go (`make([]byte, 4*w*h)`)
and its address handed to C for the duration of the call — safe under
cgo pointer rules because the slice header stays on the Go stack.

### Phase 5 — Input events  `feat/input-events`

```go
func (*Document) PostKeyEvent(typ KeyEventType, charCode, keyCode int) error
func (*Document) PostMouseEvent(typ MouseEventType, x, y int64, count, buttons, mods int) error
func (*Document) PostUnoCommand(cmd string, args string, notifyWhenFinished bool) error
```

Typed helpers for the dozen most-used UNO commands
(`Save`, `Bold`, `Italic`, `Underline`, `Undo`, `Redo`, `Copy`, `Cut`,
`Paste`, `SelectAll`, `InsertPageBreak`, `InsertTable`).

### Phase 6 — Views  `feat/views`

```go
func (*Document) CreateView() (ViewID, error)
func (*Document) DestroyView(ViewID) error
func (*Document) SetView(ViewID) error
func (*Document) View() ViewID
func (*Document) Views() []ViewID
```

Per-view state isolation tests.

### Phase 7 — Selection & clipboard  `feat/selection-clipboard`

```go
func (*Document) GetTextSelection(mimeType string) (string, string, error)   // text, used-mime
func (*Document) SetTextSelection(typ SelectionType, x, y int64) error
func (*Document) GetSelectionType() (SelectionType, error)
func (*Document) Copy() ([]byte, string, error)
func (*Document) Paste(mimeType string, data []byte) error
func (*Document) ResetSelection() error
func (*Document) SetGraphicSelection(typ int, x, y int64) error
```

### Phase 8 — Callbacks  `feat/callbacks`

```go
type EventType int
type Event struct {
    Type    EventType
    Payload []byte   // raw; helpers parse common types
}

func (*Document) OnEvent(cb func(Event)) error   // replaces any previous handler
```

C trampoline, handle table, and goroutine dispatch as in §5.3. Unit
tests drive synthetic callbacks by calling the package-level trampoline
directly with a known handle.

### Phase 9 — Command values  `feat/command-values`

```go
func (*Document) GetCommandValues(command string) (json.RawMessage, error)
func (*Document) SendDialogEvent(nWindowID uint64, args string) error
func (*Document) CompleteFunction(name string) error
```

Helpers that unmarshal the common command payloads (e.g.
`.uno:CellCursor`, `.uno:ViewRowColumnHeaders`) into typed structs.

### Phase 10 — Advanced (behind `lok_advanced`)  `feat/advanced`

```go
func (*Office) RunMacro(url string) error
func (*Document) Sign(certPem, keyPem []byte) error
func (*Office) InsertCertificate(cert, privateKey []byte) error
func (*Office) AddCertificate(cert []byte) error
func (*Document) SignatureState() (SignatureState, error)
func (*Office) TrimMemory(target int) error
```

Tier-2 coverage: functions exist and handle errors, but end-to-end
signing tests are only run when `LOK_TEST_CERTS` is set.

### Phase 11 — Examples  `feat/examples`

- `cmd/lok-render`: `lok-render -in file.odt -out page-%02d.png` using
  phases 1–4.
- `cmd/lok-convert`: `lok-convert -in a.docx -out a.pdf` using phases
  1–2.
- README with build / install / run instructions for each.

### Phase 12 — Integration CI  `feat/integration-ci`

- Dockerfile `ci/Dockerfile.lok` layering `libreoffice` and vendored
  headers on top of `golang:1.23`.
- GitHub Actions job that builds this image, runs
  `go test -tags=lok_integration -race ./...`, and uploads coverage.
- Optional matrix across LO 24.8 and LO 25.x once 25 is released.

## 7. Testing strategy

### 7.1 Unit tests (default `go test`)

- `lok` package uses the fake `lokc` injected via an unexported
  `lokBackend` interface set in `TestMain`.
- Every exported method has at least one happy-path and one error test.
- Callback trampoline tested by invoking the exported Go function
  directly from Go tests.
- `go test -race` is the default in CI.

### 7.2 Integration tests (`-tags=lok_integration`)

- Tiny fixture files in `testdata/` (`hello.odt`, `numbers.ods`,
  `deck.odp`, `drawing.odg`).
- Skip via `t.Skipf("LOK_PATH not set")` so developers without LO can
  still `go test ./...`.
- Cover: load/save round-trip, type detection, paint a tile and assert
  the buffer is non-empty, post a key and observe a callback event,
  create/destroy a view.

### 7.3 Coverage

- `lok` target ≥ 90%. CI fails the build below that.
- `internal/lokc` not gated — single-statement cgo wrappers skew the
  percentage. A smoke test per wrapper via integration tests is
  sufficient.
- `go tool cover -html=coverage.out -o coverage.html` artifact uploaded
  from CI for inspection.

## 8. Open questions

1. Do we want to expose `paintTileDX` / Direct2D on Windows later, or
   stay Cairo-only? Leaving this open until Windows support is picked
   up.
2. Should `image.RGBA` be the only output type, or also offer a
   zero-copy `[]byte` accessor for callers that already own a decoder?
   Leaning toward adding `PaintTileRaw` in phase 4 if benchmarks show
   the copy matters.
3. LO 25 is expected late 2026. Pinning 24.8 headers now; plan is to
   add a second vendored header directory once 25 ships and select at
   build time with a tag.

## 9. Risks & mitigations

| Risk | Mitigation |
|------|------------|
| LO process model (single LOK instance) surprises callers | Document loudly in package doc + `ErrAlreadyInitialised`; integration test asserts the error |
| cgo pointer-rule violations in callbacks | Use integer handles, not Go pointers; add a `go vet`/`cgocheck` CI step |
| Coverage gate blocks progress on tier-3 code | Exclude `internal/lokc` from the gate; tier-3 lives behind a build tag that is opt-in |
| LO version drift breaks the binding | Pin headers + version file; CI matrix across supported LO versions once a second release is supported |
| LOK thread-safety is subtle | Per-document mutex is default; `UnsafeDocument` opt-out is clearly marked |

## 10. Acceptance

The binding is "done" when, on Linux with LO 24.8 installed:

1. A user can `go get github.com/julianshen/golibreofficekit` and open,
   render, edit, and save each of ODT, DOCX, XLSX, ODS, PPTX, ODP, PDF.
2. `go test ./...` passes without LO installed.
3. `go test -tags=lok_integration -race ./...` passes with LO installed.
4. `lok` package coverage ≥ 90% on the default test run.
5. `cmd/lok-render` and `cmd/lok-convert` run against their README
   invocations.
