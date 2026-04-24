# Phase 6 — Rendering: Design Spec

Date: 2026-04-24
Status: Draft → Review
Owner: julianshen
Parent spec: `2026-04-19-lok-binding-design.md` §Phase 6

## 1. Goal

Add the rendering surface of the LOK binding: prepare a document for paint,
configure the client viewport, paint tiles into caller-provided or binding-
allocated buffers, render a search result to a bitmap, and render the current
shape selection to bytes (SVG in practice, but the binding does not promise
the format).

"Done" means a Go caller can load `hello.odt`, call one method to prepare
rendering, and get back an `*image.NRGBA` for a 256×256 tile of page 1 —
without touching cgo or worrying about pixel format.

## 2. Non-goals

- HiDPI / DPI-scale painting. `paintTileDX` / `paintWindowDPI` are Phase 10.
- Window-surface painting (`paintWindow*`). Phase 10.
- Font / glyph rendering (`renderFont`). Phase 10.
- Tile-mode negotiation. The binding supports LOK_TILEMODE_BGRA only and
  returns a typed error at `InitializeForRendering` if LOK reports anything
  else. No runtime dispatch for RGBA.
- Caller-tunable pixel conversion (lookup tables, SIMD). Start with the
  obvious per-pixel path; benchmark later if paint throughput matters.

## 3. Key decisions

Agreed with the user during brainstorming on 2026-04-24.

1. **One PR for the whole phase.** `InitializeForRendering`, `SetClientZoom`,
   `SetClientVisibleArea`, `PaintTile(Raw)`, `PaintPartTile(Raw)`,
   `RenderSearchResult(Raw)`, `RenderShapeSelection` plus the pixel
   converter ship together (answer: Option B during brainstorm).
2. **`RenderSearchResult` Raw + convenience pair.** Matches `PaintTile` /
   `PaintTileRaw`. `RenderSearchResultRaw` returns premul BGRA + pxW + pxH;
   `RenderSearchResult` returns `*image.NRGBA`.
3. **`RenderShapeSelection` returns `[]byte`.** LOK gives us bytes with a
   size_t length; we copy out and free. LO emits SVG in practice but the
   binding does not assert a content type.
4. **Verify tile mode once, in `InitializeForRendering`.** Cache on
   `Document.tileModeReady`. PaintTile refuses if unset. Any tile mode
   other than `LOK_TILEMODE_BGRA` (1) errors at `InitializeForRendering`.
5. **Wrong-size buffer is a typed error, not a panic.** `PaintTileRaw` /
   `PaintPartTileRaw` return `*LOKError{Op, Detail:"buffer too small..."}`
   when `len(buf) != 4*pxW*pxH`. Consistent with the rest of the binding's
   "no panics across cgo" rule.

## 4. Lifecycle

The rendering lifecycle a caller follows:

```
Office.Load(...)            // Phase 3
  doc.CreateView(...)        // Phase 4, optional — Load creates view 0
  doc.InitializeForRendering(args)     // NEW — cheaply verifies tile mode
  doc.SetClientZoom(...)               // NEW — optional
  doc.SetClientVisibleArea(...)        // NEW — optional, improves cache locality
  doc.PaintTile / PaintPartTile / ...  // NEW
  doc.Close()                // Phase 3
```

