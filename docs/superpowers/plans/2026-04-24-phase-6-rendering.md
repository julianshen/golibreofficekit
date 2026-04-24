# Phase 6 — Rendering Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the rendering surface of the LOK binding: prepare a document for paint, configure viewport/zoom, paint tiles into caller-provided or binding-allocated buffers, render a search result to a bitmap, and render the current shape selection to bytes.

**Architecture:** Mirrors the Phase 5 split. `internal/lokc` gets eight new 1:1 cgo wrappers for `initializeForRendering`, `getTileMode`, `setClientZoom`, `setClientVisibleArea`, `paintTile`, `paintPartTile`, `renderSearchResult`, `renderShapeSelection`. `lok` adds a single new file `render.go` plus a pure-Go `pixels.go` that unpremultiplies BGRA into straight NRGBA. `InitializeForRendering` calls `getTileMode` once and caches a `tileModeReady` flag on `Document`; every paint/render method refuses if the flag is unset.

**Tech Stack:** Go 1.23+, cgo, LibreOfficeKit 24.8 C ABI, Go standard `image` / `image/color` packages.

**Spec:** `docs/superpowers/specs/2026-04-24-phase-6-rendering-design.md` (rev 2, committed this branch).

### Deviations from spec (called out early)

None. The spec was revised during its own review loop to fix the seam coordinate type (`int64` → `int` to match LOK's `const int`) and add the overflow range check. The plan implements the post-review spec verbatim.

### Branching

`feat/rendering`, branched from `main` after the Phase 6 spec PR merges.

---

## Files

| Path | Role |
|------|------|
| `lok/pixels.go` (create) | `unpremultiplyBGRAToNRGBA` pure-Go converter |
| `lok/pixels_test.go` (create) | Golden-byte tests for the converter |
| `lok/render.go` (create) | `InitializeForRendering`, `SetClientZoom`, `SetClientVisibleArea`, `PaintTile(Raw)`, `PaintPartTile(Raw)`, `RenderSearchResult(Raw)`, `RenderShapeSelection` |
| `lok/render_test.go` (create) | Unit tests via `fakeBackend` |
| `lok/backend.go` (modify) | Extend `backend` interface with 8 methods |
| `lok/real_backend.go` (modify) | Forward 8 methods into `internal/lokc` |
| `lok/real_backend_test.go` (modify) | Forwarding coverage test |
| `lok/office_test.go` (modify) | Extend `fakeBackend` with render state |
| `lok/document.go` (modify) | Add `tileModeReady bool` field on `Document` |
| `lok/integration_test.go` (modify) | New paint/search subtests |
| `internal/lokc/render.go` (create) | Fire-and-forget cgo wrappers (void returns) — outside coverage gate |
| `internal/lokc/render_out.go` (create) | `RenderSearchResult` + `RenderShapeSelection` cgo wrappers that copy+free LOK buffers — inside coverage gate |
| `internal/lokc/render_test.go` (create) | nil-handle + fake-handle tests |
| `internal/lokc/bytes.go` (create) | `copyAndFreeBytes` helper for sized unsigned char* buffers |
| `internal/lokc/bytes_test.go` (create) | Unit test for the helper |

---

## Task 0: Branch prep

- [ ] **Step 1: Sync main**

  ```bash
  git checkout main && git pull --ff-only && git status --short
  ```

  Expected: clean, main at the merge commit of the Phase 6 spec PR.

- [ ] **Step 2: Create branch**

  ```bash
  git checkout -b feat/rendering && git branch --show-current
  ```

  Expected: `feat/rendering`.

---

## Task 1: `lok/pixels.go` — pure-Go BGRA→NRGBA converter (TDD)

Why first: no cgo dependency, no backend wiring, no LOK. Lets us lock the pixel semantics with golden bytes before touching the cgo plumbing.

**Files:**
- Create: `lok/pixels.go`
- Create: `lok/pixels_test.go`

### 1.1 Failing tests

- [ ] **Step 1: Create `lok/pixels_test.go`**

  ```go
  //go:build linux || darwin

  package lok

  import (
  	"bytes"
  	"testing"
  )

  func TestUnpremultiplyBGRAToNRGBA_OpaqueRoundTrip(t *testing.T) {
  	// Opaque white: BGRA premul (255, 255, 255, 255) → NRGBA (255, 255, 255, 255).
  	src := []byte{255, 255, 255, 255}
  	dst := make([]byte, 4)
  	unpremultiplyBGRAToNRGBA(dst, src, 1, 1)
  	want := []byte{255, 255, 255, 255}
  	if !bytes.Equal(dst, want) {
  		t.Errorf("got %v, want %v", dst, want)
  	}
  }

  func TestUnpremultiplyBGRAToNRGBA_SwizzlesBGRAToRGBA(t *testing.T) {
  	// Opaque red in premul BGRA = (B=0, G=0, R=255, A=255) → NRGBA (R=255, G=0, B=0, A=255).
  	src := []byte{0, 0, 255, 255}
  	dst := make([]byte, 4)
  	unpremultiplyBGRAToNRGBA(dst, src, 1, 1)
  	want := []byte{255, 0, 0, 255}
  	if !bytes.Equal(dst, want) {
  		t.Errorf("got %v, want %v", dst, want)
  	}
  }

  func TestUnpremultiplyBGRAToNRGBA_UnpremultipliesHalfAlpha(t *testing.T) {
  	// 50% red premul: (B=0, G=0, R=128, A=128) → straight (R=255, G=0, B=0, A=128).
  	src := []byte{0, 0, 128, 128}
  	dst := make([]byte, 4)
  	unpremultiplyBGRAToNRGBA(dst, src, 1, 1)
  	want := []byte{255, 0, 0, 128}
  	if !bytes.Equal(dst, want) {
  		t.Errorf("got %v, want %v", dst, want)
  	}
  }

  func TestUnpremultiplyBGRAToNRGBA_ZeroAlpha(t *testing.T) {
  	// α=0 → (0, 0, 0, 0) regardless of src RGB.
  	src := []byte{200, 100, 50, 0}
  	dst := make([]byte, 4)
  	unpremultiplyBGRAToNRGBA(dst, src, 1, 1)
  	want := []byte{0, 0, 0, 0}
  	if !bytes.Equal(dst, want) {
  		t.Errorf("got %v, want %v", dst, want)
  	}
  }

  func TestUnpremultiplyBGRAToNRGBA_LowAlphaClamps(t *testing.T) {
  	// α=1 with channels=1 → straight (255, 255, 255, 1) — 1*255/1 = 255.
  	src := []byte{1, 1, 1, 1}
  	dst := make([]byte, 4)
  	unpremultiplyBGRAToNRGBA(dst, src, 1, 1)
  	want := []byte{255, 255, 255, 1}
  	if !bytes.Equal(dst, want) {
  		t.Errorf("got %v, want %v", dst, want)
  	}
  }

  func TestUnpremultiplyBGRAToNRGBA_TwoByTwo(t *testing.T) {
  	// Four pixels, in raster order: opaque red, opaque green, opaque blue, half red.
  	src := []byte{
  		0, 0, 255, 255, // red
  		0, 255, 0, 255, // green
  		255, 0, 0, 255, // blue
  		0, 0, 128, 128, // 50% red
  	}
  	dst := make([]byte, len(src))
  	unpremultiplyBGRAToNRGBA(dst, src, 2, 2)
  	want := []byte{
  		255, 0, 0, 255,
  		0, 255, 0, 255,
  		0, 0, 255, 255,
  		255, 0, 0, 128,
  	}
  	if !bytes.Equal(dst, want) {
  		t.Errorf("got %v, want %v", dst, want)
  	}
  }
  ```

- [ ] **Step 2: Run — red**

  ```bash
  go test ./lok/... -run TestUnpremultiplyBGRAToNRGBA
  ```

  Expected: compile error `undefined: unpremultiplyBGRAToNRGBA`.

### 1.2 Implement

- [ ] **Step 3: Create `lok/pixels.go`**

  ```go
  //go:build linux || darwin

  package lok

  // unpremultiplyBGRAToNRGBA copies src (premul BGRA, 4*pxW*pxH bytes,
  // byte order B, G, R, A with RGB premultiplied by A) into dst
  // (straight NRGBA, byte order R, G, B, A). len(dst) and len(src) must
  // both equal 4*pxW*pxH — callers validate. α=0 pixels yield
  // (0, 0, 0, 0) regardless of src RGB. src and dst may alias.
  func unpremultiplyBGRAToNRGBA(dst, src []byte, pxW, pxH int) {
  	n := 4 * pxW * pxH
  	for i := 0; i < n; i += 4 {
  		b, g, r, a := src[i], src[i+1], src[i+2], src[i+3]
  		if a == 0 {
  			dst[i], dst[i+1], dst[i+2], dst[i+3] = 0, 0, 0, 0
  			continue
  		}
  		dst[i] = uint8(uint16(r) * 255 / uint16(a))
  		dst[i+1] = uint8(uint16(g) * 255 / uint16(a))
  		dst[i+2] = uint8(uint16(b) * 255 / uint16(a))
  		dst[i+3] = a
  	}
  }
  ```

- [ ] **Step 4: Run — green**

  ```bash
  go test -race ./lok/... -run TestUnpremultiplyBGRAToNRGBA
  ```

  Expected: PASS (6 tests).

- [ ] **Step 5: Commit**

  ```bash
  git add lok/pixels.go lok/pixels_test.go
  git commit -m "$(cat <<'EOF'
  feat(lok): BGRA→NRGBA unpremultiply helper

  Pure Go, no cgo: takes LOK's premultiplied BGRA tile bytes and
  writes straight NRGBA into the caller's buffer. α=0 collapses to
  (0,0,0,0). Golden-byte tests cover opaque swizzle, half-alpha
  unpremul, zero-alpha passthrough, 2×2 raster-order correctness,
  and the low-alpha clamp case.

  Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
  EOF
  )"
  ```

---

## Task 2: `internal/lokc` — `copyAndFreeBytes` helper (TDD)

Why separate: the two `Render*` cgo wrappers both need it; get the helper landed first so the render wrappers can depend on it.

**Files:**
- Create: `internal/lokc/bytes.go`
- Create: `internal/lokc/bytes_test.go`

### 2.1 Failing test

- [ ] **Step 1: Create `internal/lokc/bytes_test.go`**

  ```go
  //go:build (linux && (amd64 || arm64)) || (darwin && (amd64 || arm64))

  package lokc

  import (
  	"bytes"
  	"testing"
  	"unsafe"
  )

  /*
  #include <stdlib.h>
  #include <string.h>
  */
  import "C"

  func TestCopyAndFreeBytes_CopiesAndFrees(t *testing.T) {
  	// Allocate a known 5-byte payload in C memory; copyAndFreeBytes
  	// must return a Go []byte that equals it and then free the C
  	// buffer. We can't observe the free directly, but the function's
  	// contract is that the returned slice is independent of the C
  	// memory.
  	payload := []byte{0xDE, 0xAD, 0xBE, 0xEF, 0x00}
  	cbuf := C.malloc(C.size_t(len(payload)))
  	if cbuf == nil {
  		t.Fatal("malloc failed")
  	}
  	// Fill cbuf with payload.
  	C.memcpy(cbuf, unsafe.Pointer(&payload[0]), C.size_t(len(payload)))

  	got := copyAndFreeBytes(cbuf, C.size_t(len(payload)))
  	if !bytes.Equal(got, payload) {
  		t.Errorf("got %v, want %v", got, payload)
  	}
  }

  func TestCopyAndFreeBytes_NilIsNil(t *testing.T) {
  	if got := copyAndFreeBytes(nil, 0); got != nil {
  		t.Errorf("nil input: got %v, want nil", got)
  	}
  }

  func TestCopyAndFreeBytes_ZeroLengthFreesAndReturnsNil(t *testing.T) {
  	cbuf := C.malloc(1) // non-nil but irrelevant contents
  	if got := copyAndFreeBytes(cbuf, 0); got != nil {
  		t.Errorf("0-length: got %v, want nil", got)
  	}
  	// cbuf must be freed even though n=0 — no way to assert directly;
  	// contract is documented so leaks would show under valgrind.
  }
  ```

- [ ] **Step 2: Run — red**

  ```bash
  go test ./internal/lokc/... -run TestCopyAndFreeBytes
  ```

  Expected: compile error `undefined: copyAndFreeBytes`.

### 2.2 Implement

- [ ] **Step 3: Create `internal/lokc/bytes.go`**

  ```go
  //go:build linux || darwin

  package lokc

  /*
  #include <stdlib.h>
  #include <string.h>
  */
  import "C"

  import "unsafe"

  // copyAndFreeBytes copies n bytes from the C-allocated region p into
  // a fresh Go slice and frees p with free(3). Returns nil for nil p
  // or n==0. Used by LOK wrappers that return unsigned char* plus an
  // explicit size_t length (renderSearchResult, renderShapeSelection
  // via its char** output).
  func copyAndFreeBytes(p unsafe.Pointer, n C.size_t) []byte {
  	if p == nil {
  		return nil
  	}
  	defer C.free(p)
  	if n == 0 {
  		return nil
  	}
  	return C.GoBytes(p, C.int(n))
  }
  ```

- [ ] **Step 4: Run — green**

  ```bash
  go test -race ./internal/lokc/... -run TestCopyAndFreeBytes
  ```

  Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

  ```bash
  git add internal/lokc/bytes.go internal/lokc/bytes_test.go
  git commit -m "$(cat <<'EOF'
  feat(lokc): copyAndFreeBytes helper for sized C buffers

  Complements copyAndFree (char*): LOK's renderSearchResult and
  renderShapeSelection hand us (unsigned char*, size_t) pairs that
  callers must free(3). The helper copies the bytes into a Go slice,
  frees the C pointer, and returns nil for nil-in or zero length.

  Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
  EOF
  )"
  ```

---

## Task 3: `internal/lokc` render wrappers (TDD)

**Files:**
- Create: `internal/lokc/render.go` (trivial void-returning wrappers — outside coverage gate)
- Create: `internal/lokc/render_out.go` (Render* wrappers that copy+free — inside coverage gate)
- Create: `internal/lokc/render_test.go`

### 3.1 Failing tests

- [ ] **Step 1: Create `internal/lokc/render_test.go`**

  ```go
  //go:build (linux && (amd64 || arm64)) || (darwin && (amd64 || arm64))

  package lokc

  import "testing"

  func TestDocumentRender_NilHandleAreNoOps(t *testing.T) {
  	var d DocumentHandle

  	DocumentInitializeForRendering(d, "")
  	DocumentSetClientZoom(d, 1, 1, 1, 1)
  	DocumentSetClientVisibleArea(d, 0, 0, 0, 0)
  	DocumentPaintTile(d, make([]byte, 16), 2, 2, 0, 0, 100, 100)
  	DocumentPaintPartTile(d, make([]byte, 16), 0, 0, 2, 2, 0, 0, 100, 100)

  	if got := DocumentGetTileMode(d); got != 0 {
  		t.Errorf("GetTileMode on nil: got %d, want 0", got)
  	}
  	if buf, w, h, ok := DocumentRenderSearchResult(d, "q"); buf != nil || w != 0 || h != 0 || ok {
  		t.Errorf("RenderSearchResult on nil: got (%v, %d, %d, %v)", buf, w, h, ok)
  	}
  	if got := DocumentRenderShapeSelection(d); got != nil {
  		t.Errorf("RenderShapeSelection on nil: got %v, want nil", got)
  	}
  }

  func TestDocumentRender_FakeHandle_SafeNoOps(t *testing.T) {
  	d := NewFakeDocumentHandle()
  	t.Cleanup(func() { FreeFakeDocumentHandle(d) })

  	DocumentInitializeForRendering(d, "{}")
  	DocumentSetClientZoom(d, 256, 256, 1440, 1440)
  	DocumentSetClientVisibleArea(d, 0, 0, 14400, 14400)
  	DocumentPaintTile(d, make([]byte, 16), 2, 2, 0, 0, 100, 100)
  	DocumentPaintPartTile(d, make([]byte, 16), 0, 0, 2, 2, 0, 0, 100, 100)

  	if got := DocumentGetTileMode(d); got != 0 {
  		t.Errorf("GetTileMode on fake: got %d, want 0", got)
  	}
  	if buf, w, h, ok := DocumentRenderSearchResult(d, "q"); buf != nil || w != 0 || h != 0 || ok {
  		t.Errorf("RenderSearchResult on fake: got (%v, %d, %d, %v)", buf, w, h, ok)
  	}
  	if got := DocumentRenderShapeSelection(d); got != nil {
  		t.Errorf("RenderShapeSelection on fake: got %v, want nil", got)
  	}
  }
  ```

- [ ] **Step 2: Run — red**

  ```bash
  go test ./internal/lokc/... -run TestDocumentRender
  ```

  Expected: undefined symbols.

### 3.2 Implement trivial (void) wrappers

- [ ] **Step 3: Create `internal/lokc/render.go`**

  ```go
  //go:build linux || darwin

  package lokc

  /*
  #cgo CFLAGS: -I${SRCDIR}/../../third_party/lok -DLOK_USE_UNSTABLE_API
  #include <stdlib.h>
  #include "LibreOfficeKit/LibreOfficeKit.h"

  static void go_doc_initialize_for_rendering(LibreOfficeKitDocument* d, const char* args) {
      if (d == NULL || d->pClass == NULL || d->pClass->initializeForRendering == NULL) return;
      d->pClass->initializeForRendering(d, args);
  }
  static int go_doc_get_tile_mode(LibreOfficeKitDocument* d) {
      if (d == NULL || d->pClass == NULL || d->pClass->getTileMode == NULL) return 0;
      return d->pClass->getTileMode(d);
  }
  static void go_doc_set_client_zoom(LibreOfficeKitDocument* d, int tpw, int tph, int ttw, int tth) {
      if (d == NULL || d->pClass == NULL || d->pClass->setClientZoom == NULL) return;
      d->pClass->setClientZoom(d, tpw, tph, ttw, tth);
  }
  static void go_doc_set_client_visible_area(LibreOfficeKitDocument* d, int x, int y, int w, int h) {
      if (d == NULL || d->pClass == NULL || d->pClass->setClientVisibleArea == NULL) return;
      d->pClass->setClientVisibleArea(d, x, y, w, h);
  }
  static void go_doc_paint_tile(LibreOfficeKitDocument* d, unsigned char* buf,
      int canvasW, int canvasH, int posX, int posY, int tileW, int tileH) {
      if (d == NULL || d->pClass == NULL || d->pClass->paintTile == NULL) return;
      d->pClass->paintTile(d, buf, canvasW, canvasH, posX, posY, tileW, tileH);
  }
  static void go_doc_paint_part_tile(LibreOfficeKitDocument* d, unsigned char* buf,
      int part, int mode, int canvasW, int canvasH, int posX, int posY, int tileW, int tileH) {
      if (d == NULL || d->pClass == NULL || d->pClass->paintPartTile == NULL) return;
      d->pClass->paintPartTile(d, buf, part, mode, canvasW, canvasH, posX, posY, tileW, tileH);
  }
  */
  import "C"

  import "unsafe"

  // DocumentInitializeForRendering forwards to pClass->initializeForRendering.
  // args is LOK's JSON hint string (may be empty).
  func DocumentInitializeForRendering(d DocumentHandle, args string) {
  	if !d.IsValid() {
  		return
  	}
  	cargs := C.CString(args)
  	defer C.free(unsafe.Pointer(cargs))
  	C.go_doc_initialize_for_rendering(d.p, cargs)
  }

  // DocumentGetTileMode returns LOK's tile-mode enum (0=RGBA, 1=BGRA).
  // Returns 0 on unavailable handle/vtable.
  func DocumentGetTileMode(d DocumentHandle) int {
  	if !d.IsValid() {
  		return 0
  	}
  	return int(C.go_doc_get_tile_mode(d.p))
  }

  // DocumentSetClientZoom forwards to pClass->setClientZoom.
  func DocumentSetClientZoom(d DocumentHandle, tilePxW, tilePxH, tileTwipW, tileTwipH int) {
  	if !d.IsValid() {
  		return
  	}
  	C.go_doc_set_client_zoom(d.p, C.int(tilePxW), C.int(tilePxH), C.int(tileTwipW), C.int(tileTwipH))
  }

  // DocumentSetClientVisibleArea forwards to pClass->setClientVisibleArea.
  // All coordinates are twips; LOK's C ABI takes int (32-bit) for these.
  func DocumentSetClientVisibleArea(d DocumentHandle, x, y, w, h int) {
  	if !d.IsValid() {
  		return
  	}
  	C.go_doc_set_client_visible_area(d.p, C.int(x), C.int(y), C.int(w), C.int(h))
  }

  // DocumentPaintTile forwards to pClass->paintTile. buf must hold at
  // least 4*pxW*pxH bytes — callers validate. The backing array is
  // pinned for the duration of this synchronous cgo call (cgo pointer
  // rule): the pointer must NOT be retained or stored by C.
  func DocumentPaintTile(d DocumentHandle, buf []byte, pxW, pxH, x, y, w, h int) {
  	if !d.IsValid() || len(buf) == 0 {
  		return
  	}
  	C.go_doc_paint_tile(d.p, (*C.uchar)(unsafe.Pointer(&buf[0])),
  		C.int(pxW), C.int(pxH), C.int(x), C.int(y), C.int(w), C.int(h))
  }

  // DocumentPaintPartTile forwards to pClass->paintPartTile. mode is
  // the LOK_PARTMODE_* enum (0 = normal). Same buffer-pinning rule
  // as DocumentPaintTile.
  func DocumentPaintPartTile(d DocumentHandle, buf []byte, part, mode, pxW, pxH, x, y, w, h int) {
  	if !d.IsValid() || len(buf) == 0 {
  		return
  	}
  	C.go_doc_paint_part_tile(d.p, (*C.uchar)(unsafe.Pointer(&buf[0])),
  		C.int(part), C.int(mode), C.int(pxW), C.int(pxH),
  		C.int(x), C.int(y), C.int(w), C.int(h))
  }
  ```

### 3.3 Implement Render* wrappers (inside coverage gate)

- [ ] **Step 4: Create `internal/lokc/render_out.go`**

  ```go
  //go:build linux || darwin

  package lokc

  /*
  #cgo CFLAGS: -I${SRCDIR}/../../third_party/lok -DLOK_USE_UNSTABLE_API
  #include <stdlib.h>
  #include <stdbool.h>
  #include "LibreOfficeKit/LibreOfficeKit.h"

  static bool go_doc_render_search_result(LibreOfficeKitDocument* d, const char* q,
      unsigned char** outBuf, int* outW, int* outH, size_t* outSize) {
      *outBuf = NULL; *outW = 0; *outH = 0; *outSize = 0;
      if (d == NULL || d->pClass == NULL || d->pClass->renderSearchResult == NULL) return false;
      return d->pClass->renderSearchResult(d, q, outBuf, outW, outH, outSize);
  }
  static size_t go_doc_render_shape_selection(LibreOfficeKitDocument* d, char** outBuf) {
      *outBuf = NULL;
      if (d == NULL || d->pClass == NULL || d->pClass->renderShapeSelection == NULL) return 0;
      return d->pClass->renderShapeSelection(d, outBuf);
  }
  */
  import "C"

  import "unsafe"

  // DocumentRenderSearchResult renders the first match of the LOK
  // search-item JSON query as a premul BGRA bitmap. Returns
  // (nil, 0, 0, false) when LOK returns false (no match) or when the
  // handle/vtable is unavailable. The caller-owned bitmap is copied
  // into a Go slice and the C buffer is freed before return.
  func DocumentRenderSearchResult(d DocumentHandle, query string) (buf []byte, pxW, pxH int, ok bool) {
  	if !d.IsValid() {
  		return nil, 0, 0, false
  	}
  	cq := C.CString(query)
  	defer C.free(unsafe.Pointer(cq))

  	var cbuf *C.uchar
  	var cw, ch C.int
  	var csize C.size_t
  	res := C.go_doc_render_search_result(d.p, cq, &cbuf, &cw, &ch, &csize)
  	if !bool(res) {
  		// LOK guarantees a NULL buffer on false; defensive free anyway.
  		if cbuf != nil {
  			C.free(unsafe.Pointer(cbuf))
  		}
  		return nil, 0, 0, false
  	}
  	return copyAndFreeBytes(unsafe.Pointer(cbuf), csize), int(cw), int(ch), true
  }

  // DocumentRenderShapeSelection returns the current shape selection's
  // bytes (SVG in practice on LO 24.8). Returns nil when nothing is
  // selected or the handle/vtable is unavailable. The LOK-allocated
  // buffer is copied and freed before return.
  func DocumentRenderShapeSelection(d DocumentHandle) []byte {
  	if !d.IsValid() {
  		return nil
  	}
  	var cbuf *C.char
  	n := C.go_doc_render_shape_selection(d.p, &cbuf)
  	return copyAndFreeBytes(unsafe.Pointer(cbuf), n)
  }
  ```

- [ ] **Step 5: Run tests — green**

  ```bash
  go test -race ./internal/lokc/... -run TestDocumentRender
  ```

  Expected: PASS (2 tests).

- [ ] **Step 6: Coverage gate**

  ```bash
  make cover-gate
  ```

  Expected: ≥ 90.0%.

- [ ] **Step 7: Commit**

  ```bash
  git add internal/lokc/render.go internal/lokc/render_out.go internal/lokc/render_test.go
  git commit -m "$(cat <<'EOF'
  feat(lokc): add render-level cgo wrappers

  Eight 1:1 vtable wrappers: InitializeForRendering, GetTileMode,
  SetClientZoom, SetClientVisibleArea, PaintTile, PaintPartTile,
  RenderSearchResult, RenderShapeSelection. Void wrappers live in
  render.go (outside the coverage gate per the Phase 5 pattern);
  the two Render* wrappers that copy+free LOK-allocated buffers
  live in render_out.go and stay in the gate.

  Paint wrappers pass the caller's []byte backing array through a
  single synchronous cgo call — the Go runtime pins it for the
  duration, and the pointer is not retained by C.

  Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
  EOF
  )"
  ```

---

## Task 4: `lok` backend seam + fakeBackend extensions

**Files:**
- Modify: `lok/backend.go`
- Modify: `lok/real_backend.go`
- Modify: `lok/office_test.go`
- Modify: `lok/real_backend_test.go`

### Step 1: Extend `lok/backend.go`

- [ ] Append to the `backend` interface:

  ```go
  	DocumentInitializeForRendering(d documentHandle, args string)
  	DocumentGetTileMode(d documentHandle) int
  	DocumentSetClientZoom(d documentHandle, tilePxW, tilePxH, tileTwipW, tileTwipH int)
  	DocumentSetClientVisibleArea(d documentHandle, x, y, w, h int)
  	DocumentPaintTile(d documentHandle, buf []byte, pxW, pxH, x, y, w, h int)
  	DocumentPaintPartTile(d documentHandle, buf []byte, part, mode, pxW, pxH, x, y, w, h int)
  	DocumentRenderSearchResult(d documentHandle, query string) (buf []byte, pxW, pxH int, ok bool)
  	DocumentRenderShapeSelection(d documentHandle) []byte
  ```

### Step 2: Extend `lok/real_backend.go`

- [ ] Append one-line forwarders, each passing `mustDoc(d).d`:

  ```go
  func (realBackend) DocumentInitializeForRendering(d documentHandle, args string) {
  	lokc.DocumentInitializeForRendering(mustDoc(d).d, args)
  }
  func (realBackend) DocumentGetTileMode(d documentHandle) int {
  	return lokc.DocumentGetTileMode(mustDoc(d).d)
  }
  func (realBackend) DocumentSetClientZoom(d documentHandle, tpw, tph, ttw, tth int) {
  	lokc.DocumentSetClientZoom(mustDoc(d).d, tpw, tph, ttw, tth)
  }
  func (realBackend) DocumentSetClientVisibleArea(d documentHandle, x, y, w, h int) {
  	lokc.DocumentSetClientVisibleArea(mustDoc(d).d, x, y, w, h)
  }
  func (realBackend) DocumentPaintTile(d documentHandle, buf []byte, pxW, pxH, x, y, w, h int) {
  	lokc.DocumentPaintTile(mustDoc(d).d, buf, pxW, pxH, x, y, w, h)
  }
  func (realBackend) DocumentPaintPartTile(d documentHandle, buf []byte, part, mode, pxW, pxH, x, y, w, h int) {
  	lokc.DocumentPaintPartTile(mustDoc(d).d, buf, part, mode, pxW, pxH, x, y, w, h)
  }
  func (realBackend) DocumentRenderSearchResult(d documentHandle, q string) ([]byte, int, int, bool) {
  	return lokc.DocumentRenderSearchResult(mustDoc(d).d, q)
  }
  func (realBackend) DocumentRenderShapeSelection(d documentHandle) []byte {
  	return lokc.DocumentRenderShapeSelection(mustDoc(d).d)
  }
  ```

### Step 3: Extend `fakeBackend` in `lok/office_test.go`

- [ ] Add render state fields to the `fakeBackend` struct (alongside the part state, under a new "Render state" comment):

  ```go
  	// Render state.
  	lastInitArgs        string
  	tileMode            int // fakes can programme this; default 0
  	lastZoom            [4]int
  	lastVisibleArea     [4]int
  	paintCalls          []fakePaint
  	partPaintCalls      []fakePartPaint
  	searchResultBuf     []byte
  	searchResultPxW     int
  	searchResultPxH     int
  	searchResultOK      bool
  	lastSearchQuery     string
  	shapeSelection      []byte
  ```

- [ ] Add the auxiliary struct types above `fakeBackend`:

  ```go
  type fakePaint struct {
  	pxW, pxH, x, y, w, h int
  	bufLen               int
  }
  type fakePartPaint struct {
  	part, mode, pxW, pxH, x, y, w, h int
  	bufLen                           int
  }
  ```

- [ ] Add method implementations at the bottom of the file:

  ```go
  func (f *fakeBackend) DocumentInitializeForRendering(_ documentHandle, args string) {
  	f.lastInitArgs = args
  }
  func (f *fakeBackend) DocumentGetTileMode(documentHandle) int { return f.tileMode }
  func (f *fakeBackend) DocumentSetClientZoom(_ documentHandle, tpw, tph, ttw, tth int) {
  	f.lastZoom = [4]int{tpw, tph, ttw, tth}
  }
  func (f *fakeBackend) DocumentSetClientVisibleArea(_ documentHandle, x, y, w, h int) {
  	f.lastVisibleArea = [4]int{x, y, w, h}
  }
  func (f *fakeBackend) DocumentPaintTile(_ documentHandle, buf []byte, pxW, pxH, x, y, w, h int) {
  	f.paintCalls = append(f.paintCalls, fakePaint{pxW: pxW, pxH: pxH, x: x, y: y, w: w, h: h, bufLen: len(buf)})
  }
  func (f *fakeBackend) DocumentPaintPartTile(_ documentHandle, buf []byte, part, mode, pxW, pxH, x, y, w, h int) {
  	f.partPaintCalls = append(f.partPaintCalls, fakePartPaint{
  		part: part, mode: mode, pxW: pxW, pxH: pxH, x: x, y: y, w: w, h: h, bufLen: len(buf),
  	})
  }
  func (f *fakeBackend) DocumentRenderSearchResult(_ documentHandle, q string) ([]byte, int, int, bool) {
  	f.lastSearchQuery = q
  	return f.searchResultBuf, f.searchResultPxW, f.searchResultPxH, f.searchResultOK
  }
  func (f *fakeBackend) DocumentRenderShapeSelection(documentHandle) []byte {
  	return f.shapeSelection
  }
  ```

### Step 4: Extend `lok/real_backend_test.go`

- [ ] Add `TestRealBackend_RenderForwarding` after `TestRealBackend_PartForwarding`, mirroring the part shape:

  ```go
  func TestRealBackend_RenderForwarding(t *testing.T) {
  	rb := realBackend{}
  	fakeDocHandle := lokc.NewFakeDocumentHandle()
  	defer lokc.FreeFakeDocumentHandle(fakeDocHandle)
  	rdoc := realDocumentHandle{d: fakeDocHandle}

  	// Void forwarders — no panic.
  	rb.DocumentInitializeForRendering(rdoc, "")
  	rb.DocumentSetClientZoom(rdoc, 256, 256, 1440, 1440)
  	rb.DocumentSetClientVisibleArea(rdoc, 0, 0, 14400, 14400)
  	rb.DocumentPaintTile(rdoc, make([]byte, 16), 2, 2, 0, 0, 100, 100)
  	rb.DocumentPaintPartTile(rdoc, make([]byte, 16), 0, 0, 2, 2, 0, 0, 100, 100)

  	if got := rb.DocumentGetTileMode(rdoc); got != 0 {
  		t.Errorf("GetTileMode: got %d, want 0", got)
  	}
  	if buf, w, h, ok := rb.DocumentRenderSearchResult(rdoc, "q"); buf != nil || w != 0 || h != 0 || ok {
  		t.Errorf("RenderSearchResult: got (%v, %d, %d, %v)", buf, w, h, ok)
  	}
  	if got := rb.DocumentRenderShapeSelection(rdoc); got != nil {
  		t.Errorf("RenderShapeSelection: got %v, want nil", got)
  	}
  }
  ```

### Step 5: Verify + commit

- [ ] **Run:** `make all && make cover-gate`
  Expected: green, coverage ≥ 90%.

- [ ] **Commit:**

  ```bash
  git add lok/backend.go lok/real_backend.go lok/office_test.go lok/real_backend_test.go
  git commit -m "$(cat <<'EOF'
  feat(lok): backend seam and fake for rendering

  backend interface grows 8 render methods. realBackend forwards
  each to internal/lokc; fakeBackend captures init args, zoom,
  visible area, and paint-call tuples for assertion, and can
  programme tileMode / search-result / shape-selection outputs
  per-test.

  Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
  EOF
  )"
  ```

---

## Task 5: `Document.InitializeForRendering` + zoom/visible-area (TDD)

**Files:**
- Create: `lok/render.go`
- Create: `lok/render_test.go`
- Modify: `lok/document.go` (add `tileModeReady bool` on `Document`)

### 5.1 Add the `tileModeReady` field

- [ ] **Step 1: Modify `lok/document.go`**

  Inside the `Document` struct, after `closed bool`, add:

  ```go
  	tileModeReady bool // set by InitializeForRendering after LOK_TILEMODE_BGRA is confirmed
  ```

### 5.2 Failing tests

- [ ] **Step 2: Create `lok/render_test.go`**

  ```go
  //go:build linux || darwin

  package lok

  import (
  	"errors"
  	"testing"
  )

  func TestInitializeForRendering_HappyPath(t *testing.T) {
  	fb := &fakeBackend{tileMode: 1} // BGRA
  	_, doc := loadFakeDoc(t, fb)
  	if err := doc.InitializeForRendering(""); err != nil {
  		t.Fatalf("InitializeForRendering: %v", err)
  	}
  	if fb.lastInitArgs != "" {
  		t.Errorf("lastInitArgs=%q, want empty", fb.lastInitArgs)
  	}
  }

  func TestInitializeForRendering_ForwardsArgs(t *testing.T) {
  	fb := &fakeBackend{tileMode: 1}
  	_, doc := loadFakeDoc(t, fb)
  	args := `{".uno:HideWhitespace":{"type":"boolean","value":"true"}}`
  	if err := doc.InitializeForRendering(args); err != nil {
  		t.Fatal(err)
  	}
  	if fb.lastInitArgs != args {
  		t.Errorf("lastInitArgs=%q, want %q", fb.lastInitArgs, args)
  	}
  }

  func TestInitializeForRendering_UnsupportedTileMode(t *testing.T) {
  	fb := &fakeBackend{tileMode: 0} // RGBA — unsupported
  	_, doc := loadFakeDoc(t, fb)
  	err := doc.InitializeForRendering("")
  	var lokErr *LOKError
  	if !errors.As(err, &lokErr) || lokErr.Op != "InitializeForRendering" {
  		t.Errorf("want *LOKError{Op: InitializeForRendering}, got %T %v", err, err)
  	}
  }

  func TestInitializeForRendering_UnexpectedTileMode(t *testing.T) {
  	// Any non-1 mode (including future enum values) is treated as an error.
  	fb := &fakeBackend{tileMode: 2}
  	_, doc := loadFakeDoc(t, fb)
  	err := doc.InitializeForRendering("")
  	var lokErr *LOKError
  	if !errors.As(err, &lokErr) {
  		t.Errorf("want *LOKError, got %T %v", err, err)
  	}
  }

  func TestInitializeForRendering_AfterCloseErrors(t *testing.T) {
  	_, doc := loadFakeDoc(t, &fakeBackend{tileMode: 1})
  	doc.Close()
  	if err := doc.InitializeForRendering(""); !errors.Is(err, ErrClosed) {
  		t.Errorf("want ErrClosed, got %v", err)
  	}
  }

  func TestSetClientZoom_Passes(t *testing.T) {
  	fb := &fakeBackend{}
  	_, doc := loadFakeDoc(t, fb)
  	if err := doc.SetClientZoom(256, 256, 1440, 1440); err != nil {
  		t.Fatal(err)
  	}
  	if fb.lastZoom != [4]int{256, 256, 1440, 1440} {
  		t.Errorf("lastZoom=%v", fb.lastZoom)
  	}
  }

  func TestSetClientZoom_WithoutInitializeOK(t *testing.T) {
  	// Zoom is an optional hint; does NOT require InitializeForRendering.
  	_, doc := loadFakeDoc(t, &fakeBackend{})
  	if err := doc.SetClientZoom(1, 1, 1, 1); err != nil {
  		t.Errorf("want no error, got %v", err)
  	}
  }

  func TestSetClientVisibleArea_PassesAsInt(t *testing.T) {
  	fb := &fakeBackend{}
  	_, doc := loadFakeDoc(t, fb)
  	if err := doc.SetClientVisibleArea(TwipRect{X: 0, Y: 0, W: 14400, H: 14400}); err != nil {
  		t.Fatal(err)
  	}
  	if fb.lastVisibleArea != [4]int{0, 0, 14400, 14400} {
  		t.Errorf("lastVisibleArea=%v", fb.lastVisibleArea)
  	}
  }

  func TestSetClientVisibleArea_RejectsOverflow(t *testing.T) {
  	_, doc := loadFakeDoc(t, &fakeBackend{})
  	err := doc.SetClientVisibleArea(TwipRect{X: 0, Y: 0, W: 1<<32 + 1, H: 1})
  	var lokErr *LOKError
  	if !errors.As(err, &lokErr) {
  		t.Errorf("want *LOKError, got %T %v", err, err)
  	}
  }
  ```

- [ ] **Step 3: Run — red**

  ```bash
  go test ./lok/... -run 'TestInitializeForRendering|TestSetClientZoom|TestSetClientVisibleArea'
  ```

  Expected: undefined symbols.

### 5.3 Implement

- [ ] **Step 4: Create `lok/render.go`**

  ```go
  //go:build linux || darwin

  package lok

  import (
  	"fmt"
  	"image"
  	"math"
  )

  // lokTileModeBGRA is LOK's LOK_TILEMODE_BGRA — Cairo ARGB32 byte order
  // (B, G, R, A with premultiplied alpha). The binding refuses any other
  // tile mode at InitializeForRendering time.
  const lokTileModeBGRA = 1

  // InitializeForRendering prepares the document for tile painting and
  // verifies LOK is configured for premultiplied BGRA output. args is
  // an opaque JSON hint string passed through to LOK (empty is valid).
  // Must be called before any Paint* or Render* method; subsequent
  // paints use the cached tile-mode check.
  func (d *Document) InitializeForRendering(args string) error {
  	unlock, err := d.guard()
  	if err != nil {
  		return err
  	}
  	defer unlock()
  	d.office.be.DocumentInitializeForRendering(d.h, args)
  	mode := d.office.be.DocumentGetTileMode(d.h)
  	if mode != lokTileModeBGRA {
  		return &LOKError{Op: "InitializeForRendering", Detail: fmt.Sprintf("unsupported tile mode %d (binding requires LOK_TILEMODE_BGRA)", mode)}
  	}
  	d.tileModeReady = true
  	return nil
  }

  // SetClientZoom tells LOK the caller's render scale. Fire-and-forget;
  // a nil return does not confirm LOK applied the values. Does NOT
  // require a prior InitializeForRendering — zoom is a cache hint.
  func (d *Document) SetClientZoom(tilePxW, tilePxH, tileTwipW, tileTwipH int) error {
  	unlock, err := d.guard()
  	if err != nil {
  		return err
  	}
  	defer unlock()
  	d.office.be.DocumentSetClientZoom(d.h, tilePxW, tilePxH, tileTwipW, tileTwipH)
  	return nil
  }

  // SetClientVisibleArea tells LOK the client's visible region in twips.
  // Helps LOK prefetch tiles; does NOT require InitializeForRendering.
  // Any field beyond math.MaxInt32 returns *LOKError — LOK's C ABI
  // takes int (32-bit) and we refuse to silently truncate.
  func (d *Document) SetClientVisibleArea(r TwipRect) error {
  	if err := requireInt32Rect("SetClientVisibleArea", r); err != nil {
  		return err
  	}
  	unlock, err := d.guard()
  	if err != nil {
  		return err
  	}
  	defer unlock()
  	d.office.be.DocumentSetClientVisibleArea(d.h, int(r.X), int(r.Y), int(r.W), int(r.H))
  	return nil
  }

  // requireInt32Rect returns *LOKError if any rect field exceeds
  // math.MaxInt32. LOK's tile-position and visible-area ABI takes C int
  // (32-bit on LP64); truncation would silently corrupt LOK's internal
  // coordinates. Negative values are legal (LO uses them for offsets).
  func requireInt32Rect(op string, r TwipRect) error {
  	if r.X > math.MaxInt32 || r.X < math.MinInt32 ||
  		r.Y > math.MaxInt32 || r.Y < math.MinInt32 ||
  		r.W > math.MaxInt32 || r.W < math.MinInt32 ||
  		r.H > math.MaxInt32 || r.H < math.MinInt32 {
  		return &LOKError{Op: op, Detail: fmt.Sprintf("rect field exceeds int32 range: %+v", r)}
  	}
  	return nil
  }

  // imageBoundsForTile returns a Go image.Rectangle matching a pxW×pxH
  // tile. Private; callers compose with image.NewNRGBA.
  func imageBoundsForTile(pxW, pxH int) image.Rectangle {
  	return image.Rect(0, 0, pxW, pxH)
  }
  ```

- [ ] **Step 5: Run — green**

  ```bash
  go test -race ./lok/... -run 'TestInitializeForRendering|TestSetClientZoom|TestSetClientVisibleArea'
  ```

  Expected: PASS (9 tests).

- [ ] **Step 6: Commit**

  ```bash
  git add lok/document.go lok/render.go lok/render_test.go
  git commit -m "$(cat <<'EOF'
  feat(lok): InitializeForRendering + zoom + visible-area

  InitializeForRendering forwards LOK's args string and verifies
  getTileMode returns LOK_TILEMODE_BGRA (1); any other value
  surfaces as *LOKError. The check is cached on Document.tileModeReady
  so Paint*/Render* can gate on it without re-querying.

  SetClientZoom and SetClientVisibleArea are optional hints that
  don't require InitializeForRendering. SetClientVisibleArea
  refuses any TwipRect field outside int32 — LOK's ABI truncates
  silently otherwise.

  Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
  EOF
  )"
  ```

---

## Task 6: `PaintTile` + `PaintTileRaw` + part variants (TDD)

**Files:**
- Modify: `lok/render.go`
- Modify: `lok/render_test.go`

### 6.1 Failing tests

- [ ] **Step 1: Append to `lok/render_test.go`**

  ```go
  func TestPaintTileRaw_PassesTileArgs(t *testing.T) {
  	fb := &fakeBackend{tileMode: 1}
  	_, doc := loadFakeDoc(t, fb)
  	if err := doc.InitializeForRendering(""); err != nil {
  		t.Fatal(err)
  	}
  	buf := make([]byte, 4*256*256)
  	if err := doc.PaintTileRaw(buf, 256, 256, TwipRect{X: 10, Y: 20, W: 3000, H: 4000}); err != nil {
  		t.Fatal(err)
  	}
  	if len(fb.paintCalls) != 1 {
  		t.Fatalf("paintCalls: %d, want 1", len(fb.paintCalls))
  	}
  	got := fb.paintCalls[0]
  	want := fakePaint{pxW: 256, pxH: 256, x: 10, y: 20, w: 3000, h: 4000, bufLen: 4 * 256 * 256}
  	if got != want {
  		t.Errorf("got %+v, want %+v", got, want)
  	}
  }

  func TestPaintTileRaw_WrongBufferSizeErrors(t *testing.T) {
  	fb := &fakeBackend{tileMode: 1}
  	_, doc := loadFakeDoc(t, fb)
  	if err := doc.InitializeForRendering(""); err != nil {
  		t.Fatal(err)
  	}
  	// Too small.
  	err := doc.PaintTileRaw(make([]byte, 10), 256, 256, TwipRect{})
  	var lokErr *LOKError
  	if !errors.As(err, &lokErr) {
  		t.Errorf("too-small: want *LOKError, got %T %v", err, err)
  	}
  	// Too large — exact match required.
  	err = doc.PaintTileRaw(make([]byte, 4*256*256+1), 256, 256, TwipRect{})
  	if !errors.As(err, &lokErr) {
  		t.Errorf("too-large: want *LOKError, got %T %v", err, err)
  	}
  	if len(fb.paintCalls) != 0 {
  		t.Errorf("paintCalls should be empty on size-mismatch; got %d", len(fb.paintCalls))
  	}
  }

  func TestPaintTileRaw_WithoutInitializeErrors(t *testing.T) {
  	fb := &fakeBackend{tileMode: 1}
  	_, doc := loadFakeDoc(t, fb)
  	err := doc.PaintTileRaw(make([]byte, 4*256*256), 256, 256, TwipRect{})
  	var lokErr *LOKError
  	if !errors.As(err, &lokErr) || lokErr.Op != "PaintTile" {
  		t.Errorf("want *LOKError{Op: PaintTile}, got %T %v", err, err)
  	}
  }

  func TestPaintTileRaw_RangeCheck(t *testing.T) {
  	fb := &fakeBackend{tileMode: 1}
  	_, doc := loadFakeDoc(t, fb)
  	if err := doc.InitializeForRendering(""); err != nil {
  		t.Fatal(err)
  	}
  	err := doc.PaintTileRaw(make([]byte, 4), 1, 1, TwipRect{W: 1<<32 + 1})
  	var lokErr *LOKError
  	if !errors.As(err, &lokErr) {
  		t.Errorf("want *LOKError, got %T %v", err, err)
  	}
  	if len(fb.paintCalls) != 0 {
  		t.Error("paintCalls not empty after range error")
  	}
  }

  func TestPaintTile_AllocatesAndUnpremultiplies(t *testing.T) {
  	// Fake backend writes known premul BGRA into the caller's buffer;
  	// PaintTile should return NRGBA with the unpremultiplied values.
  	fb := &fakePaintingBackend{
  		fakeBackend: fakeBackend{tileMode: 1},
  		// Two pixels: opaque red, 50% red.
  		paintBytes: []byte{0, 0, 255, 255, 0, 0, 128, 128},
  	}
  	_, doc := loadFakeDocWithBackend(t, fb)
  	if err := doc.InitializeForRendering(""); err != nil {
  		t.Fatal(err)
  	}
  	img, err := doc.PaintTile(2, 1, TwipRect{W: 100, H: 50})
  	if err != nil {
  		t.Fatal(err)
  	}
  	want := []byte{255, 0, 0, 255, 255, 0, 0, 128}
  	if !bytes.Equal(img.Pix, want) {
  		t.Errorf("img.Pix=%v, want %v", img.Pix, want)
  	}
  	if img.Rect != image.Rect(0, 0, 2, 1) {
  		t.Errorf("img.Rect=%v, want (0,0)-(2,1)", img.Rect)
  	}
  }

  func TestPaintPartTileRaw_PassesPart(t *testing.T) {
  	fb := &fakeBackend{tileMode: 1}
  	_, doc := loadFakeDoc(t, fb)
  	if err := doc.InitializeForRendering(""); err != nil {
  		t.Fatal(err)
  	}
  	if err := doc.PaintPartTileRaw(make([]byte, 4*2*2), 3, 2, 2, TwipRect{W: 100, H: 100}); err != nil {
  		t.Fatal(err)
  	}
  	if len(fb.partPaintCalls) != 1 || fb.partPaintCalls[0].part != 3 {
  		t.Errorf("partPaintCalls=%+v", fb.partPaintCalls)
  	}
  }
  ```

  Add the helper types at the bottom of `lok/render_test.go`:

  ```go
  // fakePaintingBackend extends fakeBackend with a programmable tile
  // payload that PaintTile writes into the caller's buffer, so the
  // unpremultiply path has something deterministic to decode.
  type fakePaintingBackend struct {
  	fakeBackend
  	paintBytes []byte
  }

  func (f *fakePaintingBackend) DocumentPaintTile(_ documentHandle, buf []byte, pxW, pxH, x, y, w, h int) {
  	copy(buf, f.paintBytes)
  	f.paintCalls = append(f.paintCalls, fakePaint{pxW: pxW, pxH: pxH, x: x, y: y, w: w, h: h, bufLen: len(buf)})
  }

  // loadFakeDocWithBackend is loadFakeDoc for callers that need to
  // install a backend other than *fakeBackend (e.g. *fakePaintingBackend
  // which embeds it).
  func loadFakeDocWithBackend(t *testing.T, be backend) (*Office, *Document) {
  	t.Helper()
  	orig := currentBackend
  	t.Cleanup(func() { setBackend(orig); resetSingleton() })
  	setBackend(be)
  	resetSingleton()
  	o, err := New("/install")
  	if err != nil {
  		t.Fatal(err)
  	}
  	doc, err := o.Load("/tmp/x.odt")
  	if err != nil {
  		o.Close()
  		t.Fatal(err)
  	}
  	t.Cleanup(func() { doc.Close(); o.Close() })
  	return o, doc
  }
  ```

  Also add imports at the top of the file: `"bytes"` and `"image"`.

- [ ] **Step 2: Run — red**

  ```bash
  go test ./lok/... -run 'TestPaintTileRaw|TestPaintTile|TestPaintPartTile'
  ```

  Expected: undefined symbols.

### 6.2 Implement

- [ ] **Step 3: Append to `lok/render.go`**

  ```go
  // PaintTileRaw writes premultiplied BGRA (Cairo ARGB32; byte order
  // B, G, R, A with RGB premultiplied by A) into buf. len(buf) must
  // equal exactly 4*pxW*pxH — wrong-size buffers return *LOKError
  // without invoking LOK. InitializeForRendering must have been
  // called first.
  //
  // buf's backing array is pinned by the Go runtime for the duration
  // of a single synchronous cgo call; LOK does not retain the pointer.
  // Do not hand buf to long-lived Go structures that might outlive
  // the call stack and then race with GC.
  func (d *Document) PaintTileRaw(buf []byte, pxW, pxH int, r TwipRect) error {
  	if err := checkPaintBuf(buf, pxW, pxH); err != nil {
  		return err
  	}
  	if err := requireInt32Rect("PaintTile", r); err != nil {
  		return err
  	}
  	unlock, err := d.guard()
  	if err != nil {
  		return err
  	}
  	defer unlock()
  	if !d.tileModeReady {
  		return &LOKError{Op: "PaintTile", Detail: "InitializeForRendering not called"}
  	}
  	d.office.be.DocumentPaintTile(d.h, buf, pxW, pxH, int(r.X), int(r.Y), int(r.W), int(r.H))
  	return nil
  }

  // PaintPartTileRaw is PaintTileRaw for a specific part (sheet/page/
  // slide). mode is always 0 in the current binding; the LOK notes
  // mode (Impress) is not exposed yet.
  func (d *Document) PaintPartTileRaw(buf []byte, part, pxW, pxH int, r TwipRect) error {
  	if err := checkPaintBuf(buf, pxW, pxH); err != nil {
  		return err
  	}
  	if err := requireInt32Rect("PaintPartTile", r); err != nil {
  		return err
  	}
  	unlock, err := d.guard()
  	if err != nil {
  		return err
  	}
  	defer unlock()
  	if !d.tileModeReady {
  		return &LOKError{Op: "PaintPartTile", Detail: "InitializeForRendering not called"}
  	}
  	d.office.be.DocumentPaintPartTile(d.h, buf, part, 0, pxW, pxH, int(r.X), int(r.Y), int(r.W), int(r.H))
  	return nil
  }

  // PaintTile allocates a pxW×pxH NRGBA image, paints into a scratch
  // premul BGRA buffer via PaintTileRaw, and unpremultiplies into the
  // returned image. For hot paint loops, prefer PaintTileRaw and
  // reuse a buffer.
  func (d *Document) PaintTile(pxW, pxH int, r TwipRect) (*image.NRGBA, error) {
  	img := image.NewNRGBA(imageBoundsForTile(pxW, pxH))
  	raw := make([]byte, 4*pxW*pxH)
  	if err := d.PaintTileRaw(raw, pxW, pxH, r); err != nil {
  		return nil, err
  	}
  	unpremultiplyBGRAToNRGBA(img.Pix, raw, pxW, pxH)
  	return img, nil
  }

  // PaintPartTile is PaintTile for a specific part.
  func (d *Document) PaintPartTile(part, pxW, pxH int, r TwipRect) (*image.NRGBA, error) {
  	img := image.NewNRGBA(imageBoundsForTile(pxW, pxH))
  	raw := make([]byte, 4*pxW*pxH)
  	if err := d.PaintPartTileRaw(raw, part, pxW, pxH, r); err != nil {
  		return nil, err
  	}
  	unpremultiplyBGRAToNRGBA(img.Pix, raw, pxW, pxH)
  	return img, nil
  }

  // checkPaintBuf is the buffer-size precondition shared by the two
  // Raw paint methods. Returns *LOKError on mismatch so callers get
  // the typed error the rest of the binding uses.
  func checkPaintBuf(buf []byte, pxW, pxH int) error {
  	want := 4 * pxW * pxH
  	if len(buf) != want {
  		return &LOKError{Op: "PaintTile", Detail: fmt.Sprintf("buffer size mismatch: len=%d, want %d", len(buf), want)}
  	}
  	return nil
  }
  ```

- [ ] **Step 4: Run — green**

  ```bash
  go test -race ./lok/... -run 'TestPaintTileRaw|TestPaintTile|TestPaintPartTile'
  ```

  Expected: PASS (6 tests).

- [ ] **Step 5: Commit**

  ```bash
  git add lok/render.go lok/render_test.go
  git commit -m "$(cat <<'EOF'
  feat(lok): PaintTile(Raw) + PaintPartTile(Raw)

  Raw methods take a caller-provided BGRA buffer; wrong-size and
  out-of-int32 rect fields surface as *LOKError without calling LOK.
  The tileModeReady flag from InitializeForRendering gates every
  paint; missing init returns *LOKError{Op:\"PaintTile\"} with
  a clear message.

  Convenience PaintTile(pxW, pxH, r) allocates internally, paints
  premul BGRA, and unpremultiplies into *image.NRGBA via pixels.go.

  Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
  EOF
  )"
  ```

---

## Task 7: `RenderSearchResult(Raw)` + `RenderShapeSelection` (TDD)

**Files:**
- Modify: `lok/render.go`
- Modify: `lok/render_test.go`

### 7.1 Failing tests

- [ ] **Step 1: Append to `lok/render_test.go`**

  ```go
  func TestRenderSearchResultRaw_NoMatch(t *testing.T) {
  	fb := &fakeBackend{tileMode: 1} // searchResultOK defaults to false
  	_, doc := loadFakeDoc(t, fb)
  	if err := doc.InitializeForRendering(""); err != nil {
  		t.Fatal(err)
  	}
  	buf, w, h, err := doc.RenderSearchResultRaw("nope")
  	if err != nil {
  		t.Fatal(err)
  	}
  	if buf != nil || w != 0 || h != 0 {
  		t.Errorf("no-match: got (%v, %d, %d), want (nil, 0, 0)", buf, w, h)
  	}
  	if fb.lastSearchQuery != "nope" {
  		t.Errorf("query not forwarded: %q", fb.lastSearchQuery)
  	}
  }

  func TestRenderSearchResultRaw_Match(t *testing.T) {
  	bgra := []byte{0, 0, 255, 255} // opaque red pixel
  	fb := &fakeBackend{
  		tileMode:        1,
  		searchResultBuf: bgra,
  		searchResultPxW: 1,
  		searchResultPxH: 1,
  		searchResultOK:  true,
  	}
  	_, doc := loadFakeDoc(t, fb)
  	if err := doc.InitializeForRendering(""); err != nil {
  		t.Fatal(err)
  	}
  	buf, w, h, err := doc.RenderSearchResultRaw("q")
  	if err != nil {
  		t.Fatal(err)
  	}
  	if !bytes.Equal(buf, bgra) || w != 1 || h != 1 {
  		t.Errorf("got (%v, %d, %d)", buf, w, h)
  	}
  }

  func TestRenderSearchResult_UnpremultipliesToNRGBA(t *testing.T) {
  	fb := &fakeBackend{
  		tileMode:        1,
  		searchResultBuf: []byte{0, 0, 255, 255}, // red
  		searchResultPxW: 1,
  		searchResultPxH: 1,
  		searchResultOK:  true,
  	}
  	_, doc := loadFakeDoc(t, fb)
  	if err := doc.InitializeForRendering(""); err != nil {
  		t.Fatal(err)
  	}
  	img, err := doc.RenderSearchResult("q")
  	if err != nil {
  		t.Fatal(err)
  	}
  	if img == nil {
  		t.Fatal("img is nil on match")
  	}
  	want := []byte{255, 0, 0, 255}
  	if !bytes.Equal(img.Pix, want) {
  		t.Errorf("got %v, want %v", img.Pix, want)
  	}
  }

  func TestRenderSearchResult_NoMatch(t *testing.T) {
  	_, doc := loadFakeDoc(t, &fakeBackend{tileMode: 1})
  	if err := doc.InitializeForRendering(""); err != nil {
  		t.Fatal(err)
  	}
  	img, err := doc.RenderSearchResult("nope")
  	if err != nil {
  		t.Fatal(err)
  	}
  	if img != nil {
  		t.Errorf("no-match: img=%v, want nil", img)
  	}
  }

  func TestRenderSearchResult_RequiresInitialize(t *testing.T) {
  	_, doc := loadFakeDoc(t, &fakeBackend{tileMode: 1, searchResultOK: true})
  	_, err := doc.RenderSearchResult("q")
  	var lokErr *LOKError
  	if !errors.As(err, &lokErr) {
  		t.Errorf("want *LOKError, got %T %v", err, err)
  	}
  }

  func TestRenderShapeSelection_Empty(t *testing.T) {
  	_, doc := loadFakeDoc(t, &fakeBackend{tileMode: 1})
  	if err := doc.InitializeForRendering(""); err != nil {
  		t.Fatal(err)
  	}
  	got, err := doc.RenderShapeSelection()
  	if err != nil {
  		t.Fatal(err)
  	}
  	if got != nil {
  		t.Errorf("empty: got %v, want nil", got)
  	}
  }

  func TestRenderShapeSelection_ReturnsBytes(t *testing.T) {
  	payload := []byte("<svg/>")
  	fb := &fakeBackend{tileMode: 1, shapeSelection: payload}
  	_, doc := loadFakeDoc(t, fb)
  	if err := doc.InitializeForRendering(""); err != nil {
  		t.Fatal(err)
  	}
  	got, err := doc.RenderShapeSelection()
  	if err != nil {
  		t.Fatal(err)
  	}
  	if !bytes.Equal(got, payload) {
  		t.Errorf("got %q, want %q", got, payload)
  	}
  }
  ```

- [ ] **Step 2: Run — red**

  ```bash
  go test ./lok/... -run 'TestRenderSearchResult|TestRenderShapeSelection'
  ```

  Expected: undefined symbols.

### 7.2 Implement

- [ ] **Step 3: Append to `lok/render.go`**

  ```go
  // RenderSearchResultRaw asks LOK to render the first match of query
  // as a premultiplied BGRA bitmap. query is the LOK `.uno:SearchItem`
  // JSON payload. Returns (nil, 0, 0, nil) on no match — the error
  // path is reserved for binding-side failures like a closed document
  // or missing InitializeForRendering.
  func (d *Document) RenderSearchResultRaw(query string) (buf []byte, pxW, pxH int, err error) {
  	unlock, gerr := d.guard()
  	if gerr != nil {
  		return nil, 0, 0, gerr
  	}
  	defer unlock()
  	if !d.tileModeReady {
  		return nil, 0, 0, &LOKError{Op: "RenderSearchResult", Detail: "InitializeForRendering not called"}
  	}
  	b, w, h, ok := d.office.be.DocumentRenderSearchResult(d.h, query)
  	if !ok {
  		return nil, 0, 0, nil
  	}
  	return b, w, h, nil
  }

  // RenderSearchResult is the *image.NRGBA convenience form of
  // RenderSearchResultRaw. Returns (nil, nil) when there's no match.
  func (d *Document) RenderSearchResult(query string) (*image.NRGBA, error) {
  	buf, pxW, pxH, err := d.RenderSearchResultRaw(query)
  	if err != nil || buf == nil {
  		return nil, err
  	}
  	img := image.NewNRGBA(imageBoundsForTile(pxW, pxH))
  	unpremultiplyBGRAToNRGBA(img.Pix, buf, pxW, pxH)
  	return img, nil
  }

  // RenderShapeSelection returns LOK's bytes for the current shape
  // selection (SVG in practice on LO 24.8, but the binding does not
  // promise a format). Returns (nil, nil) when nothing is selected.
  func (d *Document) RenderShapeSelection() ([]byte, error) {
  	unlock, err := d.guard()
  	if err != nil {
  		return nil, err
  	}
  	defer unlock()
  	if !d.tileModeReady {
  		return nil, &LOKError{Op: "RenderShapeSelection", Detail: "InitializeForRendering not called"}
  	}
  	return d.office.be.DocumentRenderShapeSelection(d.h), nil
  }
  ```

- [ ] **Step 4: Run — green**

  ```bash
  go test -race ./lok/... -run 'TestRenderSearchResult|TestRenderShapeSelection'
  ```

  Expected: PASS (7 tests).

- [ ] **Step 5: Commit**

  ```bash
  git add lok/render.go lok/render_test.go
  git commit -m "$(cat <<'EOF'
  feat(lok): RenderSearchResult(Raw) + RenderShapeSelection

  Raw form returns premul BGRA + px dimensions; convenience form
  wraps with unpremultiply into *image.NRGBA. Both gate on
  tileModeReady. No-match on search is (nil, nil) — not an error.
  RenderShapeSelection returns []byte unchanged; (nil, nil) when
  nothing is selected.

  Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
  EOF
  )"
  ```

---

## Task 8: After-close table test

**Files:**
- Modify: `lok/render_test.go`

### Step 1: Add the table

- [ ] **Step 1: Append**

  ```go
  func TestRenderMethods_AfterCloseErrors(t *testing.T) {
  	cases := []struct {
  		name string
  		call func(*Document) error
  	}{
  		{"InitializeForRendering", func(d *Document) error { return d.InitializeForRendering("") }},
  		{"SetClientZoom", func(d *Document) error { return d.SetClientZoom(1, 1, 1, 1) }},
  		{"SetClientVisibleArea", func(d *Document) error { return d.SetClientVisibleArea(TwipRect{}) }},
  		{"PaintTileRaw", func(d *Document) error { return d.PaintTileRaw(make([]byte, 4), 1, 1, TwipRect{}) }},
  		{"PaintTile", func(d *Document) error { _, err := d.PaintTile(1, 1, TwipRect{}); return err }},
  		{"PaintPartTileRaw", func(d *Document) error { return d.PaintPartTileRaw(make([]byte, 4), 0, 1, 1, TwipRect{}) }},
  		{"PaintPartTile", func(d *Document) error { _, err := d.PaintPartTile(0, 1, 1, TwipRect{}); return err }},
  		{"RenderSearchResultRaw", func(d *Document) error { _, _, _, err := d.RenderSearchResultRaw("q"); return err }},
  		{"RenderSearchResult", func(d *Document) error { _, err := d.RenderSearchResult("q"); return err }},
  		{"RenderShapeSelection", func(d *Document) error { _, err := d.RenderShapeSelection(); return err }},
  	}
  	for _, tc := range cases {
  		t.Run(tc.name, func(t *testing.T) {
  			_, doc := loadFakeDoc(t, &fakeBackend{tileMode: 1})
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
  go test -race ./lok/... -run TestRenderMethods_AfterCloseErrors
  ```

  Expected: PASS (10 subtests).

- [ ] **Step 3: Coverage gate**

  ```bash
  make cover-gate
  ```

  Expected: ≥ 90.0%.

- [ ] **Step 4: Commit**

  ```bash
  git add lok/render_test.go
  git commit -m "$(cat <<'EOF'
  test(lok): after-close table test for all Phase 6 methods

  Every new public method on Document — including the Raw and
  convenience paint/render variants — returns ErrClosed when
  invoked after Close(). Mirrors Phase 5's
  TestPartMethods_AfterCloseErrors shape.

  Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
  EOF
  )"
  ```

---

## Task 9: Integration tests

**Files:**
- Modify: `lok/integration_test.go`

Constraint recap (from `memory/feedback_lok_singleton_per_process.md`):
1. All subtests share ONE `New`/`Close` pair in `TestIntegration_FullLifecycle`.
2. `LoadFromReader(doc2)` MUST stay last — the two-docs + DestroyView ordering hazard.
3. `SetView(initialView)` MUST precede `DestroyView(newView)` — already wired.

Place the new subtests between the existing `PartPageRectangles` block and the `LoadFromReader(doc2)` block.

### Step 1: Add render subtests

- [ ] **Step 1: Modify `lok/integration_test.go`**

  Find the line that begins the `LoadFromReader` block (the comment starts with `// LoadFromReader deliberately comes last.`). Insert the following immediately BEFORE that comment:

  ```go
  	// Rendering round-trip on doc.

  	if err := doc.InitializeForRendering(""); err != nil {
  		t.Fatalf("InitializeForRendering: %v", err)
  	}
  	if err := doc.SetClientZoom(256, 256, 1440, 1440); err != nil {
  		t.Errorf("SetClientZoom: %v", err)
  	}
  	if err := doc.SetClientVisibleArea(TwipRect{X: 0, Y: 0, W: 14400, H: 14400}); err != nil {
  		t.Errorf("SetClientVisibleArea: %v", err)
  	}

  	// PaintTile: expect non-nil image; check some pixel was drawn so
  	// we know the path isn't silently returning an all-zero buffer.
  	img, err := doc.PaintTile(256, 256, TwipRect{X: 0, Y: 0, W: 14400, H: 14400})
  	if err != nil {
  		t.Fatalf("PaintTile: %v", err)
  	}
  	if img == nil {
  		t.Fatal("PaintTile returned nil image")
  	}
  	var nonZero int
  	for _, b := range img.Pix {
  		if b != 0 {
  			nonZero++
  		}
  	}
  	if nonZero == 0 {
  		t.Error("PaintTile buffer is entirely zero — nothing painted?")
  	}

  	// PaintTileRaw with correct buffer.
  	rawBuf := make([]byte, 4*256*256)
  	if err := doc.PaintTileRaw(rawBuf, 256, 256, TwipRect{X: 0, Y: 0, W: 14400, H: 14400}); err != nil {
  		t.Errorf("PaintTileRaw: %v", err)
  	}

  	// PaintTileRaw with wrong-size buffer must return *LOKError
  	// without invoking LOK.
  	if err := doc.PaintTileRaw(make([]byte, 10), 256, 256, TwipRect{}); err == nil {
  		t.Error("PaintTileRaw with wrong buffer size: want *LOKError, got nil")
  	} else {
  		var lokErr *LOKError
  		if !errors.As(err, &lokErr) {
  			t.Errorf("PaintTileRaw wrong-size: want *LOKError, got %T %v", err, err)
  		}
  	}

  	// PaintPartTile — only sensible when parts > 0. Writer returns 0.
  	if nParts > 0 {
  		if _, err := doc.PaintPartTile(0, 256, 256, TwipRect{X: 0, Y: 0, W: 14400, H: 14400}); err != nil {
  			t.Errorf("PaintPartTile(0): %v", err)
  		}
  	}

  	// RenderSearchResult: pass a plausible SearchItem payload; tolerate
  	// both outcomes — a zero-match result and a real hit are both legal.
  	searchQuery := `{"SearchItem.SearchString":{"type":"string","value":"LibreOffice"},` +
  		`"SearchItem.Backward":{"type":"boolean","value":"false"},` +
  		`"SearchItem.Command":{"type":"long","value":"0"}}`
  	sImg, err := doc.RenderSearchResult(searchQuery)
  	if err != nil {
  		t.Errorf("RenderSearchResult: %v", err)
  	}
  	if sImg == nil {
  		t.Log("RenderSearchResult: no match (acceptable — depends on fixture text)")
  	}

  	// RenderShapeSelection with no selection — expect (nil, nil).
  	shape, err := doc.RenderShapeSelection()
  	if err != nil {
  		t.Errorf("RenderShapeSelection: %v", err)
  	}
  	if shape != nil {
  		t.Logf("RenderShapeSelection returned %d bytes without a selection (LO may emit empty SVG envelope)", len(shape))
  	}
  ```

- [ ] **Step 2: Run**

  ```bash
  make test-integration
  ```

  Expected: green. If `make test-integration` fails on the render subtest with a SIGWINCH/SA_ONSTACK crash, the most likely cause is ordering: verify the render block is between `PartPageRectangles` and `LoadFromReader`. If it fails with `RenderShapeSelection returned bytes without selection`, that's `t.Logf`, not a failure — proceed.

- [ ] **Step 3: Commit**

  ```bash
  git add lok/integration_test.go
  git commit -m "$(cat <<'EOF'
  test(lok): integration coverage for Phase 6 rendering

  Exercises InitializeForRendering → SetClientZoom →
  SetClientVisibleArea → PaintTile/PaintTileRaw → PaintPartTile
  (when parts>0) → RenderSearchResult → RenderShapeSelection on
  the real LO backend via the existing TestIntegration_FullLifecycle
  singleton. Subtests are placed between the Phase 5
  PartPageRectangles block and the LoadFromReader(doc2) block to
  respect the two-docs-plus-DestroyView layout hazard documented
  on 2026-04-23.

  Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
  EOF
  )"
  ```

---

## Task 10: PR

- [ ] **Step 1: Final verification**

  ```bash
  make all && make cover-gate && make test-integration
  ```

  Expected: all green, coverage ≥ 90%.

- [ ] **Step 2: Push + PR**

  ```bash
  git push -u origin feat/rendering
  gh pr create --title "feat(lok): Phase 6 — Rendering (tiles, search, shape selection)" --body "$(cat <<'EOF'
  ## Summary
  - Adds the rendering surface of the LOK binding per `docs/superpowers/specs/2026-04-24-phase-6-rendering-design.md`.
  - 11 new public `Document` methods: `InitializeForRendering`, `SetClientZoom`, `SetClientVisibleArea`, `PaintTile(Raw)`, `PaintPartTile(Raw)`, `RenderSearchResult(Raw)`, `RenderShapeSelection`.
  - New pure-Go `pixels.go` unpremultiplies BGRA → NRGBA with golden-byte tests.
  - Tile-mode verified once in `InitializeForRendering`; non-BGRA surfaces as `*LOKError`.
  - Buffer-size and int32 range checks on `PaintTileRaw` return typed errors without calling LOK.

  ## Test plan
  - [ ] `make test` — unit tests, race-enabled, ≥ 90% coverage.
  - [ ] `make cover-gate` — explicit gate passes.
  - [ ] `make test-integration` — real LO paint + search + shape selection.
  - [ ] Spot-check: PaintTile(256, 256, full-page) on testdata/hello.odt yields a non-zero NRGBA buffer.

  🤖 Generated with [Claude Code](https://claude.com/claude-code)
  EOF
  )"
  ```

  Expected: PR URL returned.

---

## Out of scope (deferred)

- HiDPI / `paintTileDX` / `paintWindowDPI` — Phase 10.
- Window-surface painting — Phase 10.
- `renderFont` glyph rendering — Phase 10.
- Impress notes mode on `paintPartTile` — not exposed; seam argument present for future work.
- Pixel-conversion LUT / SIMD — measure first; private change if needed.
