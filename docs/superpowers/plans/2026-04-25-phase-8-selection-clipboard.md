# Phase 8 — Selection & Clipboard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add typed Go bindings for LibreOfficeKit's selection and
per-view clipboard API on `*lok.Document`, covering all eight methods
in the Phase 8 spec section plus a `GetSelectionKind` probe.

**Architecture:** Four-layer binding (public `lok` → `backend`
interface → `realBackend` → `internal/lokc` cgo), identical to Phases
3–7. Clipboard triple-array memory juggling is done in a C helper so
Go never allocates into C storage through `unsafe`. Integration tests
drive selections via the Phase 7 UNO helpers and poll with a timeout
(Phase 9 callbacks not yet available).

**Tech Stack:** Go + cgo on `linux || darwin`, LibreOfficeKit C ABI,
`go test` + the `lok_integration` build tag.

**Branch:** `feat/selection-clipboard` (already created off `main`).

**Spec:** `docs/superpowers/specs/2026-04-24-phase-8-selection-clipboard-design.md`

---

## File Structure

Files created:

- `internal/lokc/selection.go` — cgo wrappers for the six selection /
  block entry points. One `go_doc_*` static C shim per call. Each
  wrapper is nil-safe (returns zero value when the handle or vtable
  slot is NULL).
- `internal/lokc/selection_test.go` — unit tests using the calloc'd
  fake-LOK handle with `pClass == NULL` (pre-existing pattern).
- `internal/lokc/clipboard.go` — `DocumentGetClipboard` and
  `DocumentSetClipboard` wrappers plus the `go_doc_get_clipboard`,
  `go_doc_free_clipboard`, and `go_doc_set_clipboard` C helpers.
- `internal/lokc/clipboard_test.go` — unit tests for the Go side of
  the wrappers (nil-handle / nil-vtable guards).
- `lok/selection.go` — `SetTextSelectionType`,
  `SetGraphicSelectionType`, `SelectionKind` enums plus the seven
  selection methods on `*Document`.
- `lok/selection_test.go` — unit tests via `fakeBackend`.
- `lok/clipboard.go` — `ClipboardItem`, `validateMime`, and the two
  clipboard methods.
- `lok/clipboard_test.go` — unit tests via `fakeBackend`.

Files modified:

- `lok/errors.go` — add `ErrUnsupported` sentinel.
- `lok/backend.go` — add 11 interface methods (seven selection, two
  clipboard, one selection-type getter, one typed-and-text getter).
- `lok/real_backend.go` — add forwarders for each new interface
  method.
- `lok/office_test.go` — add `fakeBackend` fields and methods for the
  new interface surface.
- `lok/integration_test.go` — add `TestSelectionRoundTrip` and
  `TestClipboardRoundTrip`.
- `lok/real_backend_test.go` — add smoke calls for the new
  realBackend forwarders so coverage reaches them when
  `-tags=lok_integration` is set.

---

## Task 1: `ErrUnsupported` sentinel

**Files:**
- Modify: `lok/errors.go`
- Test: `lok/errors_test.go` (create if missing, otherwise append)

- [ ] **Step 1: Check if `lok/errors_test.go` exists**

Run: `ls lok/errors_test.go 2>/dev/null || echo missing`
Expected: either a path (append to it) or `missing` (create it in
Step 2).

- [ ] **Step 2: Write the failing test**

If `lok/errors_test.go` exists, append the test below. Otherwise
create it with this contents:

```go
package lok

import (
	"errors"
	"testing"
)

func TestErrUnsupported_Sentinel(t *testing.T) {
	// A fresh wrap of ErrUnsupported must still compare equal with
	// errors.Is, and ErrUnsupported must be distinct from the other
	// known sentinels.
	wrapped := errors.Join(ErrUnsupported, errors.New("ctx"))
	if !errors.Is(wrapped, ErrUnsupported) {
		t.Errorf("errors.Is(wrapped, ErrUnsupported) = false, want true")
	}
	if errors.Is(ErrUnsupported, ErrClosed) {
		t.Errorf("ErrUnsupported must not alias ErrClosed")
	}
	if ErrUnsupported.Error() == "" {
		t.Error("ErrUnsupported.Error() must not be empty")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./lok -run TestErrUnsupported_Sentinel -v`
Expected: FAIL with `undefined: ErrUnsupported`.

- [ ] **Step 4: Implement the sentinel**

In `lok/errors.go`, inside the existing `var (...)` block, append:

```go
	ErrUnsupported         = errors.New("lok: operation not supported by this LibreOfficeKit build")
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./lok -run TestErrUnsupported_Sentinel -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add lok/errors.go lok/errors_test.go
git commit -m "feat(lok): add ErrUnsupported sentinel for NULL LOK vtable slots"
```

---

## Task 2: `internal/lokc` selection setters + `resetSelection`

Fire-and-forget wrappers for the four selection methods that return
`void`.

**Files:**
- Create: `internal/lokc/selection.go`
- Create: `internal/lokc/selection_test.go`

- [ ] **Step 1: Write failing tests for the four setters**

Create `internal/lokc/selection_test.go`:

```go
//go:build linux || darwin

package lokc

import "testing"

// newFakeDoc wraps NewFakeDocumentHandle + FreeFakeDocumentHandle in
// a t.Cleanup so each test can say `d := newFakeDoc(t)` in one line.
// Uses the existing helpers in document_test_helper.go, which yield
// a calloc'd LibreOfficeKitDocument with pClass == NULL.
func newFakeDoc(t *testing.T) DocumentHandle {
	t.Helper()
	d := NewFakeDocumentHandle()
	t.Cleanup(func() { FreeFakeDocumentHandle(d) })
	return d
}

// All four setters are fire-and-forget; the only observable behaviour
// from Go is that a zero / fake-pClass handle does not crash.

func TestDocumentSetTextSelection_NilSafe(t *testing.T) {
	DocumentSetTextSelection(DocumentHandle{}, 0, 0, 0) // zero handle
	h := newFakeDoc(t)
	DocumentSetTextSelection(h, 1, 100, 200) // pClass == NULL
}

func TestDocumentResetSelection_NilSafe(t *testing.T) {
	DocumentResetSelection(DocumentHandle{})
	DocumentResetSelection(newFakeDoc(t))
}

func TestDocumentSetGraphicSelection_NilSafe(t *testing.T) {
	DocumentSetGraphicSelection(DocumentHandle{}, 0, 0, 0)
	DocumentSetGraphicSelection(newFakeDoc(t), 1, 10, 20)
}

func TestDocumentSetBlockedCommandList_NilSafe(t *testing.T) {
	DocumentSetBlockedCommandList(DocumentHandle{}, 0, ".uno:Save")
	DocumentSetBlockedCommandList(newFakeDoc(t), 1, ".uno:Save,.uno:SaveAs")
}
```

The underlying helpers (`NewFakeDocumentHandle` /
`FreeFakeDocumentHandle`) live in
`internal/lokc/document_test_helper.go` and are already used by
Phase 6/7 tests. `newFakeDoc(t)` is a convenience wrapper declared
once at the top of this test file.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/lokc -run 'TestDocument(SetText|Reset|SetGraphic|SetBlocked)Selection' -v`
Expected: FAIL with `undefined: DocumentSetTextSelection` (and the
other three).

- [ ] **Step 3: Implement the four wrappers**

Create `internal/lokc/selection.go`:

```go
//go:build linux || darwin

package lokc

/*
#cgo CFLAGS: -I${SRCDIR}/../../third_party/lok -DLOK_USE_UNSTABLE_API
#include <stdlib.h>
#include "LibreOfficeKit/LibreOfficeKit.h"

static void go_doc_set_text_selection(LibreOfficeKitDocument* d, int typ, int x, int y) {
    if (d == NULL || d->pClass == NULL || d->pClass->setTextSelection == NULL) return;
    d->pClass->setTextSelection(d, typ, x, y);
}
static void go_doc_reset_selection(LibreOfficeKitDocument* d) {
    if (d == NULL || d->pClass == NULL || d->pClass->resetSelection == NULL) return;
    d->pClass->resetSelection(d);
}
static void go_doc_set_graphic_selection(LibreOfficeKitDocument* d, int typ, int x, int y) {
    if (d == NULL || d->pClass == NULL || d->pClass->setGraphicSelection == NULL) return;
    d->pClass->setGraphicSelection(d, typ, x, y);
}
static void go_doc_set_blocked_command_list(LibreOfficeKitDocument* d, int viewId, const char* csv) {
    if (d == NULL || d->pClass == NULL || d->pClass->setBlockedCommandList == NULL) return;
    d->pClass->setBlockedCommandList(d, viewId, csv);
}
*/
import "C"

import "unsafe"

// DocumentSetTextSelection forwards to pClass->setTextSelection.
// typ is LOK_SETTEXTSELECTION_START|END|RESET; x, y are twips.
func DocumentSetTextSelection(d DocumentHandle, typ, x, y int) {
	if !d.IsValid() {
		return
	}
	C.go_doc_set_text_selection(d.p, C.int(typ), C.int(x), C.int(y))
}

// DocumentResetSelection forwards to pClass->resetSelection.
func DocumentResetSelection(d DocumentHandle) {
	if !d.IsValid() {
		return
	}
	C.go_doc_reset_selection(d.p)
}

// DocumentSetGraphicSelection forwards to pClass->setGraphicSelection.
// typ is LOK_SETGRAPHICSELECTION_START|END; x, y are twips.
func DocumentSetGraphicSelection(d DocumentHandle, typ, x, y int) {
	if !d.IsValid() {
		return
	}
	C.go_doc_set_graphic_selection(d.p, C.int(typ), C.int(x), C.int(y))
}