`InitializeForRendering` is a precondition for every paint and render
method added in this phase. Callers who skip it get `*LOKError{Op,
Detail:"InitializeForRendering not called"}`. The binding does NOT
auto-initialize on first paint — explicit is better than magic, and
real-world callers pass non-empty `args` (LOK's JSON render hints) that
the binding cannot guess.

`SetClientZoom` and `SetClientVisibleArea` are optional hints that LOK
uses for tile caching. Skipping them is legal; paint still works.
They do NOT require prior `InitializeForRendering` — callers may invoke
them in any order relative to `InitializeForRendering`. Only the paint
and render methods (`PaintTile*`, `PaintPartTile*`, `RenderSearchResult*`,
`RenderShapeSelection`) enforce the initialized precondition.

## 5. API surface (public `lok` package)

```go
// lok/render.go

// InitializeForRendering prepares the document for tile painting and
// verifies LOK is configured for premultiplied BGRA (Cairo ARGB32) output.
// args is an opaque JSON string passed through to LOK (e.g.
// `{".uno:HideWhitespace":{"type":"boolean","value":"true"}}`). An empty
// string is valid and means "no hints".
//
// Returns *LOKError{Op:"InitializeForRendering"} if LOK reports a tile
// mode other than LOK_TILEMODE_BGRA. Must be called before any paint or
// render method; subsequent paints use the cached tile-mode check.
func (*Document) InitializeForRendering(args string) error

// SetClientZoom describes the caller's render scale to LOK. All four
// arguments are in the units LOK expects: pixels for tilePx*, twips for
// tileTwip*. Fire-and-forget at the LOK level; a nil return does not
// confirm LOK applied the values.
func (*Document) SetClientZoom(tilePxW, tilePxH, tileTwipW, tileTwipH int) error

// SetClientVisibleArea tells LOK which part of the document the client
// is showing, in twips. Helps LOK's tile cache prefetch. Optional;
// skipping it is legal. Fire-and-forget.
func (*Document) SetClientVisibleArea(r TwipRect) error

// PaintTileRaw writes premultiplied BGRA (Cairo ARGB32; byte order
// B, G, R, A with RGB premultiplied by A) into buf. len(buf) must equal
// 4*pxW*pxH. A wrong-size buffer returns *LOKError without touching LOK.
// The document lock is held for the duration of the call; the buffer is
// not retained by LOK or by the binding after return.
func (*Document) PaintTileRaw(buf []byte, pxW, pxH int, r TwipRect) error

// PaintPartTileRaw is PaintTileRaw for a specific part (sheet/page/slide).
// The LOK mode argument is always 0 (normal tile); notes mode is for
// Impress and not exposed in Phase 6.
func (*Document) PaintPartTileRaw(buf []byte, part, pxW, pxH int, r TwipRect) error

// PaintTile allocates a pxW×pxH NRGBA image, calls PaintTileRaw, and
// unpremultiplies the pixels into the returned *image.NRGBA. α=0 pixels
// become (0,0,0,0). For hot paint loops use PaintTileRaw and reuse a
// buffer.
func (*Document) PaintTile(pxW, pxH int, r TwipRect) (*image.NRGBA, error)

// PaintPartTile is PaintTile for a specific part.
func (*Document) PaintPartTile(part, pxW, pxH int, r TwipRect) (*image.NRGBA, error)

// RenderSearchResultRaw renders the first match of query as a premul
// BGRA bitmap. query is the LOK search-result JSON (`.uno:SearchItem`
// payload). Returns (nil, 0, 0, nil) when LOK returns false (no match);
// the error path is reserved for binding-side failures like a closed
// document.
func (*Document) RenderSearchResultRaw(query string) (buf []byte, pxW, pxH int, err error)

// RenderSearchResult is the *image.NRGBA convenience form. Returns
// (nil, nil) when there is no match.
func (*Document) RenderSearchResult(query string) (*image.NRGBA, error)

// RenderShapeSelection returns whatever bytes LOK produces for the
// current shape selection (SVG in practice for LO 24.8, but the binding
// does not promise a format). Returns (nil, nil) when nothing is
// selected.
func (*Document) RenderShapeSelection() ([]byte, error)
```

### 5.1 Image type decision

`PaintTile` returns `*image.NRGBA` — Go's standard non-premultiplied RGBA.
Matches `image/png`'s default, plays well with the rest of the standard
library, and makes the common case (save-to-PNG) one call. Callers who
want the premul BGRA bytes LOK actually produces — for Cairo / Skia /
WebRender — use the `Raw` form and avoid the unpremultiply.

### 5.2 Buffer pinning

The `Raw` paint methods hand LOK a pointer to the caller's `buf` via
`&buf[0]` through a single synchronous cgo call. Per the cgo pointer
rules, the Go runtime pins the backing array for the duration of that
call. The pointer MUST NOT be:

- stored by the C side (LOK does not — the call is synchronous-write),
- retained by the binding after the call returns,
- handed to the callback path.

These invariants are load-bearing — a violation is undefined behavior,
not a visible crash. Comments in `lok/render.go` and the cgo file
document the rule at every pointer-passing site.

## 6. Pixel conversion (`lok/pixels.go`)

Pure Go, no cgo. One function:

```go
// unpremultiplyBGRAToNRGBA copies src (premul BGRA, 4*pxW*pxH bytes) into
// dst (straight NRGBA.Pix, same size). α=0 yields (0,0,0,0). src and dst
// may alias (the swizzle is per-pixel).
func unpremultiplyBGRAToNRGBA(dst, src []byte, pxW, pxH int)
```

Per pixel:

```go
b, g, r, a := src[0], src[1], src[2], src[3]
if a == 0 {
    dst[0], dst[1], dst[2], dst[3] = 0, 0, 0, 0
} else {
    dst[0] = uint8(uint16(r)*255/uint16(a))
    dst[1] = uint8(uint16(g)*255/uint16(a))
    dst[2] = uint8(uint16(b)*255/uint16(a))
    dst[3] = a
}
```

No lookup table. Straight-line per-pixel arithmetic. If benchmarks show
paint is CPU-bound on this loop we can add a 256×256 LUT behind the
function — the public API doesn't change.

`pixels_test.go` uses golden byte slices for α ∈ {0, 64, 128, 255} across
representative colors (opaque white, opaque red, 50% red, fully
transparent, 0×0 boundary). Tests live in the same package but do not
touch the backend seam — they run under plain `go test` with no cgo
backend involved.

## 7. Backend seam additions (`lok/backend.go`)

Eight new methods; buffer args use `[]byte` directly (the cgo layer
resolves `&buf[0]` for the LOK call).

```go
DocumentInitializeForRendering(d documentHandle, args string)
DocumentGetTileMode(d documentHandle) int
DocumentSetClientZoom(d documentHandle, tilePxW, tilePxH, tileTwipW, tileTwipH int)
DocumentSetClientVisibleArea(d documentHandle, x, y, w, h int)
DocumentPaintTile(d documentHandle, buf []byte, pxW, pxH int, x, y, w, h int)
DocumentPaintPartTile(d documentHandle, buf []byte, part, mode, pxW, pxH int, x, y, w, h int)
DocumentRenderSearchResult(d documentHandle, query string) (buf []byte, pxW, pxH int, ok bool)
DocumentRenderShapeSelection(d documentHandle) []byte
```

Coordinate-type note: LOK's `paintTile` / `paintPartTile` / `setClientVisibleArea`
declare tile positions and sizes as C `const int` (32-bit on LP64 Linux and
macOS, the only platforms we target). The seam therefore takes Go `int` — not
`int64` — for all four of those methods, even though the public `TwipRect`
uses `int64`. The public `PaintTile*` methods convert `TwipRect.X/Y/W/H` to
`int` at the seam boundary and return `*LOKError` if any field exceeds
`math.MaxInt32` (twips beyond ~2.1×10⁹ represent documents larger than 370
million inches — outside any realistic LO document, but we refuse cleanly
rather than truncate silently).

The two `Render*` methods copy LOK's allocated buffer into a fresh Go
slice and `free` the C pointer before returning. For `renderShapeSelection`
(char* plus explicit size_t length) and `renderSearchResult` (unsigned char*
plus int width/height plus size_t byte length), a new helper
`copyAndFreeBytes(p unsafe.Pointer, n C.size_t) []byte` lives next to the
existing `copyAndFree` in `internal/lokc/errstr.go` (or a sibling
`internal/lokc/bytes.go` if `errstr.go` grows unwieldy). The helper
`C.GoBytes`-copies the region, `C.free`s the C pointer, and returns the Go
slice. It is covered by the existing `internal/lokc` coverage gate. `DocumentRenderSearchResult` returns
`ok=false` when LOK returns false (no match) — the Go wrapper maps this
to `(nil, 0, 0, nil)` and does not call `OfficeGetError`.

The `mode` argument to `DocumentPaintPartTile` is always 0 in Phase 6
(the LOK Impress notes mode is not exposed yet). Present in the seam so
Phase 6b — if we ever add notes mode — can use it without breaking the
interface.

## 8. Internal cgo layer (`internal/lokc/render.go`)

Eight thin wrappers matching the LOK vtable entries. Each:

1. Guards a nil handle (return early with zero value; mirror the Phase 5
   pattern).
2. Casts the LOK function pointer out of `pClass`.
3. Calls it with C-typed args.
4. For `PaintTile` / `PaintPartTile`, passes `unsafe.Pointer(&buf[0])`
   cast to `*C.uchar`. Documents the pointer invariants inline.
5. For `RenderSearchResult` / `RenderShapeSelection`, calls the LOK
   function, copies the allocated bytes to a Go slice via `C.GoBytes`,
   frees the C buffer with `C.free`, returns the slice.

Tests (`internal/lokc/render_test.go`) exercise the nil-handle and
fake-handle paths with the existing `NewFakeDocumentHandle` helper —
same pattern as `internal/lokc/part_test.go`. They assert sentinel
values: `DocumentPaintTile` is a no-op (LOK vtable is NULL), the Render
functions return `(nil, ...)`.

## 9. Testing strategy

### 9.1 Unit tests (fake backend, `go test`)

- `fakeBackend` (`lok/fake_test.go`) gains capture fields + programmable
  outputs: `initArgs`, `tileMode`, `zoom[4]int`, `visibleArea TwipRect`,
  `paintCalls []paintCall`, `searchResult struct{buf []byte; w, h int; ok bool}`,
  `shapeSelection []byte`.
- Per Document method: happy path, `Close` → `ErrClosed` (driven by the
  `TestPhase6_AfterCloseErrors` table, matching the existing Phase 5
  `TestPartMethods_AfterCloseErrors` pattern).
- `InitializeForRendering`: tile-mode=1 happy, tile-mode=0 returns
  `*LOKError`, tile-mode=2 returns `*LOKError`.
- `PaintTileRaw`: buffer exactly right (happy), too small (error), too
  large (error — exact match required, not "at least").
- `PaintTileRaw` with a `TwipRect` whose `X` (or `Y`, `W`, `H`) exceeds
  `math.MaxInt32` returns `*LOKError` without calling the backend.
- `PaintTile` / `PaintPartTile`: fake writes a known premul pattern into
  the buffer; assert NRGBA output matches the expected unpremul pattern.
- `RenderSearchResultRaw`: ok=false → (nil, 0, 0, nil); ok=true → buf
  returned, pxW/pxH returned.
- `RenderShapeSelection`: empty → (nil, nil); non-empty → bytes returned.
- Every paint/render method without prior `InitializeForRendering`
  returns the typed "not initialized" error.

### 9.2 Pixels unit test (`lok/pixels_test.go`)

Golden-bytes table. Inputs are hand-constructed premul BGRA; expected
outputs are the known straight-NRGBA values. Cases:

- Opaque white (255,255,255,255) round-trips unchanged (modulo swizzle).
- Opaque red (0,0,255,255) stays red in NRGBA (0,0,255 src BGR →
  255,0,0 dst RGB).
- 50% red premul (0, 0, 128, 128) → straight (255, 0, 0, 128).
- Fully transparent (0, 0, 0, 0) → (0, 0, 0, 0).
- Edge: α=1 with channels=1 → (255, 255, 255, 1) after unpremul clamp.

### 9.3 Integration (`lok_integration`, `testdata/hello.odt`)

New subtests inside the existing `TestIntegration_FullLifecycle`, placed
after the `DocumentSize` / `PartPageRectangles` block and before the
`LoadFromReader(doc2)` block (the ordering constraint documented in the
Phase 5 integration test applies — two-docs + view dance still hurts
layout queries).

Assertions:

- `InitializeForRendering("")` → no error.
- `SetClientZoom(256, 256, 1440, 1440)` → no error.
- `SetClientVisibleArea({0, 0, 14400, 14400})` → no error.
- `PaintTile(256, 256, {0, 0, 14400, 14400})` → NRGBA non-nil; sum of
  RGB bytes > 0 (heuristic: something was painted).
- `PaintTileRaw` with `make([]byte, 4*256*256)` → no error; with
  `make([]byte, 10)` → `*LOKError`.
- `PaintPartTile(0, 256, 256, ...)` works when `nParts > 0`; skipped
  with `t.Logf` otherwise (Writer returns 0 parts).
- `RenderSearchResult` with an empty / minimal search-item JSON — if
  LOK returns false, assert `(nil, nil)`; if it returns true, assert
  a non-nil image. Never hard-fails on an empty match.
- `RenderShapeSelection()` with no selection → `(nil, nil)`.

### 9.4 Coverage

Target: `lok` package ≥ 90% (enforced since Phase 2). Phase 6 adds ~200
LOC of Go; the pixel converter and buffer-size guard are pure Go and
fully covered by unit tests. The paint and render methods reach LOK
through the fake in unit tests and through the real backend in the
integration suite — together they keep the new code above the gate.
The cgo wrappers in `internal/lokc/render.go` follow the established
pattern: the non-trivial ones (the two Render* functions with
copy+free) stay in the coverage gate; the fire-and-forget void wrappers
(`DocumentInitializeForRendering`, `SetClientZoom`, `SetClientVisibleArea`,
`PaintTile`, `PaintPartTile`) are split into a separate file that is
excluded from coverage, matching the Phase 5 treatment of
`DocumentSetPart` / `DocumentSetPartMode` / `DocumentSetOutlineState`.

## 10. File plan

Create:

- `lok/render.go` — the 10 Document methods + tile-mode check bookkeeping.
- `lok/render_test.go` — unit tests with fake backend.
- `lok/pixels.go` — `unpremultiplyBGRAToNRGBA` only.
- `lok/pixels_test.go` — golden-bytes tests.
- `internal/lokc/render.go` — trivial void wrappers (outside coverage gate).
- `internal/lokc/render_nontrivial.go` — `RenderSearchResult` + `RenderShapeSelection` (in coverage gate).
- `internal/lokc/render_test.go` — nil-handle + fake-handle tests.

Modify:

- `lok/backend.go` — add the 8 new interface methods.
- `lok/real_backend.go` — implement the 8 new methods by delegating to `internal/lokc`.
- `lok/fake_test.go` — add capture fields + default-value returns.
- `lok/document.go` — add `tileModeReady bool` field to `Document`.
- `lok/integration_test.go` — new subtests; keep the existing layout-
  queries / LoadFromReader ordering.

## 11. Out-of-band concerns

**Thread-local rendering state.** LOK holds rendering state per view.
`PaintTile` and friends paint the *current* view. Callers who want to
paint from a specific view must `SetView(id)` first. The binding does
not snapshot-and-restore the view around paint calls — that would hide
the contract. The godoc on `PaintTile` says so.

**`InitializeForRendering` on re-Load.** If a caller closes a doc and
loads another, the new `Document` has `tileModeReady = false` — the
flag is per-Document, not per-Office. Each Load → paint cycle pays one
`InitializeForRendering`.

**No `DocumentSize` dependency.** `PaintTile` does not require a prior
`DocumentSize` / `PartPageRectangles` call. The integration suite keeps
those before the paint block purely for the Phase 5 ordering reason
(two-docs + DestroyView interaction), not because of any rendering
dependency.

## 12. Risks

| Risk | Mitigation |
|------|------------|
| LOK returns non-BGRA tile mode on some future LO version | Explicit check in `InitializeForRendering` → typed error; we fail loudly instead of corrupting pixels |
| Caller re-uses a buffer across goroutines during paint | Office-wide mutex serializes all LOK entry points; caller concurrency is their responsibility |
| `RenderSearchResult` query format undocumented in LOK | Pass-through `string`; integration test tolerates `false` return; document the payload shape in godoc with a link |
| Pixel converter is a hot loop | Measure before optimizing. LUT is a private change if needed |
| `paintPartTile` notes-mode confusion | Phase 6 always passes mode=0. Seam argument exists for future phases |

## 13. Acceptance

- `make test` stays green; `lok` coverage ≥ 90%.
- `make test-integration` (with `LOK_PATH` and `GODEBUG=asyncpreemptoff=1`)
  passes the new paint + search + shape-selection subtests.
- A manual run of `PaintTile(256, 256, ...)` against `testdata/hello.odt`
  produces a PNG that visibly shows the "Hello, LibreOfficeKit" text
  when saved via `image/png`.
- Every LOK function in the Phase 6 row of the parent spec's §11
  coverage matrix maps to a Go symbol that lives under `lok.Document`.
