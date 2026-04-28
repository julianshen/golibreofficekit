# Architecture

This document describes how golibreofficekit is laid out and the
load-bearing decisions a new contributor needs to know. For end-user
API documentation see the [`lok` package godoc](https://pkg.go.dev/github.com/julianshen/golibreofficekit/lok).

Status: **Implemented (v1.0).**

## Goals and non-goals

**In scope:**
- Bind the LibreOfficeKit (LOK) C ABI so Go programs can load, render,
  edit, convert, and inspect every format LibreOffice supports
  (Writer / Calc / Impress / Draw).
- Idiomatic Go — `error` returns, no panics across the cgo boundary,
  `errors.Is` / `errors.As` traversal preserved.
- Linux and macOS, LibreOffice 24.8+.

**Not in scope (deferred or rejected):**
- Windows. The cgo preamble is structured so Windows can be added
  later, but no Windows code paths exist today.
- A drop-in replacement for the `soffice` CLI.
- Headless document *authoring* from scratch (start from a template).
- A GUI component (LOKDocView/GTK).
- LibreOffice versions before 24.8.

## Two-package layout

```
github.com/julianshen/golibreofficekit/
├── lok/                # public, idiomatic Go API
└── internal/lokc/      # thin cgo layer; one Go func per LOK C func
```

- **`internal/lokc`** is a faithful 1:1 cgo wrapping of the LOK ABI.
  Each exported Go function corresponds to a single LOK function and
  returns raw scalars, byte slices, or opaque handles. No JSON parsing,
  no pixel conversion, no Go conveniences. C shims at the top of each
  file follow a uniform `if (vtable_slot == NULL) return 0; else { call;
  return 1; }` pattern (canonical reference: `internal/lokc/selection.go`)
  so callers can distinguish "LOK build doesn't expose this slot" from
  "the call ran".
- **`lok`** is the public API. All interpretation lives here: typed
  enums, JSON parsing, pixel format conversion, bounded callback
  buffers, error wrapping, the office-wide mutex. `lok` depends on
  `internal/lokc` through an unexported `backend` interface
  (`lok/backend.go`) so unit tests inject a fake without touching cgo.

```
┌────────────────────────────────────────────────┐
│              user Go program                   │
└────────────────────────────────────────────────┘
                     │  idiomatic Go API
                     ▼
┌────────────────────────────────────────────────┐
│  lok            (public, ~25 .go files)        │
│   Office, Document, listener model, errors,    │
│   pixel conversion, JSON unmarshalling         │
└────────────────────────────────────────────────┘
                     │  unexported backend interface
                     ▼
┌────────────────────────────────────────────────┐
│  internal/lokc  (cgo, 1:1 wrappers)            │
│   shims, //export trampolines, dlopen loader   │
└────────────────────────────────────────────────┘
                     │  C ABI
                     ▼
┌────────────────────────────────────────────────┐
│  libsofficeapp.so  (loaded at runtime)         │
└────────────────────────────────────────────────┘
```

## Loader

`internal/lokc/loader.go` opens **`libsofficeapp.so`** inside the
caller-supplied `program/` directory with `dlopen(RTLD_LAZY|RTLD_LOCAL)`,
then resolves `libreofficekit_hook_2` via `dlsym` (falling back to
`libreofficekit_hook` for older builds), and calls it with the install
path to obtain a `LibreOfficeKit*`.

Failures from each candidate library and each candidate symbol are
accumulated with `errors.Join` so a misconfigured install reports every
candidate that was tried, not just the last one.

`dlclose` is deliberately never called — LO's static initialisers
cannot re-run cleanly within the same process, and the OS reclaims the
mapping at exit. This is also why **`lok.New` is a process singleton**:
calling `New` a second time while the first `*Office` is open returns
`ErrAlreadyInitialised`.

## Threading

LOK is not free-threaded. Document state is process-global (views
mutate document-wide state, callbacks fire from arbitrary LO threads),
so a per-document mutex would be unsafe.

This package uses **one `sync.Mutex` per `Office`**; every `Document`
operation acquires that mutex before entering LOK. Concurrent goroutines
can therefore share an `*Office` and `*Document` safely — long-running
calls block other callers, but the result is correct.

A second mutex guards `lok.New` / `Office.Close` so the process-singleton
constraint cannot race.

Listener callbacks run on a dedicated dispatcher goroutine owned by the
`*Office`. **Do not call back into the same document from inside a
listener** — that re-enters the office mutex and deadlocks.

## Callback trampoline

LOK delivers events through a C function pointer:

```c
void (*LibreOfficeKitCallback)(int nType, const char* pPayload, void* pData);
```

The trampoline (`internal/lokc/callback.go`, `//export goLOKDispatch*`)
executes synchronously on the LOK thread:

1. Reads `nType` and copies `pPayload` into a Go byte slice with
   `C.GoBytes`. **`pPayload` is never retained beyond the trampoline body**
   — LOK frees it on return.
2. Casts `pData` to a `uint64` integer handle.
3. Looks the handle up in a `sync.Map[uint64]→listenerSet` and hands the
   `Event{Type, Payload}` to a buffered channel. The dispatcher goroutine
   (started by `AddListener`) drains the channel and invokes user callbacks
   on a non-LOK goroutine.

A bounded per-listener queue protects against slow consumers; overflow
events are dropped and counted in `Office.DroppedEvents` /
`Document.DroppedEvents`. Listener panics are recovered, logged, and
counted in `Office.PanickedListeners` / `Document.PanickedListeners`.
Both counters are exposed so a misbehaving consumer is observable
rather than silent.

The trampoline never receives a Go pointer — only an integer handle —
keeping cgo's pointer rules satisfied.

## Error mapping

Two-tier error model:

- **Sentinels** (`lok/errors.go`) for discrete conditions usable with
  `errors.Is`: `ErrAlreadyInitialised`, `ErrInstallPathRequired`,
  `ErrClosed`, `ErrPathRequired`, `ErrInvalidOption`, `ErrUnsupported`,
  `ErrViewCreateFailed`, `ErrMacroFailed`, `ErrSignFailed`,
  `ErrPasteFailed`, `ErrNoValue`, `ErrClipboardFailed`.
- **`*LOKError{Op, Detail, err}`** carries LibreOffice's own error
  string. On a failed `Load` / `LoadDocumentWithOptions` / `SaveAs` /
  `Save`, the public method consults `OfficeGetError()` and embeds the
  result in `Detail` so the user sees "password required" / "filter
  rejected file" instead of a generic "documentLoad returned NULL".
  `Unwrap` preserves the inner sentinel for `errors.Is` / `errors.As`
  traversal.

`mapLokErr` (`lok/real_backend.go`) is the single point that translates
internal `lokc` sentinels to public `lok` sentinels. New forwarders
should pipe through it.

`internal/lokc` C shims that call optional LOK vtable slots return `0`
when the slot is `NULL`; the Go wrapper translates that to
`ErrUnsupported`. This was retrofitted in v1.0 so a stripped LO build
surfaces the missing capability instead of silently no-opping —
**every public method that touches an optional slot returns `error`**,
even when LOK's signature is `void`. See `internal/lokc/selection.go`
for the canonical pattern; `internal/lokc/{input,view,part,render,
document}.go` mirror it.

## Pixel format

LOK writes **premultiplied BGRA** (Cairo `ARGB32`, little-endian byte
order B, G, R, A with α pre-multiplied into RGB). The package exposes
both:

- `*Raw` methods (e.g. `PaintTileRaw`, `RenderImage`) hand back the
  premultiplied BGRA so callers feeding Cairo / Skia / WebRender skip a
  round-trip.
- The convenience methods (`PaintTile`, `RenderPNG`) unpremultiply
  (`c/α` per channel, clamped) and swizzle into straight RGBA in an
  `*image.NRGBA`. The conversion is pure Go in `lok/pixels.go` and is
  unit-tested with golden byte slices — no cgo needed.

cgo pointer rules: tile buffers are passed to a single, synchronous
`paintTile` call. LOK never retains the pointer, the Go runtime pins
the backing array for the call's duration, and the binding never hands
the pointer to the callback path.

## Testing strategy

**Unit tests (default `go test`):**
- `lok` injects a fake `backend` via `setBackend` (see `lok/office_test.go`
  for the canonical fake). Every exported method has at least one
  happy-path and one error-path test.
- `internal/lokc` uses `NewFakeDocumentHandle()` (a calloc'd
  `LibreOfficeKitDocument*` with `pClass == NULL`) to exercise
  vtable-NULL branches without LO installed.
- The callback trampoline is tested by invoking the exported Go
  function directly with synthetic handles.
- Pixel conversion is tested standalone against golden byte arrays.
- `go test -race` is the default in CI.

**Integration tests (`-tags=lok_integration`):**
- Tiny fixtures in `testdata/` (`hello.odt`, `numbers.ods`,
  `deck.odp`, `drawing.odg`).
- Skip cleanly when `LOK_PATH` is unset so `go test ./...` works
  without LO installed.
- Cover load/save round-trip, type detection, paint-and-assert,
  PDF export, callback delivery, view lifecycle.
- `-race` is **not** used here — LO's GLib internals trigger spurious
  reports.

**Coverage gate:** `lok` ≥ 90 % enforced. `internal/lokc` runs above
90 % from unit tests but trivial single-statement cgo wrappers are
exercised end-to-end by integration tests; the gate accepts that.

## CLI tools

The repository ships two example commands that double as integration
smoke tests:

- **`cmd/lokconv`** — convert documents to PDF or PNG. Output format
  inferred from `-out` extension.
- **`cmd/lokmd`** — bidirectional Markdown ↔ DOCX/PPTX, with a
  Marp-compatible slide pipeline (`---` separators, YAML front matter,
  `# ` headings).

Both read the install path from `-lo-path`, then `$LOK_PATH`, then a
small list of platform-default candidates.

## Versioning

The module is on Go 1.23 with no external Go dependencies. It pins
LibreOffice 24.8 headers under `third_party/lok/`; when LO 25.x or
later ships, vendor a second header set and select with a build tag.

`v1.0.0` is the first tagged release; from this point breaking changes
bump the major version.