// DocumentSetBlockedCommandList forwards to
// pClass->setBlockedCommandList. csv is a comma-separated list of
// .uno:* command names.
func DocumentSetBlockedCommandList(d DocumentHandle, viewID int, csv string) {
	if !d.IsValid() {
		return
	}
	ccsv := C.CString(csv)
	defer C.free(unsafe.Pointer(ccsv))
	C.go_doc_set_blocked_command_list(d.p, C.int(viewID), ccsv)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/lokc -run 'TestDocument(SetText|Reset|SetGraphic|SetBlocked)Selection' -v`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/lokc/selection.go internal/lokc/selection_test.go
git commit -m "feat(lokc): add cgo wrappers for selection setters + reset"
```

---

## Task 3: `internal/lokc` — `DocumentGetTextSelection`

Wraps `pClass->getTextSelection`, which returns a heap-allocated text
string and writes a heap-allocated usedMime string into a `char**`
out-parameter. Both must be `free`'d.

**Files:**
- Modify: `internal/lokc/selection.go`
- Modify: `internal/lokc/selection_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/lokc/selection_test.go`:

```go
func TestDocumentGetTextSelection_NilSafe(t *testing.T) {
	// Zero handle and fake-pClass both yield empty strings without crashing.
	if text, mime := DocumentGetTextSelection(DocumentHandle{}, "text/plain"); text != "" || mime != "" {
		t.Errorf("zero handle: got (%q, %q), want empty strings", text, mime)
	}
	h := newFakeDoc(t)
	if text, mime := DocumentGetTextSelection(h, "text/plain"); text != "" || mime != "" {
		t.Errorf("nil pClass: got (%q, %q), want empty strings", text, mime)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/lokc -run TestDocumentGetTextSelection_NilSafe -v`
Expected: FAIL with `undefined: DocumentGetTextSelection`.

- [ ] **Step 3: Implement the wrapper**

In `internal/lokc/selection.go`, inside the C preamble add:

```c
static char* go_doc_get_text_selection(LibreOfficeKitDocument* d, const char* mime, char** usedMime) {
    if (d == NULL || d->pClass == NULL || d->pClass->getTextSelection == NULL) {
        if (usedMime != NULL) *usedMime = NULL;
        return NULL;
    }
    return d->pClass->getTextSelection(d, mime, usedMime);
}
```

Below the setters, add:

```go
// DocumentGetTextSelection copies the current text selection as the
// requested mime type. Returns (text, usedMime). Both strings are
// empty when LOK has nothing to return or the vtable slot is NULL.
func DocumentGetTextSelection(d DocumentHandle, mimeType string) (string, string) {
	if !d.IsValid() {
		return "", ""
	}
	cmime := C.CString(mimeType)
	defer C.free(unsafe.Pointer(cmime))
	var usedMime *C.char
	text := C.go_doc_get_text_selection(d.p, cmime, &usedMime)
	return copyAndFree(text), copyAndFree(usedMime)
}
```

(`copyAndFree` is the existing helper in `internal/lokc/errstr.go` —
nil-safe, returns `""` when the pointer is nil, otherwise copies and
`free`'s.)

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/lokc -run TestDocumentGetTextSelection_NilSafe -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/lokc/selection.go internal/lokc/selection_test.go
git commit -m "feat(lokc): add DocumentGetTextSelection wrapper"
```

---

## Task 4: `internal/lokc` — `DocumentGetSelectionType` and `DocumentGetSelectionTypeAndText`

The plain `getSelectionType` returns an `int`. The newer
`getSelectionTypeAndText` (LO 7.4+) returns the same int plus text and
usedMime in out-parameters. When the 7.4 slot is NULL, surface it as
`ErrUnsupported`; the plain getter is surfaced as "slot missing →
-1".

**Files:**
- Modify: `internal/lokc/selection.go`
- Modify: `internal/lokc/selection_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/lokc/selection_test.go`:

```go
func TestDocumentGetSelectionType_NilSafe(t *testing.T) {
	if got := DocumentGetSelectionType(DocumentHandle{}); got != -1 {
		t.Errorf("zero handle: got %d, want -1", got)
	}
	if got := DocumentGetSelectionType(newFakeDoc(t)); got != -1 {
		t.Errorf("nil pClass: got %d, want -1", got)
	}
}

func TestDocumentGetSelectionTypeAndText_UnsupportedOnNilSlot(t *testing.T) {
	// Zero handle maps to Unsupported (the slot is effectively NULL).
	kind, text, mime, err := DocumentGetSelectionTypeAndText(DocumentHandle{}, "text/plain")
	if err == nil || err != ErrUnsupported {
		t.Errorf("zero handle: err=%v, want ErrUnsupported", err)
	}
	if kind != -1 || text != "" || mime != "" {
		t.Errorf("zero handle: non-zero out values (%d, %q, %q)", kind, text, mime)
	}
	kind, text, mime, err = DocumentGetSelectionTypeAndText(newFakeDoc(t), "text/plain")
	if err != ErrUnsupported {
		t.Errorf("nil pClass: err=%v, want ErrUnsupported", err)
	}
	if kind != -1 || text != "" || mime != "" {
		t.Errorf("nil pClass: non-zero out values (%d, %q, %q)", kind, text, mime)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/lokc -run 'TestDocumentGetSelectionType' -v`
Expected: FAIL with `undefined: DocumentGetSelectionType` (and the
7.4 variant).

- [ ] **Step 3: Implement the wrappers**

In the C preamble of `internal/lokc/selection.go`, add:

```c
static int go_doc_get_selection_type(LibreOfficeKitDocument* d) {
    if (d == NULL || d->pClass == NULL || d->pClass->getSelectionType == NULL) return -1;
    return d->pClass->getSelectionType(d);
}

// Returns:
//   0 when the slot is NULL (unsupported)
//   1 when the call was made; *outKind, *outText, *outMime are populated
static int go_doc_get_selection_type_and_text(LibreOfficeKitDocument* d,
                                              const char* mime,
                                              int* outKind,
                                              char** outText,
                                              char** outMime) {
    if (d == NULL || d->pClass == NULL || d->pClass->getSelectionTypeAndText == NULL) {
        *outKind = -1;
        *outText = NULL;
        *outMime = NULL;
        return 0;
    }
    *outKind = d->pClass->getSelectionTypeAndText(d, mime, outText, outMime);
    return 1;
}
```

Add a new sentinel and the two Go wrappers (place the sentinel with
the existing `ErrSaveFailed` / `ErrNilDocument` near the top of
`document.go` — but cgo packages can keep sentinels per-file, so
adding them in `selection.go` is fine):

```go
// ErrUnsupported is returned when the LOK function pointer for an
// operation is NULL on the loaded LibreOffice build. The public
// lok.ErrUnsupported sentinel wraps this.
var ErrUnsupported = errors.New("lokc: LOK vtable slot is NULL")
```

(Add `"errors"` to the imports in `internal/lokc/selection.go`.)

Then:

```go
// DocumentGetSelectionType returns the LOK_SELTYPE_* value for the
// current selection, or -1 when the handle or vtable slot is NULL.
func DocumentGetSelectionType(d DocumentHandle) int {
	if !d.IsValid() {
		return -1
	}
	return int(C.go_doc_get_selection_type(d.p))
}

// DocumentGetSelectionTypeAndText reads both the selection kind and
// the selected text in one LOK call (LO 7.4+). Returns ErrUnsupported
// when the pClass slot is NULL.
func DocumentGetSelectionTypeAndText(d DocumentHandle, mimeType string) (kind int, text, usedMime string, err error) {
	if !d.IsValid() {
		return -1, "", "", ErrUnsupported
	}
	cmime := C.CString(mimeType)
	defer C.free(unsafe.Pointer(cmime))
	var ck C.int
	var cText, cMime *C.char
	ok := C.go_doc_get_selection_type_and_text(d.p, cmime, &ck, &cText, &cMime)
	if ok == 0 {
		return -1, "", "", ErrUnsupported
	}
	return int(ck), copyAndFree(cText), copyAndFree(cMime), nil
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/lokc -run 'TestDocumentGetSelectionType' -v`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/lokc/selection.go internal/lokc/selection_test.go
git commit -m "feat(lokc): add getSelectionType + getSelectionTypeAndText"
```

---

## Task 5: `internal/lokc` — `DocumentGetClipboard`

The C side: `getClipboard` takes a `NULL`-terminated list of
requested mimes (or `NULL` for "all offered"), and returns three
parallel C-heap arrays — `char **outMimes`, `size_t *outSizes`,
`char **outStreams` — plus a count. Every element in every array must
be `free`'d, plus the arrays themselves.

**Files:**
- Create: `internal/lokc/clipboard.go`
- Create: `internal/lokc/clipboard_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/lokc/clipboard_test.go`. Note: `newFakeDoc` was
declared at the top of `internal/lokc/selection_test.go` in Task 2;
since both `_test.go` files in the same package share helpers,
`clipboard_test.go` can call `newFakeDoc(t)` directly without
redefining.

```go
//go:build linux || darwin

package lokc

import "testing"

func TestDocumentGetClipboard_NilSafe(t *testing.T) {
	items, err := DocumentGetClipboard(DocumentHandle{}, nil)
	if err == nil || err != ErrUnsupported {
		t.Errorf("zero handle: err=%v, want ErrUnsupported", err)
	}
	if items != nil {
		t.Errorf("zero handle: items=%v, want nil", items)
	}

	items, err = DocumentGetClipboard(newFakeDoc(t), []string{"text/plain"})
	if err != ErrUnsupported {
		t.Errorf("nil pClass: err=%v, want ErrUnsupported", err)
	}
	if items != nil {
		t.Errorf("nil pClass: items=%v, want nil", items)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/lokc -run TestDocumentGetClipboard_NilSafe -v`
Expected: FAIL with `undefined: DocumentGetClipboard` (and possibly
`ClipboardItem`).

- [ ] **Step 3: Implement the wrapper and its C helper**

Create `internal/lokc/clipboard.go`:

```go
//go:build linux || darwin

package lokc

/*
#cgo CFLAGS: -I${SRCDIR}/../../third_party/lok -DLOK_USE_UNSTABLE_API
#include <stdlib.h>
#include <string.h>
#include "LibreOfficeKit/LibreOfficeKit.h"

// go_doc_get_clipboard calls pClass->getClipboard and passes the
// returned triple arrays back by pointer. Returns:
//   -1 when the vtable slot is NULL (unsupported)
//    0 when LOK reported failure
//    1 on success; *pCount / *pMimes / *pSizes / *pStreams populated
//      and owned by the caller (must be freed with go_doc_free_clipboard).
static int go_doc_get_clipboard(LibreOfficeKitDocument* d,
                                const char** inMimes,
                                size_t* pCount,
                                char*** pMimes,
                                size_t** pSizes,
                                char*** pStreams) {
    *pCount   = 0;
    *pMimes   = NULL;
    *pSizes   = NULL;
    *pStreams = NULL;
    if (d == NULL || d->pClass == NULL || d->pClass->getClipboard == NULL) return -1;
    int ok = d->pClass->getClipboard(d, inMimes, pCount, pMimes, pSizes, pStreams);
    return ok ? 1 : 0;
}

// go_doc_free_clipboard releases the triple arrays returned by
// go_doc_get_clipboard. Safe on NULL inputs and on zero count.
static void go_doc_free_clipboard(size_t count, char** mimes, size_t* sizes, char** streams) {
    if (mimes != NULL) {
        for (size_t i = 0; i < count; ++i) free(mimes[i]);
        free(mimes);
    }
    if (sizes != NULL) free(sizes);
    if (streams != NULL) {
        for (size_t i = 0; i < count; ++i) free(streams[i]);
        free(streams);
    }
}

// Accessors — cgo cannot index char** / size_t* from Go directly.
static char*  go_doc_clipboard_mime(char** mimes, size_t i)     { return mimes[i]; }
static size_t go_doc_clipboard_size(size_t* sizes, size_t i)    { return sizes[i]; }
static char*  go_doc_clipboard_stream(char** streams, size_t i) { return streams[i]; }
*/
import "C"

import (
	"errors"
	"unsafe"
)

// ClipboardItem is the in-package representation of one per-view
// clipboard entry returned by DocumentGetClipboard. Data is nil when
// LOK had no payload for the corresponding mime.
type ClipboardItem struct {
	MimeType string
	Data     []byte
}

// DocumentGetClipboard invokes pClass->getClipboard. A nil mimeTypes
// slice is forwarded as C NULL (all natively-offered types); an
// empty slice is also forwarded as NULL (LOK treats a zero-length
// NULL-terminated list identically). Returns ErrUnsupported when the
// vtable slot is NULL.
func DocumentGetClipboard(d DocumentHandle, mimeTypes []string) ([]ClipboardItem, error) {
	if !d.IsValid() {
		return nil, ErrUnsupported
	}

	// Build a NULL-terminated **char or nil.
	var inMimes **C.char
	if len(mimeTypes) > 0 {
		carr := C.malloc(C.size_t(len(mimeTypes)+1) * C.size_t(unsafe.Sizeof(uintptr(0))))
		defer C.free(carr)
		slice := (*[1 << 20]*C.char)(carr)[: len(mimeTypes)+1 : len(mimeTypes)+1]
		for i, m := range mimeTypes {
			slice[i] = C.CString(m)
			defer C.free(unsafe.Pointer(slice[i]))
		}
		slice[len(mimeTypes)] = nil
		inMimes = (**C.char)(carr)
	}

	var count C.size_t
	var outMimes, outStreams **C.char
	var outSizes *C.size_t
	ok := C.go_doc_get_clipboard(d.p, inMimes, &count, &outMimes, &outSizes, &outStreams)
	switch ok {
	case -1:
		return nil, ErrUnsupported
	case 0:
		// LOK reported failure; clean up any partial allocation.
		C.go_doc_free_clipboard(count, outMimes, outSizes, outStreams)
		return nil, errors.New("lokc: getClipboard returned failure")
	}
	defer C.go_doc_free_clipboard(count, outMimes, outSizes, outStreams)

	n := int(count)
	items := make([]ClipboardItem, n)
	for i := 0; i < n; i++ {
		cmime := C.go_doc_clipboard_mime(outMimes, C.size_t(i))
		sz := C.go_doc_clipboard_size(outSizes, C.size_t(i))
		cstream := C.go_doc_clipboard_stream(outStreams, C.size_t(i))
		items[i].MimeType = C.GoString(cmime)
		if cstream != nil {
			items[i].Data = C.GoBytes(unsafe.Pointer(cstream), C.int(sz))
		}
	}
	return items, nil
}
```

(The `errors` import is already in the block above. The
`[1 << 20]*C.char` cast is the established Go + cgo idiom for
indexing `**char`; length-capped via the re-slice on the next line
so it can't grow beyond the requested count.)

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/lokc -run TestDocumentGetClipboard_NilSafe -v`
Expected: PASS.

- [ ] **Step 5: Check cgo compiles the rest**

Run: `go build ./internal/lokc/...`
Expected: clean build. If the `[1 << 20]*C.char` cast produces a
warning on 32-bit platforms, it does not — we're building
`linux || darwin` only.

- [ ] **Step 6: Commit**

```bash
git add internal/lokc/clipboard.go internal/lokc/clipboard_test.go
git commit -m "feat(lokc): add DocumentGetClipboard with C-side free helper"
```

---

## Task 6: `internal/lokc` — `DocumentSetClipboard`

Symmetric to `Get`: build three parallel C-heap arrays from
`[]ClipboardItem`, call `pClass->setClipboard`, free everything on
return.

**Files:**
- Modify: `internal/lokc/clipboard.go`
- Modify: `internal/lokc/clipboard_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/lokc/clipboard_test.go`:

```go
func TestDocumentSetClipboard_NilSafe(t *testing.T) {
	if err := DocumentSetClipboard(DocumentHandle{}, nil); err != ErrUnsupported {
		t.Errorf("zero handle: err=%v, want ErrUnsupported", err)
	}
	items := []ClipboardItem{{MimeType: "text/plain", Data: []byte("hi")}}
	if err := DocumentSetClipboard(newFakeDoc(t), items); err != ErrUnsupported {
		t.Errorf("nil pClass: err=%v, want ErrUnsupported", err)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/lokc -run TestDocumentSetClipboard_NilSafe -v`
Expected: FAIL with `undefined: DocumentSetClipboard`.

- [ ] **Step 3: Implement the wrapper**

Append to `internal/lokc/clipboard.go`'s C preamble:

```c
static int go_doc_set_clipboard(LibreOfficeKitDocument* d,
                                size_t count,
                                const char** mimes,
                                const size_t* sizes,
                                const char** streams) {
    if (d == NULL || d->pClass == NULL || d->pClass->setClipboard == NULL) return -1;
    int ok = d->pClass->setClipboard(d, count, mimes, sizes, streams);
    return ok ? 1 : 0;
}
```

Append the Go wrapper:

```go
// DocumentSetClipboard invokes pClass->setClipboard. An empty items
// slice is forwarded as count=0 (LOK accepts this; the platform
// convention is that callers who want to clear the clipboard use
// ResetSelection or the .uno:Clear command). Returns ErrUnsupported
// when the vtable slot is NULL.
func DocumentSetClipboard(d DocumentHandle, items []ClipboardItem) error {
	if !d.IsValid() {
		return ErrUnsupported
	}

	n := len(items)
	var (
		mimesPtr   unsafe.Pointer
		sizesPtr   unsafe.Pointer
		streamsPtr unsafe.Pointer
		cMimes     **C.char
		cSizes     *C.size_t
		cStreams   **C.char
	)
	if n > 0 {
		mimesPtr = C.malloc(C.size_t(n) * C.size_t(unsafe.Sizeof(uintptr(0))))
		sizesPtr = C.malloc(C.size_t(n) * C.size_t(unsafe.Sizeof(C.size_t(0))))
		streamsPtr = C.malloc(C.size_t(n) * C.size_t(unsafe.Sizeof(uintptr(0))))
		defer C.free(mimesPtr)
		defer C.free(sizesPtr)
		defer C.free(streamsPtr)
		cMimes = (**C.char)(mimesPtr)
		cSizes = (*C.size_t)(sizesPtr)
		cStreams = (**C.char)(streamsPtr)

		mimesSlice := (*[1 << 20]*C.char)(mimesPtr)[:n:n]
		sizesSlice := (*[1 << 20]C.size_t)(sizesPtr)[:n:n]
		streamsSlice := (*[1 << 20]*C.char)(streamsPtr)[:n:n]

		for i, it := range items {
			mimesSlice[i] = C.CString(it.MimeType)
			defer C.free(unsafe.Pointer(mimesSlice[i]))
			sizesSlice[i] = C.size_t(len(it.Data))
			if len(it.Data) == 0 {
				streamsSlice[i] = nil
			} else {
				streamsSlice[i] = (*C.char)(C.CBytes(it.Data))
				defer C.free(unsafe.Pointer(streamsSlice[i]))
			}
		}
	}

	ok := C.go_doc_set_clipboard(d.p, C.size_t(n),
		(**C.char)(unsafe.Pointer(cMimes)),
		cSizes,
		(**C.char)(unsafe.Pointer(cStreams)))
	switch ok {
	case -1:
		return ErrUnsupported
	case 0:
		return errors.New("lokc: setClipboard returned failure")
	}
	return nil
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/lokc -run TestDocumentSetClipboard_NilSafe -v`
Expected: PASS.

- [ ] **Step 5: Build-check the whole package**

Run: `go build ./internal/lokc/... && go test ./internal/lokc/...`
Expected: both clean.

- [ ] **Step 6: Commit**

```bash
git add internal/lokc/clipboard.go internal/lokc/clipboard_test.go
git commit -m "feat(lokc): add DocumentSetClipboard symmetric to Get"
```

---

## Task 7: Extend `backend` interface + `realBackend` for selection

Add the selection / block methods to the private `backend` interface
and the matching `realBackend` forwarders. `fakeBackend` updates come
with the public tests in later tasks.

**Files:**
- Modify: `lok/backend.go`
- Modify: `lok/real_backend.go`

- [ ] **Step 1: Write a failing compile-level test**

This task has no user-visible behaviour yet — the assertion is a
static compile-time check that `realBackend` satisfies the extended
`backend` interface. Add the check in `lok/real_backend.go` by
augmenting (or adding) the existing `var _ backend = realBackend{}`
line. If that line already exists, it will fail to compile after
Step 2 until Step 3 fills in the forwarders.

Run: `grep -n 'var _ backend' lok/real_backend.go`
Expected: a line like `var _ backend = realBackend{}`. If absent, add
it at the bottom of `real_backend.go`:

```go
var _ backend = realBackend{}
```

- [ ] **Step 2: Add the new interface methods**

In `lok/backend.go`, append to the `backend` interface (inside the
interface block):

```go
	DocumentSetTextSelection(d documentHandle, typ, x, y int)
	DocumentResetSelection(d documentHandle)
	DocumentSetGraphicSelection(d documentHandle, typ, x, y int)
	DocumentSetBlockedCommandList(d documentHandle, viewID int, csv string)
	DocumentGetTextSelection(d documentHandle, mimeType string) (text, usedMime string)
	DocumentGetSelectionType(d documentHandle) int
	DocumentGetSelectionTypeAndText(d documentHandle, mimeType string) (kind int, text, usedMime string, err error)
```

- [ ] **Step 3: Run build to confirm realBackend now fails to compile**

Run: `go build ./lok/...`
Expected: FAIL with `realBackend does not implement backend (missing
method DocumentSetTextSelection)` or similar.

- [ ] **Step 4: Add the forwarders**

In `lok/real_backend.go`, append at the bottom (before the `init()`
block if it's still last):

```go
func (realBackend) DocumentSetTextSelection(d documentHandle, typ, x, y int) {
	lokc.DocumentSetTextSelection(mustDoc(d).d, typ, x, y)
}
func (realBackend) DocumentResetSelection(d documentHandle) {
	lokc.DocumentResetSelection(mustDoc(d).d)
}
func (realBackend) DocumentSetGraphicSelection(d documentHandle, typ, x, y int) {
	lokc.DocumentSetGraphicSelection(mustDoc(d).d, typ, x, y)
}
func (realBackend) DocumentSetBlockedCommandList(d documentHandle, viewID int, csv string) {
	lokc.DocumentSetBlockedCommandList(mustDoc(d).d, viewID, csv)
}
func (realBackend) DocumentGetTextSelection(d documentHandle, mimeType string) (string, string) {
	return lokc.DocumentGetTextSelection(mustDoc(d).d, mimeType)
}
func (realBackend) DocumentGetSelectionType(d documentHandle) int {
	return lokc.DocumentGetSelectionType(mustDoc(d).d)
}
func (realBackend) DocumentGetSelectionTypeAndText(d documentHandle, mimeType string) (int, string, string, error) {
	kind, text, mime, err := lokc.DocumentGetSelectionTypeAndText(mustDoc(d).d, mimeType)
	if err == lokc.ErrUnsupported {
		return -1, "", "", ErrUnsupported
	}
	return kind, text, mime, err
}
```

- [ ] **Step 5: Build to verify clean**

Run: `go build ./lok/...`
Expected: clean. The `var _ backend = realBackend{}` assertion now
holds with the new methods.

- [ ] **Step 6: Commit**

```bash
git add lok/backend.go lok/real_backend.go
git commit -m "feat(lok): extend backend seam with selection methods"
```

---

## Task 8: Extend `backend` interface + `realBackend` for clipboard

Same pattern as Task 7 but for `GetClipboard` / `SetClipboard`. Split
into a separate commit because the interface uses a package-public
type (`ClipboardItem`) that has to be visible from `lok`.

**Files:**
- Modify: `lok/backend.go`
- Modify: `lok/real_backend.go`

- [ ] **Step 1: Add the clipboard methods to the interface**

In `lok/backend.go`, append inside the `backend` interface:

```go
	DocumentGetClipboard(d documentHandle, mimeTypes []string) (items []clipboardItemInternal, err error)
	DocumentSetClipboard(d documentHandle, items []clipboardItemInternal) error
```

`clipboardItemInternal` is a package-private mirror of `lokc.ClipboardItem`;
it decouples the `backend` seam from the public `lok.ClipboardItem`
type. Declare it in `lok/backend.go` just below the interface:

```go
// clipboardItemInternal is the backend-interface mirror of
// lok.ClipboardItem / lokc.ClipboardItem. The three types carry the
// same fields; the indirection keeps the public type defined in
// lok/clipboard.go (Task 12) without the interface needing to import
// internal/lokc.
type clipboardItemInternal struct {
	MimeType string
	Data     []byte
}
```

- [ ] **Step 2: Run build, expect compile failure**

Run: `go build ./lok/...`
Expected: FAIL — `realBackend` missing `DocumentGetClipboard` /
`DocumentSetClipboard`.

- [ ] **Step 3: Add the forwarders**

In `lok/real_backend.go`, append:

```go
func (realBackend) DocumentGetClipboard(d documentHandle, mimeTypes []string) ([]clipboardItemInternal, error) {
	items, err := lokc.DocumentGetClipboard(mustDoc(d).d, mimeTypes)
	if err == lokc.ErrUnsupported {
		return nil, ErrUnsupported
	}
	if err != nil {
		return nil, err
	}
	out := make([]clipboardItemInternal, len(items))
	for i, it := range items {
		out[i] = clipboardItemInternal{MimeType: it.MimeType, Data: it.Data}
	}
	return out, nil
}
func (realBackend) DocumentSetClipboard(d documentHandle, items []clipboardItemInternal) error {
	lokItems := make([]lokc.ClipboardItem, len(items))
	for i, it := range items {
		lokItems[i] = lokc.ClipboardItem{MimeType: it.MimeType, Data: it.Data}
	}
	err := lokc.DocumentSetClipboard(mustDoc(d).d, lokItems)
	if err == lokc.ErrUnsupported {
		return ErrUnsupported
	}
	return err
}
```

- [ ] **Step 4: Build clean**

Run: `go build ./lok/...`
Expected: clean.

- [ ] **Step 5: Commit**

```bash
git add lok/backend.go lok/real_backend.go
git commit -m "feat(lok): extend backend seam with clipboard methods"
```

---

## Task 9: Typed enums `SetTextSelectionType`, `SetGraphicSelectionType`, `SelectionKind`

**Files:**
- Create: `lok/selection.go`
- Create: `lok/selection_test.go`

- [ ] **Step 1: Write the failing test**

Create `lok/selection_test.go`:

```go
//go:build linux || darwin

package lok

import "testing"

func TestSetTextSelectionType_String(t *testing.T) {
	cases := []struct {
		typ  SetTextSelectionType
		want string
	}{
		{SetTextSelectionStart, "SetTextSelectionStart"},
		{SetTextSelectionEnd, "SetTextSelectionEnd"},
		{SetTextSelectionReset, "SetTextSelectionReset"},
		{SetTextSelectionType(99), "SetTextSelectionType(99)"},
	}
	for _, tc := range cases {
		if got := tc.typ.String(); got != tc.want {
			t.Errorf("%d: got %q, want %q", tc.typ, got, tc.want)
		}
	}
}

func TestSetGraphicSelectionType_String(t *testing.T) {
	cases := []struct {
		typ  SetGraphicSelectionType
		want string
	}{
		{SetGraphicSelectionStart, "SetGraphicSelectionStart"},
		{SetGraphicSelectionEnd, "SetGraphicSelectionEnd"},
		{SetGraphicSelectionType(99), "SetGraphicSelectionType(99)"},
	}
	for _, tc := range cases {
		if got := tc.typ.String(); got != tc.want {
			t.Errorf("%d: got %q, want %q", tc.typ, got, tc.want)
		}
	}
}

func TestSelectionKind_String(t *testing.T) {
	cases := []struct {
		k    SelectionKind
		want string
	}{
		{SelectionKindNone, "SelectionKindNone"},
		{SelectionKindText, "SelectionKindText"},
		{SelectionKindComplex, "SelectionKindComplex"},
		{SelectionKind(99), "SelectionKind(99)"},
	}
	for _, tc := range cases {
		if got := tc.k.String(); got != tc.want {
			t.Errorf("%d: got %q, want %q", tc.k, got, tc.want)
		}
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./lok -run 'Test(SetTextSelectionType|SetGraphicSelectionType|SelectionKind)_String' -v`
Expected: FAIL — undefined types.

- [ ] **Step 3: Create `lok/selection.go` with the enums**

```go
//go:build linux || darwin

package lok

import "fmt"

// SetTextSelectionType mirrors LOK_SETTEXTSELECTION_*.
type SetTextSelectionType int

const (
	SetTextSelectionStart SetTextSelectionType = 0 // LOK_SETTEXTSELECTION_START
	SetTextSelectionEnd   SetTextSelectionType = 1 // LOK_SETTEXTSELECTION_END
	SetTextSelectionReset SetTextSelectionType = 2 // LOK_SETTEXTSELECTION_RESET
)

func (t SetTextSelectionType) String() string {
	switch t {
	case SetTextSelectionStart:
		return "SetTextSelectionStart"
	case SetTextSelectionEnd:
		return "SetTextSelectionEnd"
	case SetTextSelectionReset:
		return "SetTextSelectionReset"
	default:
		return fmt.Sprintf("SetTextSelectionType(%d)", int(t))
	}
}

// SetGraphicSelectionType mirrors LOK_SETGRAPHICSELECTION_*.
type SetGraphicSelectionType int

const (
	SetGraphicSelectionStart SetGraphicSelectionType = 0 // LOK_SETGRAPHICSELECTION_START
	SetGraphicSelectionEnd   SetGraphicSelectionType = 1 // LOK_SETGRAPHICSELECTION_END
)

func (t SetGraphicSelectionType) String() string {
	switch t {
	case SetGraphicSelectionStart:
		return "SetGraphicSelectionStart"
	case SetGraphicSelectionEnd:
		return "SetGraphicSelectionEnd"
	default:
		return fmt.Sprintf("SetGraphicSelectionType(%d)", int(t))
	}
}

// SelectionKind mirrors LOK_SELTYPE_*. LARGE_TEXT is not surfaced as
// a distinct kind — the LOK header documents it as "unused (same as
// LOK_SELTYPE_COMPLEX)" and code that receives it folds to
// SelectionKindComplex.
type SelectionKind int

const (
	SelectionKindNone    SelectionKind = 0 // LOK_SELTYPE_NONE
	SelectionKindText    SelectionKind = 1 // LOK_SELTYPE_TEXT
	SelectionKindComplex SelectionKind = 3 // LOK_SELTYPE_COMPLEX (LARGE_TEXT = 2 folds here)
)

// selectionKindFromLOK normalises a raw LOK int into SelectionKind.
// LOK_SELTYPE_LARGE_TEXT (2) is folded into SelectionKindComplex.
// Any other unknown value is returned verbatim so callers can log
// the surprise.
func selectionKindFromLOK(v int) SelectionKind {
	switch v {
	case 0:
		return SelectionKindNone
	case 1:
		return SelectionKindText
	case 2, 3:
		return SelectionKindComplex
	default:
		return SelectionKind(v)
	}
}

func (k SelectionKind) String() string {
	switch k {
	case SelectionKindNone:
		return "SelectionKindNone"
	case SelectionKindText:
		return "SelectionKindText"
	case SelectionKindComplex:
		return "SelectionKindComplex"
	default:
		return fmt.Sprintf("SelectionKind(%d)", int(k))
	}
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./lok -run 'Test(SetTextSelectionType|SetGraphicSelectionType|SelectionKind)_String' -v`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add lok/selection.go lok/selection_test.go
git commit -m "feat(lok): add SetTextSelectionType/SetGraphicSelectionType/SelectionKind enums"
```

---

## Task 10: `*Document` selection *getters* — `GetTextSelection`, `GetSelectionKind`, `GetSelectionTypeAndText`

**Files:**
- Modify: `lok/selection.go`
- Modify: `lok/selection_test.go`
- Modify: `lok/office_test.go` (extend `fakeBackend` with stubs + fields)

- [ ] **Step 1: Extend `fakeBackend`**

In `lok/office_test.go`, append to the `fakeBackend` struct fields
(add a new section at the bottom, before the closing `}`):

```go
	// Selection state (Phase 8).
	lastGetSelectionMime string
	selectionText        string
	selectionUsedMime    string
	selectionKind        int
	selectionTypeTextErr error

	lastSetTextSelectionTyp int
	lastSetTextSelectionX   int
	lastSetTextSelectionY   int
	resetSelectionCalls     int
	lastSetGraphicTyp       int
	lastSetGraphicX         int
	lastSetGraphicY         int
	lastBlockedViewID       int
	lastBlockedCSV          string
```

Then add the fake methods (place with the other fake methods at the
bottom of the file):

```go
func (f *fakeBackend) DocumentGetTextSelection(_ documentHandle, mime string) (string, string) {
	f.lastGetSelectionMime = mime
	return f.selectionText, f.selectionUsedMime
}

func (f *fakeBackend) DocumentGetSelectionType(documentHandle) int {
	return f.selectionKind
}

func (f *fakeBackend) DocumentGetSelectionTypeAndText(_ documentHandle, mime string) (int, string, string, error) {
	f.lastGetSelectionMime = mime
	if f.selectionTypeTextErr != nil {
		return -1, "", "", f.selectionTypeTextErr
	}
	return f.selectionKind, f.selectionText, f.selectionUsedMime, nil
}

func (f *fakeBackend) DocumentSetTextSelection(_ documentHandle, typ, x, y int) {
	f.lastSetTextSelectionTyp = typ
	f.lastSetTextSelectionX = x
	f.lastSetTextSelectionY = y
}

func (f *fakeBackend) DocumentResetSelection(documentHandle) {
	f.resetSelectionCalls++
}

func (f *fakeBackend) DocumentSetGraphicSelection(_ documentHandle, typ, x, y int) {
	f.lastSetGraphicTyp = typ
	f.lastSetGraphicX = x
	f.lastSetGraphicY = y
}

func (f *fakeBackend) DocumentSetBlockedCommandList(_ documentHandle, viewID int, csv string) {
	f.lastBlockedViewID = viewID
	f.lastBlockedCSV = csv
}
```

Run: `go build ./lok/...`
Expected: clean (fakeBackend now satisfies the extended interface).

- [ ] **Step 2: Write the failing tests for the getters**

Append to `lok/selection_test.go`:

```go
import "errors"

func TestGetTextSelection_ForwardsArgsAndStrings(t *testing.T) {
	fb := &fakeBackend{selectionText: "hello", selectionUsedMime: "text/plain;charset=utf-8"}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()

	text, usedMime, err := doc.GetTextSelection("text/plain")
	if err != nil {
		t.Fatalf("GetTextSelection: %v", err)
	}
	if text != "hello" || usedMime != "text/plain;charset=utf-8" {
		t.Errorf("got (%q, %q), want (hello, text/plain;charset=utf-8)", text, usedMime)
	}
	if fb.lastGetSelectionMime != "text/plain" {
		t.Errorf("mime forwarded = %q, want text/plain", fb.lastGetSelectionMime)
	}
}

func TestGetTextSelection_ClosedDoc(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	doc.Close()

	if _, _, err := doc.GetTextSelection("text/plain"); !errors.Is(err, ErrClosed) {
		t.Errorf("closed: want ErrClosed, got %v", err)
	}
}

func TestGetTextSelection_InvalidMime(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()

	cases := []struct {
		name string
		mime string
	}{
		{"empty", ""},
		{"nul", "text/plain\x00"},
		{"too-long", string(make([]byte, 257))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := doc.GetTextSelection(tc.mime); !errors.Is(err, ErrInvalidOption) {
				t.Errorf("mime=%q: want ErrInvalidOption, got %v", tc.mime, err)
			}
		})
	}
}

func TestGetSelectionKind_ReturnsKinds(t *testing.T) {
	cases := []struct {
		raw  int
		want SelectionKind
	}{
		{0, SelectionKindNone},
		{1, SelectionKindText},
		{2, SelectionKindComplex}, // LARGE_TEXT folds to Complex.
		{3, SelectionKindComplex},
	}
	for _, tc := range cases {
		fb := &fakeBackend{selectionKind: tc.raw}
		withFakeBackend(t, fb)
		o, _ := New("/install")
		doc, _ := o.Load("/tmp/x.odt")
		got, err := doc.GetSelectionKind()
		if err != nil {
			t.Fatalf("raw=%d: %v", tc.raw, err)
		}
		if got != tc.want {
			t.Errorf("raw=%d: got %v, want %v", tc.raw, got, tc.want)
		}
		doc.Close()
		o.Close()
	}
}

func TestGetSelectionKind_ClosedDoc(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	doc.Close()

	if _, err := doc.GetSelectionKind(); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestGetSelectionTypeAndText_HappyPath(t *testing.T) {
	fb := &fakeBackend{
		selectionKind:     1,
		selectionText:     "hi",
		selectionUsedMime: "text/plain;charset=utf-8",
	}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()

	kind, text, mime, err := doc.GetSelectionTypeAndText("text/plain")
	if err != nil {
		t.Fatalf("GetSelectionTypeAndText: %v", err)
	}
	if kind != SelectionKindText || text != "hi" || mime != "text/plain;charset=utf-8" {
		t.Errorf("got (%v, %q, %q)", kind, text, mime)
	}
}

func TestGetSelectionTypeAndText_UnsupportedBubbles(t *testing.T) {
	fb := &fakeBackend{selectionTypeTextErr: ErrUnsupported}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()

	_, _, _, err := doc.GetSelectionTypeAndText("text/plain")
	if !errors.Is(err, ErrUnsupported) {
		t.Errorf("want ErrUnsupported, got %v", err)
	}
}
```

- [ ] **Step 3: Run to verify failure**

Run: `go test ./lok -run 'TestGet(TextSelection|SelectionKind|SelectionTypeAndText)' -v`
Expected: FAIL — methods undefined.

- [ ] **Step 4: Implement the getters**

Append to `lok/selection.go`:

```go
// validateMime rejects empty / NUL-containing / > 256-byte MIME
// strings. LOK does its own structural validation; this catches the
// cases where we would crash or corrupt the C side before reaching
// it (embedded NUL truncates at C.CString).
func validateMime(s string) error {
	if s == "" || len(s) > 256 {
		return &LOKError{Op: "mime", Detail: "mime type must be non-empty and <= 256 bytes", err: ErrInvalidOption}
	}
	for i := 0; i < len(s); i++ {
		if s[i] == 0 {
			return &LOKError{Op: "mime", Detail: "mime type contains NUL byte", err: ErrInvalidOption}
		}
	}
	return nil
}

// GetTextSelection copies the current text selection as mimeType.
// LOK may substitute a different, compatible mime, which is returned
// in usedMime.
func (d *Document) GetTextSelection(mimeType string) (text, usedMime string, err error) {
	if err := validateMime(mimeType); err != nil {
		return "", "", err
	}
	unlock, gerr := d.guard()
	if gerr != nil {
		return "", "", gerr
	}
	defer unlock()
	t, m := d.office.be.DocumentGetTextSelection(d.h, mimeType)
	return t, m, nil
}

// GetSelectionKind reports what kind of selection is currently
// active without copying any text. Works on all supported LO
// versions.
func (d *Document) GetSelectionKind() (SelectionKind, error) {
	unlock, err := d.guard()
	if err != nil {
		return SelectionKindNone, err
	}
	defer unlock()
	return selectionKindFromLOK(d.office.be.DocumentGetSelectionType(d.h)), nil
}

// GetSelectionTypeAndText returns the selection kind and the
// selected text in a single LOK call. Requires LibreOffice >= 7.4;
// returns ErrUnsupported on older builds.
func (d *Document) GetSelectionTypeAndText(mimeType string) (kind SelectionKind, text, usedMime string, err error) {
	if verr := validateMime(mimeType); verr != nil {
		return SelectionKindNone, "", "", verr
	}
	unlock, gerr := d.guard()
	if gerr != nil {
		return SelectionKindNone, "", "", gerr
	}
	defer unlock()
	k, t, m, ierr := d.office.be.DocumentGetSelectionTypeAndText(d.h, mimeType)
	if ierr != nil {
		return SelectionKindNone, "", "", ierr
	}
	return selectionKindFromLOK(k), t, m, nil
}
```

- [ ] **Step 5: Run tests to verify pass**

Run: `go test ./lok -run 'TestGet(TextSelection|SelectionKind|SelectionTypeAndText)' -v`
Expected: PASS (6+ tests).

- [ ] **Step 6: Commit**

```bash
git add lok/selection.go lok/selection_test.go lok/office_test.go
git commit -m "feat(lok): add Document.GetTextSelection/GetSelectionKind/GetSelectionTypeAndText"
```

---

## Task 11: `*Document` selection *setters* — `SetTextSelection`, `ResetSelection`, `SetGraphicSelection`, `SetBlockedCommandList`

**Files:**
- Modify: `lok/selection.go`
- Modify: `lok/selection_test.go`

- [ ] **Step 1: Write failing tests**

Append to `lok/selection_test.go`:

```go
func TestSetTextSelection_ForwardsArgs(t *testing.T) {
	fb := &fakeBackend{}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()

	if err := doc.SetTextSelection(SetTextSelectionEnd, 1000, 2000); err != nil {
		t.Fatal(err)
	}
	if fb.lastSetTextSelectionTyp != 1 || fb.lastSetTextSelectionX != 1000 || fb.lastSetTextSelectionY != 2000 {
		t.Errorf("recorded (%d, %d, %d)", fb.lastSetTextSelectionTyp, fb.lastSetTextSelectionX, fb.lastSetTextSelectionY)
	}
}

func TestSetTextSelection_InvalidType(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()

	if err := doc.SetTextSelection(SetTextSelectionType(99), 0, 0); !errors.Is(err, ErrInvalidOption) {
		t.Errorf("want ErrInvalidOption, got %v", err)
	}
}

func TestSetTextSelection_Closed(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	doc.Close()

	if err := doc.SetTextSelection(SetTextSelectionStart, 0, 0); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestResetSelection_Forwards(t *testing.T) {
	fb := &fakeBackend{}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()

	for i := 0; i < 3; i++ {
		if err := doc.ResetSelection(); err != nil {
			t.Fatal(err)
		}
	}
	if fb.resetSelectionCalls != 3 {
		t.Errorf("resetSelectionCalls=%d, want 3", fb.resetSelectionCalls)
	}
}

func TestResetSelection_Closed(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	doc.Close()
	if err := doc.ResetSelection(); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestSetGraphicSelection_ForwardsArgs(t *testing.T) {
	fb := &fakeBackend{}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()

	if err := doc.SetGraphicSelection(SetGraphicSelectionEnd, 10, 20); err != nil {
		t.Fatal(err)
	}
	if fb.lastSetGraphicTyp != 1 || fb.lastSetGraphicX != 10 || fb.lastSetGraphicY != 20 {
		t.Errorf("recorded (%d, %d, %d)", fb.lastSetGraphicTyp, fb.lastSetGraphicX, fb.lastSetGraphicY)
	}
}

func TestSetGraphicSelection_InvalidType(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()

	if err := doc.SetGraphicSelection(SetGraphicSelectionType(99), 0, 0); !errors.Is(err, ErrInvalidOption) {
		t.Errorf("want ErrInvalidOption, got %v", err)
	}
}

func TestSetGraphicSelection_Closed(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	doc.Close()
	if err := doc.SetGraphicSelection(SetGraphicSelectionStart, 0, 0); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestSetBlockedCommandList_ForwardsArgs(t *testing.T) {
	fb := &fakeBackend{}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()

	if err := doc.SetBlockedCommandList(2, ".uno:Save,.uno:SaveAs"); err != nil {
		t.Fatal(err)
	}
	if fb.lastBlockedViewID != 2 || fb.lastBlockedCSV != ".uno:Save,.uno:SaveAs" {
		t.Errorf("recorded (%d, %q)", fb.lastBlockedViewID, fb.lastBlockedCSV)
	}
}

func TestSetBlockedCommandList_Closed(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	doc.Close()
	if err := doc.SetBlockedCommandList(0, ""); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./lok -run 'Test(SetTextSelection|ResetSelection|SetGraphicSelection|SetBlockedCommandList)' -v`
Expected: FAIL — methods undefined.

- [ ] **Step 3: Implement the setters**

Append to `lok/selection.go`:

```go
// SetTextSelection drags the selection handle of kind typ to the
// document position (x, y) in twips.
func (d *Document) SetTextSelection(typ SetTextSelectionType, x, y int64) error {
	switch typ {
	case SetTextSelectionStart, SetTextSelectionEnd, SetTextSelectionReset:
		// valid
	default:
		return &LOKError{Op: "SetTextSelection", Detail: fmt.Sprintf("type out of range: %d", int(typ)), err: ErrInvalidOption}
	}
	if err := requireInt32XY("SetTextSelection", x, y); err != nil {
		return err
	}
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	d.office.be.DocumentSetTextSelection(d.h, int(typ), int(x), int(y))
	return nil
}

// ResetSelection clears the current selection.
func (d *Document) ResetSelection() error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	d.office.be.DocumentResetSelection(d.h)
	return nil
}

// SetGraphicSelection drags a graphic-selection handle at (x, y)
// twips. typ = Start begins the drag; typ = End completes it.
func (d *Document) SetGraphicSelection(typ SetGraphicSelectionType, x, y int64) error {
	switch typ {
	case SetGraphicSelectionStart, SetGraphicSelectionEnd:
		// valid
	default:
		return &LOKError{Op: "SetGraphicSelection", Detail: fmt.Sprintf("type out of range: %d", int(typ)), err: ErrInvalidOption}
	}
	if err := requireInt32XY("SetGraphicSelection", x, y); err != nil {
		return err
	}
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	d.office.be.DocumentSetGraphicSelection(d.h, int(typ), int(x), int(y))
	return nil
}

// SetBlockedCommandList blocks the comma-separated set of UNO
// commands (csv) for the given view.
func (d *Document) SetBlockedCommandList(viewID int, csv string) error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	d.office.be.DocumentSetBlockedCommandList(d.h, viewID, csv)
	return nil
}
```

Add `"fmt"` to the imports if not already present.

- [ ] **Step 4: Run to verify pass**

Run: `go test ./lok -run 'Test(SetTextSelection|ResetSelection|SetGraphicSelection|SetBlockedCommandList)' -v`
Expected: PASS (9 tests).

- [ ] **Step 5: Commit**

```bash
git add lok/selection.go lok/selection_test.go
git commit -m "feat(lok): add Document.SetTextSelection/ResetSelection/SetGraphicSelection/SetBlockedCommandList"
```

---

## Task 12: `ClipboardItem` public type + fake-backend state

**Files:**
- Create: `lok/clipboard.go`
- Create: `lok/clipboard_test.go`
- Modify: `lok/office_test.go`

- [ ] **Step 1: Write a failing test for the type shape**

Create `lok/clipboard_test.go`:

```go
//go:build linux || darwin

package lok

import (
	"bytes"
	"testing"
)

func TestClipboardItem_ShapeCompiles(t *testing.T) {
	// Compile-time assertion: ClipboardItem has MimeType and Data
	// fields with the documented types.
	it := ClipboardItem{MimeType: "text/plain", Data: []byte("hi")}
	if it.MimeType != "text/plain" || !bytes.Equal(it.Data, []byte("hi")) {
		t.Errorf("ClipboardItem round-trip failed: %+v", it)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./lok -run TestClipboardItem_ShapeCompiles -v`
Expected: FAIL — `undefined: ClipboardItem`.

- [ ] **Step 3: Create `lok/clipboard.go`**

```go
//go:build linux || darwin

package lok

// ClipboardItem is a single per-view clipboard entry. Data is nil
// when LOK had no payload for the corresponding MimeType (GetClipboard
// preserves request order; unsupported MIME types come back as
// zero-Data entries).
type ClipboardItem struct {
	MimeType string
	Data     []byte
}
```

- [ ] **Step 4: Extend `fakeBackend` with clipboard state + methods**

In `lok/office_test.go`, add to the `fakeBackend` struct:

```go
	// Clipboard state (Phase 8).
	lastGetClipboardMimes []string
	getClipboardResult    []clipboardItemInternal
	getClipboardErr       error
	lastSetClipboardItems []clipboardItemInternal
	setClipboardErr       error
```

And the fake methods:

```go
func (f *fakeBackend) DocumentGetClipboard(_ documentHandle, mimes []string) ([]clipboardItemInternal, error) {
	// Record a copy so test mutations don't race with the fake.
	if mimes != nil {
		f.lastGetClipboardMimes = append([]string(nil), mimes...)
	} else {
		f.lastGetClipboardMimes = nil
	}
	if f.getClipboardErr != nil {
		return nil, f.getClipboardErr
	}
	out := make([]clipboardItemInternal, len(f.getClipboardResult))
	copy(out, f.getClipboardResult)
	return out, nil
}

func (f *fakeBackend) DocumentSetClipboard(_ documentHandle, items []clipboardItemInternal) error {
	f.lastSetClipboardItems = append([]clipboardItemInternal(nil), items...)
	return f.setClipboardErr
}
```

Run: `go build ./lok/...`
Expected: clean.

- [ ] **Step 5: Run the shape test to verify pass**

Run: `go test ./lok -run TestClipboardItem_ShapeCompiles -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add lok/clipboard.go lok/clipboard_test.go lok/office_test.go
git commit -m "feat(lok): add ClipboardItem type + fakeBackend clipboard plumbing"
```

---

## Task 13: `*Document.GetClipboard`

**Files:**
- Modify: `lok/clipboard.go`
- Modify: `lok/clipboard_test.go`

- [ ] **Step 1: Write failing tests**

Append to `lok/clipboard_test.go`:

```go
import "errors"

func TestGetClipboard_NilMimesForwardedAsNil(t *testing.T) {
	fb := &fakeBackend{getClipboardResult: []clipboardItemInternal{
		{MimeType: "text/plain;charset=utf-8", Data: []byte("hi")},
	}}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()

	items, err := doc.GetClipboard(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].MimeType != "text/plain;charset=utf-8" || string(items[0].Data) != "hi" {
		t.Errorf("items=%+v", items)
	}
	if fb.lastGetClipboardMimes != nil {
		t.Errorf("nil mimes forwarded as %v, want nil", fb.lastGetClipboardMimes)
	}
}

func TestGetClipboard_EmptyMimesForwardedAsNil(t *testing.T) {
	fb := &fakeBackend{}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()

	if _, err := doc.GetClipboard([]string{}); err != nil {
		t.Fatal(err)
	}
	if fb.lastGetClipboardMimes != nil {
		t.Errorf("empty mimes forwarded as %v, want nil", fb.lastGetClipboardMimes)
	}
}

func TestGetClipboard_PreservesRequestOrderWithNilData(t *testing.T) {
	fb := &fakeBackend{
		getClipboardResult: []clipboardItemInternal{
			{MimeType: "text/plain", Data: []byte("hi")},
			{MimeType: "application/x-nothing", Data: nil},
		},
	}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()

	items, err := doc.GetClipboard([]string{"text/plain", "application/x-nothing"})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("len=%d, want 2", len(items))
	}
	if items[1].Data != nil {
		t.Errorf("items[1].Data=%v, want nil", items[1].Data)
	}
	if fb.lastGetClipboardMimes[1] != "application/x-nothing" {
		t.Errorf("mime[1] forwarded=%q", fb.lastGetClipboardMimes[1])
	}
}

func TestGetClipboard_InvalidMime(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()

	if _, err := doc.GetClipboard([]string{""}); !errors.Is(err, ErrInvalidOption) {
		t.Errorf("want ErrInvalidOption, got %v", err)
	}
}

func TestGetClipboard_Closed(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	doc.Close()
	if _, err := doc.GetClipboard(nil); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestGetClipboard_BackendErrorSurfaces(t *testing.T) {
	fb := &fakeBackend{getClipboardErr: ErrUnsupported}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()
	if _, err := doc.GetClipboard(nil); !errors.Is(err, ErrUnsupported) {
		t.Errorf("want ErrUnsupported, got %v", err)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./lok -run 'TestGetClipboard' -v`
Expected: FAIL — method undefined.

- [ ] **Step 3: Implement `GetClipboard`**

Append to `lok/clipboard.go`:

```go
// GetClipboard reads the per-view clipboard. A nil (or empty)
// mimeTypes slice asks LOK for every MIME type it offers natively;
// a populated slice requests those specific types, returning one
// ClipboardItem per request in request order (unavailable entries
// come back with Data == nil).
func (d *Document) GetClipboard(mimeTypes []string) ([]ClipboardItem, error) {
	for _, m := range mimeTypes {
		if err := validateMime(m); err != nil {
			return nil, err
		}
	}
	unlock, err := d.guard()
	if err != nil {
		return nil, err
	}
	defer unlock()
	// Normalise empty slice to nil for the backend — both map to C
	// NULL in the real backend.
	var reqMimes []string
	if len(mimeTypes) > 0 {
		reqMimes = mimeTypes
	}
	inner, err := d.office.be.DocumentGetClipboard(d.h, reqMimes)
	if err != nil {
		return nil, err
	}
	out := make([]ClipboardItem, len(inner))
	for i, it := range inner {
		out[i] = ClipboardItem{MimeType: it.MimeType, Data: it.Data}
	}
	return out, nil
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./lok -run 'TestGetClipboard' -v`
Expected: PASS (6 tests).

- [ ] **Step 5: Commit**

```bash
git add lok/clipboard.go lok/clipboard_test.go
git commit -m "feat(lok): add Document.GetClipboard with ordered nil-Data semantics"
```

---

## Task 14: `*Document.SetClipboard`

**Files:**
- Modify: `lok/clipboard.go`
- Modify: `lok/clipboard_test.go`

- [ ] **Step 1: Write failing tests**

Append to `lok/clipboard_test.go`:

```go
func TestSetClipboard_Forwards(t *testing.T) {
	fb := &fakeBackend{}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()

	items := []ClipboardItem{{MimeType: "text/plain", Data: []byte("hi")}}
	if err := doc.SetClipboard(items); err != nil {
		t.Fatal(err)
	}
	if len(fb.lastSetClipboardItems) != 1 ||
		fb.lastSetClipboardItems[0].MimeType != "text/plain" ||
		string(fb.lastSetClipboardItems[0].Data) != "hi" {
		t.Errorf("recorded %+v", fb.lastSetClipboardItems)
	}
}

func TestSetClipboard_EmptySliceAllowed(t *testing.T) {
	fb := &fakeBackend{}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()

	if err := doc.SetClipboard(nil); err != nil {
		t.Fatalf("nil items: %v", err)
	}
	if err := doc.SetClipboard([]ClipboardItem{}); err != nil {
		t.Fatalf("empty items: %v", err)
	}
}

func TestSetClipboard_InvalidMime(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()

	items := []ClipboardItem{{MimeType: "", Data: []byte("x")}}
	if err := doc.SetClipboard(items); !errors.Is(err, ErrInvalidOption) {
		t.Errorf("want ErrInvalidOption, got %v", err)
	}
}

func TestSetClipboard_Closed(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	doc.Close()
	if err := doc.SetClipboard(nil); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestSetClipboard_BackendErrorSurfaces(t *testing.T) {
	fb := &fakeBackend{setClipboardErr: ErrUnsupported}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()
	items := []ClipboardItem{{MimeType: "text/plain", Data: []byte("hi")}}
	if err := doc.SetClipboard(items); !errors.Is(err, ErrUnsupported) {
		t.Errorf("want ErrUnsupported, got %v", err)
	}
}

func TestClipboard_RoundTripThroughFake(t *testing.T) {
	// Go: SetClipboard, have the fake stash what it saw, arrange for
	// GetClipboard to return that, and verify deep-equality.
	fb := &fakeBackend{}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()

	in := []ClipboardItem{
		{MimeType: "text/plain;charset=utf-8", Data: []byte("alpha")},
		{MimeType: "text/html", Data: []byte("<b>alpha</b>")},
	}
	if err := doc.SetClipboard(in); err != nil {
		t.Fatal(err)
	}
	// Round-trip: rehearse what the real backend would do.
	fb.getClipboardResult = fb.lastSetClipboardItems

	out, err := doc.GetClipboard(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != len(in) {
		t.Fatalf("len out=%d in=%d", len(out), len(in))
	}
	for i := range in {
		if out[i].MimeType != in[i].MimeType || string(out[i].Data) != string(in[i].Data) {
			t.Errorf("item %d: got %+v, want %+v", i, out[i], in[i])
		}
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./lok -run 'Test(SetClipboard|Clipboard_RoundTrip)' -v`
Expected: FAIL — method undefined.

- [ ] **Step 3: Implement `SetClipboard`**

Append to `lok/clipboard.go`:

```go
// SetClipboard writes items to the per-view clipboard, replacing
// the current contents. Each item's MimeType must pass
// validateMime. An empty or nil items slice is accepted (forwarded
// as zero count).
func (d *Document) SetClipboard(items []ClipboardItem) error {
	for _, it := range items {
		if err := validateMime(it.MimeType); err != nil {
			return err
		}
	}
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	inner := make([]clipboardItemInternal, len(items))
	for i, it := range items {
		inner[i] = clipboardItemInternal{MimeType: it.MimeType, Data: it.Data}
	}
	return d.office.be.DocumentSetClipboard(d.h, inner)
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./lok -run 'Test(SetClipboard|Clipboard_RoundTrip)' -v`
Expected: PASS (6 tests).

- [ ] **Step 5: Run full unit test suite to spot regressions**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add lok/clipboard.go lok/clipboard_test.go
git commit -m "feat(lok): add Document.SetClipboard + fake-backend round-trip"
```

---

## Task 15: Integration test — selection round-trip (SelectAll → Get → Reset)

The existing `lok/integration_test.go` runs every integration check
inside a single `TestIntegration_FullLifecycle` function because
`lok_init` cannot be re-invoked (memory:
`feedback_lok_singleton_per_process`). Extend that function rather
than adding a new top-level test.

**Files:**
- Modify: `lok/integration_test.go`

- [ ] **Step 1: Read the existing function**

Run: `grep -n 'Phase 8\|TestIntegration_FullLifecycle\|PostUnoCommand' lok/integration_test.go | head -20`
Expected: locate `TestIntegration_FullLifecycle` and the existing
Phase 7 block (the `PostUnoCommand(".uno:Deselect", ...)` / `SelectAll`
calls around lines ~335–360 in the file read earlier).

- [ ] **Step 2: Append the Phase 8 block inside `TestIntegration_FullLifecycle`**

Add the block at the end of the function, after the Phase 7 section
and before the final `t.Cleanup`/close. Per memory
`feedback_lok_input_needs_callback`, LO 24.8 on Fedora silently
drops posted input until `registerCallback` is hooked — **Phase 8
does not land a registerCallback binding** (that's Phase 9 scope).
So the integration block here is shaped as a best-effort probe: we
post `SelectAll`, poll for a selection with a short timeout, and
record `t.Logf` + `t.Skipf` only for the polling-timeout branch (the
feature itself is still fully exercised by the unit tests in Task
10). This is an explicit capability gate tied to a platform quirk
already in memory, not a silent skip.

```go
	// --- Phase 8: selection + clipboard smoke on a real document ---
	//
	// SelectAll is posted as a UNO command. On LO 24.8 on Fedora, LOK
	// silently drops posted input until a document-level callback is
	// registered — binding registerCallback is Phase 9 scope. If the
	// selection never appears within the poll budget, Skipf this
	// block (t.Logf the reason) rather than failing. The unit tests
	// in lok/selection_test.go exercise every argument-forwarding and
	// error path; this block exists to catch real-LOK regressions in
	// the cgo glue, not to gate the test suite on Phase 9 callback
	// plumbing.
	if err := doc.PostUnoCommand(".uno:SelectAll", "", false); err != nil {
		t.Errorf("Phase 8: SelectAll: %v", err)
	}
	selectionAppeared := false
	deadline := time.Now().Add(500 * time.Millisecond)
	var kind SelectionKind
	for time.Now().Before(deadline) {
		k, err := doc.GetSelectionKind()
		if err != nil {
			t.Errorf("Phase 8: GetSelectionKind: %v", err)
			break
		}
		if k != SelectionKindNone {
			kind = k
			selectionAppeared = true
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if !selectionAppeared {
		t.Logf("Phase 8: SelectAll yielded no observable selection (LO build drops input without registerCallback — Phase 9 scope); skipping selection assertions")
	} else {
		if kind != SelectionKindText && kind != SelectionKindComplex {
			t.Errorf("Phase 8: selection kind after SelectAll: %v", kind)
		}
		text, usedMime, err := doc.GetTextSelection("text/plain;charset=utf-8")
		if err != nil {
			t.Errorf("Phase 8: GetTextSelection: %v", err)
		}
		if usedMime == "" {
			t.Errorf("Phase 8: usedMime should be non-empty")
		}
		if !strings.Contains(text, "Hello") {
			t.Errorf("Phase 8: selection text %q does not contain 'Hello'", text)
		}
		// GetSelectionTypeAndText is LO 7.4+; the supported LO here is
		// 24.8 so it should always be present. Still capability-gate
		// for safety — see spec §4.3.
		kind2, text2, _, err := doc.GetSelectionTypeAndText("text/plain;charset=utf-8")
		if errors.Is(err, ErrUnsupported) {
			t.Logf("Phase 8: GetSelectionTypeAndText unsupported on this LO build")
		} else if err != nil {
			t.Errorf("Phase 8: GetSelectionTypeAndText: %v", err)
		} else {
			if kind2 != SelectionKindText {
				t.Errorf("Phase 8: kind2=%v, want %v", kind2, SelectionKindText)
			}
			if text2 != text {
				t.Errorf("Phase 8: text mismatch: %q vs %q", text2, text)
			}
		}

		if err := doc.ResetSelection(); err != nil {
			t.Errorf("Phase 8: ResetSelection: %v", err)
		}
		deadline = time.Now().Add(500 * time.Millisecond)
		for time.Now().Before(deadline) {
			k, err := doc.GetSelectionKind()
			if err != nil {
				t.Errorf("Phase 8: GetSelectionKind after reset: %v", err)
				break
			}
			if k == SelectionKindNone {
				break
			}
			time.Sleep(25 * time.Millisecond)
		}
	}

	// Smoke calls — assert only that the cgo path doesn't crash.
	// Phase 10 window geometry will let us drive these with real
	// coordinates.
	if err := doc.SetTextSelection(SetTextSelectionStart, 0, 0); err != nil {
		t.Errorf("Phase 8: SetTextSelection: %v", err)
	}
	if err := doc.SetGraphicSelection(SetGraphicSelectionEnd, 0, 0); err != nil {
		t.Errorf("Phase 8: SetGraphicSelection: %v", err)
	}
	if err := doc.SetBlockedCommandList(0, ""); err != nil {
		t.Errorf("Phase 8: SetBlockedCommandList: %v", err)
	}
```

Also add `"time"` to the imports if not already present. `"errors"`
and `"strings"` are already imported per the file read above.

- [ ] **Step 3: Run the integration test**

```bash
make test-integration INTEGRATION_ARGS='-run TestIntegration_FullLifecycle'
```

(The Makefile target sets `GODEBUG=asyncpreemptoff=1` — see the
file header comment.)

Expected: PASS. If the selection block logs the "no observable
selection" skip reason, that is an expected Phase-9 gated outcome —
not a failure. The smoke calls must not crash regardless.

- [ ] **Step 4: Commit**

```bash
git add lok/integration_test.go
git commit -m "test(lok): integration SelectAll → Get → Reset in FullLifecycle"
```

---

## Task 16: Integration — clipboard round-trip in `TestIntegration_FullLifecycle`

Extend the same function as Task 15 with a clipboard block. The
clipboard entry points do not depend on callbacks being hooked, so
this block asserts rather than skips.

**Files:**
- Modify: `lok/integration_test.go`

- [ ] **Step 1: Append the clipboard block after Task 15's block**

```go
	// --- Phase 8: clipboard round-trip ---
	//
	// setClipboard / getClipboard do not depend on registerCallback
	// — they are synchronous. Assert, don't skip.
	in := []ClipboardItem{
		{MimeType: "text/plain;charset=utf-8", Data: []byte("hi")},
	}
	if err := doc.SetClipboard(in); err != nil {
		t.Errorf("Phase 8: SetClipboard: %v", err)
	}

	items, err := doc.GetClipboard(nil)
	if err != nil {
		t.Errorf("Phase 8: GetClipboard(nil): %v", err)
	}
	var found bool
	for _, it := range items {
		if strings.HasPrefix(it.MimeType, "text/plain") && string(it.Data) == "hi" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Phase 8: text/plain hi not found in clipboard items: %+v", items)
	}

	req := []string{"text/plain;charset=utf-8", "application/x-totally-not-a-thing"}
	got, err := doc.GetClipboard(req)
	if err != nil {
		t.Errorf("Phase 8: GetClipboard(req): %v", err)
	}
	if len(got) != len(req) {
		t.Errorf("Phase 8: len(got)=%d, want %d", len(got), len(req))
	} else {
		if got[0].Data == nil {
			t.Errorf("Phase 8: got[0].Data is nil; want bytes")
		}
		if got[1].Data != nil {
			t.Errorf("Phase 8: got[1].Data=%v; want nil for unsupported mime", got[1].Data)
		}
		if got[1].MimeType != "application/x-totally-not-a-thing" {
			t.Logf("Phase 8: got[1].MimeType=%q (normalised by LOK)", got[1].MimeType)
		}
	}
```

- [ ] **Step 2: Run the integration test**

```bash
make test-integration INTEGRATION_ARGS='-run TestIntegration_FullLifecycle'
```

Expected: PASS. Both the Task-15 selection block and this clipboard
block now run in the same process against the shared LOK singleton.

- [ ] **Step 3: Commit**

```bash
git add lok/integration_test.go
git commit -m "test(lok): integration clipboard Set → Get round-trip"
```

---

## Task 17: `realBackend` forwarder coverage in `real_backend_test.go`

The pattern in `lok/real_backend_test.go` (e.g.
`TestRealBackend_InputForwarding`) is **unit-level**, not integration:
it uses `lokc.NewFakeDocumentHandle()` (NULL-pClass fake) and calls
`realBackend{}` forwarders, proving the Go-side forwarding statements
execute even though LOK never runs. This is what reaches the coverage
numbers. Follow the same pattern for Phase 8.

**Files:**
- Modify: `lok/real_backend_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `lok/real_backend_test.go`:

```go
func TestRealBackend_SelectionForwarding(t *testing.T) {
	rb := realBackend{}
	fakeDocHandle := lokc.NewFakeDocumentHandle()
	defer lokc.FreeFakeDocumentHandle(fakeDocHandle)
	rdoc := realDocumentHandle{d: fakeDocHandle}

	rb.DocumentSetTextSelection(rdoc, int(SetTextSelectionStart), 10, 20)
	rb.DocumentSetGraphicSelection(rdoc, int(SetGraphicSelectionEnd), 10, 20)
	rb.DocumentSetBlockedCommandList(rdoc, 0, ".uno:Save")
	rb.DocumentResetSelection(rdoc)

	// getters on a NULL-pClass handle return empty / -1 / Unsupported.
	if text, mime := rb.DocumentGetTextSelection(rdoc, "text/plain"); text != "" || mime != "" {
		t.Errorf("GetTextSelection on NULL pClass: got (%q, %q), want empty", text, mime)
	}
	if k := rb.DocumentGetSelectionType(rdoc); k != -1 {
		t.Errorf("GetSelectionType on NULL pClass: got %d, want -1", k)
	}
	kind, _, _, err := rb.DocumentGetSelectionTypeAndText(rdoc, "text/plain")
	if !errors.Is(err, ErrUnsupported) {
		t.Errorf("GetSelectionTypeAndText: err=%v, want ErrUnsupported", err)
	}
	if kind != -1 {
		t.Errorf("GetSelectionTypeAndText kind=%d on NULL pClass, want -1", kind)
	}
}

func TestRealBackend_ClipboardForwarding(t *testing.T) {
	rb := realBackend{}
	fakeDocHandle := lokc.NewFakeDocumentHandle()
	defer lokc.FreeFakeDocumentHandle(fakeDocHandle)
	rdoc := realDocumentHandle{d: fakeDocHandle}

	items, err := rb.DocumentGetClipboard(rdoc, nil)
	if !errors.Is(err, ErrUnsupported) {
		t.Errorf("GetClipboard on NULL pClass: err=%v, want ErrUnsupported", err)
	}
	if items != nil {
		t.Errorf("GetClipboard on NULL pClass: items=%v, want nil", items)
	}

	// Request-list path also exercises the CString allocation loop.
	_, err = rb.DocumentGetClipboard(rdoc, []string{"text/plain", "text/html"})
	if !errors.Is(err, ErrUnsupported) {
		t.Errorf("GetClipboard([list]): err=%v, want ErrUnsupported", err)
	}

	if err := rb.DocumentSetClipboard(rdoc, nil); !errors.Is(err, ErrUnsupported) {
		t.Errorf("SetClipboard(nil): err=%v, want ErrUnsupported", err)
	}
	// Non-empty items path exercises the CBytes + CString allocation loop.
	in := []clipboardItemInternal{{MimeType: "text/plain", Data: []byte("hi")}}
	if err := rb.DocumentSetClipboard(rdoc, in); !errors.Is(err, ErrUnsupported) {
		t.Errorf("SetClipboard([items]): err=%v, want ErrUnsupported", err)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./lok -run 'TestRealBackend_(Selection|Clipboard)Forwarding' -v`
Expected: initially compiles and likely FAILS at the brand check or
missing `NewFakeDocumentHandle` (if that helper isn't exported). If
the helper is missing, add a thin exported wrapper in
`internal/lokc/document_test_helper.go` matching the naming used by
Phase 7 — the file is untagged (not `_test.go`), so the rest of the
test code can call it freely.

Run: `grep -n 'NewFakeDocumentHandle\|FreeFakeDocumentHandle' internal/lokc/*.go`
Expected: both helpers already exist (per Phase 7's
`TestRealBackend_InputForwarding`). If not, they need adding —
declare the failure and add them before continuing.

- [ ] **Step 3: If helpers exist, run to verify pass**

Run: `go test ./lok -run 'TestRealBackend_(Selection|Clipboard)Forwarding' -v`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add lok/real_backend_test.go
git commit -m "test(lok): realBackend forwarder coverage for Phase 8"
```

---

## Task 18: Coverage gate + final verification

**Files:**
- No code changes unless coverage is short.

- [ ] **Step 1: Run unit tests with coverage**

Run:

```bash
go test -covermode=atomic -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | tail -n 1
```

Expected: the total line reports ≥ 90.0 %. Note: the repo rule lives
in CLAUDE.md §6; treat the number as load-bearing.

- [ ] **Step 2: If coverage is short, identify the gaps**

Run: `go tool cover -func=coverage.out | awk '$3 != "100.0%" {print}' | head -40`
Expected: any sub-90 % packages are visible. Tasks 1–14 should already
cover every branch in the selection/clipboard code paths; if one
slipped, add the missing test in the same file as the test it
neighbours and re-run. **Do not lower the threshold.**

- [ ] **Step 3: Run the full suite (unit + integration) once more**

```bash
go test ./...
LOK_PATH=/usr/lib64/libreoffice/program go test -tags=lok_integration -p 1 ./...
```

Expected: both clean.

- [ ] **Step 4: Run `go vet` and `gofmt -s` check**

```bash
go vet ./...
gofmt -s -l .
```

Expected: both silent.

- [ ] **Step 5: Commit any coverage patches / formatting fixes**

If earlier steps needed patches:

```bash
git add <files>
git commit -m "test(lok): top up coverage for Phase 8"
```

- [ ] **Step 6: Update the CHANGELOG / `docs/superpowers/plans/...` state**

Run: `git log --oneline main..HEAD` to review the phase 8 commit log
and confirm the series is ready for PR.

---

## Out-of-scope / deferred

- **Selection-handle state assertions.** `SetTextSelection` and
  `SetGraphicSelection` integration coverage is a smoke-call only;
  meaningful assertions need window geometry from Phase 10. Called
  out in spec §7.3 and in the real-backend smoke block above.
- **`.uno:SelectAll` callback acknowledgement.** Phase 9 (`feat/callbacks`)
  will replace the `time.Sleep` poll loop with a registered callback
  listening for `LOK_CALLBACK_TEXT_SELECTION` / similar. Phase 8 uses
  a timeout-bounded poll, matching Phase 7's precedent.
- **`pClass->paste`.** LOK's direct `paste` entry point is separate
  from the per-view clipboard and is not in the spec; revisit with
  the curated UNO helpers in Phase 10.

## Self-Review

- **Spec coverage.** Every method in spec §3 has a task: Tasks 9–11
  cover selection, 12–14 cover clipboard, 10 covers
  `GetSelectionKind` (the question-4b add-on). `ErrUnsupported` from
  spec §5 is Task 1. Deferrals from spec §9 are restated in the
  "Out-of-scope" section above.
- **Placeholder scan.** No TBD / TODO / "add appropriate error
  handling" / "similar to Task N" remain. Each test step shows the
  test body; each impl step shows the code.
- **Type consistency.** `SetTextSelectionType` / `SetGraphicSelectionType`
  / `SelectionKind` / `ClipboardItem` / `clipboardItemInternal` /
  `ErrUnsupported` appear with identical signatures in every task
  that references them. `fakeBackend` fields introduced in Task 10
  (`selectionTypeTextErr`, `selectionKind`, etc.) are the same names
  used in Tasks 10–14 tests; clipboard fields in Task 12
  (`getClipboardResult`, `setClipboardErr`, `lastSetClipboardItems`)
  are consistent with Tasks 13–14.
