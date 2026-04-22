# Phase 5 — Parts &amp; Sizing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give `lok.Document` the ability to enumerate parts
(sheets/pages/slides), switch between them, read their metadata,
query document size, and manipulate outline state. Public API:

```go
type TwipRect struct{ X, Y, W, H int64 } // twips (1/1440 inch)

func (*Document) Parts() (int, error)
func (*Document) Part() (int, error)
func (*Document) SetPart(n int) error
func (*Document) PartName(n int) (string, error)
func (*Document) PartHash(n int) (string, error)
func (*Document) PartInfo(n int) (json.RawMessage, error)
func (*Document) SetPartMode(mode int) error
func (*Document) DocumentSize() (widthTwips, heightTwips int64, err error)
func (*Document) PartPageRectangles() ([]TwipRect, error)
func (*Document) SetOutlineState(column bool, level, index int, hidden bool) error
```

### Deviations from spec §Phase 5 (called out early)

1. **`SetPart(n, allowDuplicate bool)` → `SetPart(n int)`.** The
   spec's `allowDuplicate` parameter does not exist in the vendored
   LOK 24.8 `setPart` vtable entry (`void setPart(doc, int nPart)`).
   Dropped.
2. **`SetOutlineState(column, level int, hidden bool)` →
   `SetOutlineState(column bool, level, index int, hidden bool)`.**
   Spec omitted the `nIndex` parameter that's present in the
   header (`setOutlineState(doc, bool bColumn, int nLevel, int
   nIndex, bool bHidden)`), and had `column` typed as `int` when
   it's a `bool`. Plan follows the header.
3. **`Parts()` / `Part()` / `PartName` / `PartHash` / `DocumentSize` /
   `PartPageRectangles`** now return `(X, error)` where spec had
   no error. Matches Phase 4's precedent — closed-doc calls surface
   `ErrClosed` instead of lying with zero values.
4. **`SetPartMode(mode int)`** added opportunistically. LOK exposes
   `setPartMode` adjacent to `setPart`; cheap to bring in now.

### Architecture

- `internal/lokc` gains thin wrappers: `DocumentGetParts`,
  `DocumentGetPart`, `DocumentSetPart`, `DocumentGetPartName`,
  `DocumentGetPartHash`, `DocumentGetPartInfo`,
  `DocumentSetPartMode`, `DocumentGetDocumentSize`,
  `DocumentGetPartPageRectangles`, `DocumentSetOutlineState`.
- String-returning wrappers use `copyAndFree` for LOK-owned `char*`.
- `getDocumentSize(long* w, long* h)` fills two `C.long` out-params;
  Go wrapper returns `(int64, int64)`.
- `getPartPageRectangles` returns a single char* of the form
  `"x, y, w, h; x, y, w, h; ..."`. `lok.PartPageRectangles` parses
  that string into `[]TwipRect`.
- `getPartInfo` returns a LOK-allocated JSON string; `lok.PartInfo`
  surfaces it as `json.RawMessage` (typed struct unmarshal is the
  caller's responsibility — schemas vary by document type).
- All public methods use the existing `Document.guard()` helper from
  Phase 4, so lock/closed semantics are consistent.

### Coverage gate

Unchanged at 90% across `./internal/lokc/...` + `./lok/...`.

### Branching

`chore/parts-and-size`, branched from `main` after PR #10 merged.

---

## Files

| Path | Role |
|------|------|
| `internal/lokc/part.go` (create) | Cgo wrappers for the 10 Part/size functions |
| `internal/lokc/part_test.go` (create) | Unit tests via `NewFakeDocumentHandle` |
| `lok/part.go` (create) | `TwipRect`, Part methods + `parsePartPageRectangles` + `parseDocumentSize` |
| `lok/part_test.go` (create) | Unit tests via `fakeBackend` |
| `lok/backend.go` (modify) | Extend `backend` interface |
| `lok/real_backend.go` (modify) | Wire through `internal/lokc` |
| `lok/real_backend_test.go` (modify) | Forwarding tests via `NewFakeDocumentHandle` |
| `lok/office_test.go` (modify) | Extend `fakeBackend` with part state |
| `lok/integration_test.go` (modify) | Part subtests inside `TestIntegration_FullLifecycle` |

---

## Task 0: Branch prep

- [ ] **Step 1: Sync main**
  ```bash
  git checkout main && git pull --ff-only && git status --short
  ```
  Expected: clean, main at post-PR-10 tip.

- [ ] **Step 2: Create branch**
  ```bash
  git checkout -b chore/parts-and-size && git branch --show-current
  ```
  Expected: `chore/parts-and-size`.

---

## Task 1: `internal/lokc` part wrappers (TDD)

**Files:**
- Create: `internal/lokc/part.go`
- Create: `internal/lokc/part_test.go`

### 1.1 Failing tests

- [ ] **Step 1: Create `internal/lokc/part_test.go`**

  ```go
  //go:build linux || darwin

  package lokc

  import "testing"

  func TestDocumentPart_NilHandleAreNoOps(t *testing.T) {
  	var d DocumentHandle
  	if got := DocumentGetParts(d); got != -1 {
  		t.Errorf("GetParts on nil: got %d, want -1", got)
  	}
  	if got := DocumentGetPart(d); got != -1 {
  		t.Errorf("GetPart on nil: got %d, want -1", got)
  	}
  	if got := DocumentGetPartName(d, 0); got != "" {
  		t.Errorf("GetPartName on nil: got %q, want empty", got)
  	}
  	if got := DocumentGetPartHash(d, 0); got != "" {
  		t.Errorf("GetPartHash on nil: got %q, want empty", got)
  	}
  	if got := DocumentGetPartInfo(d, 0); got != "" {
  		t.Errorf("GetPartInfo on nil: got %q, want empty", got)
  	}
  	if w, h := DocumentGetDocumentSize(d); w != 0 || h != 0 {
  		t.Errorf("GetDocumentSize on nil: got (%d, %d), want (0, 0)", w, h)
  	}
  	if got := DocumentGetPartPageRectangles(d); got != "" {
  		t.Errorf("GetPartPageRectangles on nil: got %q, want empty", got)
  	}
  	DocumentSetPart(d, 0)
  	DocumentSetPartMode(d, 0)
  	DocumentSetOutlineState(d, false, 0, 0, false)
  }

  func TestDocumentPart_FakeHandle_SafeNoOps(t *testing.T) {
  	d := NewFakeDocumentHandle()
  	t.Cleanup(func() { FreeFakeDocumentHandle(d) })

  	DocumentGetParts(d)
  	DocumentGetPart(d)
  	DocumentGetPartName(d, 0)
  	DocumentGetPartHash(d, 0)
  	DocumentGetPartInfo(d, 0)
  	DocumentGetDocumentSize(d)
  	DocumentGetPartPageRectangles(d)
  	DocumentSetPart(d, 0)
  	DocumentSetPartMode(d, 0)
  	DocumentSetOutlineState(d, false, 0, 0, false)
  }
  ```

- [ ] **Step 2: Run — red**

  Run: `go test ./internal/lokc/... -run DocumentPart`
  Expected: undefined symbols.

### 1.2 Implement

- [ ] **Step 3: Create `internal/lokc/part.go`**

  ```go
  //go:build linux || darwin

  package lokc

  /*
  #cgo CFLAGS: -I${SRCDIR}/../../third_party/lok -DLOK_USE_UNSTABLE_API
  #include <stdbool.h>
  #include <stdlib.h>
  #include "LibreOfficeKit/LibreOfficeKit.h"

  static int go_doc_get_parts(LibreOfficeKitDocument* d) {
      if (d == NULL || d->pClass == NULL || d->pClass->getParts == NULL) return -1;
      return d->pClass->getParts(d);
  }
  static int go_doc_get_part(LibreOfficeKitDocument* d) {
      if (d == NULL || d->pClass == NULL || d->pClass->getPart == NULL) return -1;
      return d->pClass->getPart(d);
  }
  static void go_doc_set_part(LibreOfficeKitDocument* d, int n) {
      if (d == NULL || d->pClass == NULL || d->pClass->setPart == NULL) return;
      d->pClass->setPart(d, n);
  }
  static void go_doc_set_part_mode(LibreOfficeKitDocument* d, int mode) {
      if (d == NULL || d->pClass == NULL || d->pClass->setPartMode == NULL) return;
      d->pClass->setPartMode(d, mode);
  }
  static char* go_doc_get_part_name(LibreOfficeKitDocument* d, int n) {
      if (d == NULL || d->pClass == NULL || d->pClass->getPartName == NULL) return NULL;
      return d->pClass->getPartName(d, n);
  }
  static char* go_doc_get_part_hash(LibreOfficeKitDocument* d, int n) {
      if (d == NULL || d->pClass == NULL || d->pClass->getPartHash == NULL) return NULL;
      return d->pClass->getPartHash(d, n);
  }
  static char* go_doc_get_part_info(LibreOfficeKitDocument* d, int n) {
      if (d == NULL || d->pClass == NULL || d->pClass->getPartInfo == NULL) return NULL;
      return d->pClass->getPartInfo(d, n);
  }
  static char* go_doc_get_part_page_rects(LibreOfficeKitDocument* d) {
      if (d == NULL || d->pClass == NULL || d->pClass->getPartPageRectangles == NULL) return NULL;
      return d->pClass->getPartPageRectangles(d);
  }
  static void go_doc_get_document_size(LibreOfficeKitDocument* d, long* w, long* h) {
      *w = 0; *h = 0;
      if (d == NULL || d->pClass == NULL || d->pClass->getDocumentSize == NULL) return;
      d->pClass->getDocumentSize(d, w, h);
  }
  static void go_doc_set_outline_state(LibreOfficeKitDocument* d, bool col, int level, int idx, bool hidden) {
      if (d == NULL || d->pClass == NULL || d->pClass->setOutlineState == NULL) return;
      d->pClass->setOutlineState(d, col, level, idx, hidden);
  }
  */
  import "C"

  // DocumentGetParts returns the number of parts (sheets/pages/slides),
  // or -1 on unavailable handle/vtable.
  func DocumentGetParts(d DocumentHandle) int {
  	if !d.IsValid() {
  		return -1
  	}
  	return int(C.go_doc_get_parts(d.p))
  }

  // DocumentGetPart returns the currently-active part index, or -1.
  func DocumentGetPart(d DocumentHandle) int {
  	if !d.IsValid() {
  		return -1
  	}
  	return int(C.go_doc_get_part(d.p))
  }

  // DocumentSetPart forwards to pClass->setPart.
  func DocumentSetPart(d DocumentHandle, n int) {
  	if !d.IsValid() {
  		return
  	}
  	C.go_doc_set_part(d.p, C.int(n))
  }

  // DocumentSetPartMode forwards to pClass->setPartMode. The mode
  // enum values live in LibreOfficeKitEnums.h (LOK_PARTMODE_*).
  func DocumentSetPartMode(d DocumentHandle, mode int) {
  	if !d.IsValid() {
  		return
  	}
  	C.go_doc_set_part_mode(d.p, C.int(mode))
  }

  // DocumentGetPartName returns the part's display name. Empty string
  // on error or missing vtable.
  func DocumentGetPartName(d DocumentHandle, n int) string {
  	if !d.IsValid() {
  		return ""
  	}
  	return copyAndFree(C.go_doc_get_part_name(d.p, C.int(n)))
  }

  // DocumentGetPartHash returns the part's stable hash string.
  func DocumentGetPartHash(d DocumentHandle, n int) string {
  	if !d.IsValid() {
  		return ""
  	}
  	return copyAndFree(C.go_doc_get_part_hash(d.p, C.int(n)))
  }

  // DocumentGetPartInfo returns the LOK-allocated JSON blob for a part.
  func DocumentGetPartInfo(d DocumentHandle, n int) string {
  	if !d.IsValid() {
  		return ""
  	}
  	return copyAndFree(C.go_doc_get_part_info(d.p, C.int(n)))
  }

  // DocumentGetPartPageRectangles returns LOK's semicolon-separated
  // "x, y, w, h; …" rectangle string. Caller parses.
  func DocumentGetPartPageRectangles(d DocumentHandle) string {
  	if !d.IsValid() {
  		return ""
  	}
  	return copyAndFree(C.go_doc_get_part_page_rects(d.p))
  }

  // DocumentGetDocumentSize returns (width, height) in twips. Both
  // zero if unavailable. Assumes LP64 (Linux amd64, macOS arm64,
  // macOS amd64) — `long` is 64-bit on all supported platforms, so
  // int64(C.long) is lossless. 32-bit platforms are unsupported
  // per the spec.
  func DocumentGetDocumentSize(d DocumentHandle) (int64, int64) {
  	if !d.IsValid() {
  		return 0, 0
  	}
  	var w, h C.long
  	C.go_doc_get_document_size(d.p, &w, &h)
  	return int64(w), int64(h)
  }

  // DocumentSetOutlineState forwards to pClass->setOutlineState.
  func DocumentSetOutlineState(d DocumentHandle, column bool, level, index int, hidden bool) {
  	if !d.IsValid() {
  		return
  	}
  	C.go_doc_set_outline_state(d.p, C.bool(column), C.int(level), C.int(index), C.bool(hidden))
  }
  ```

- [ ] **Step 4: Run tests — green**

  Run: `go test -race ./internal/lokc/...`
  Expected: PASS.

- [ ] **Step 5: Cover-gate**

  Run: `make cover-gate`
  Expected: ≥ 90%.

- [ ] **Step 6: Commit**

  ```bash
  git add internal/lokc/part.go internal/lokc/part_test.go
  git commit -m "feat(lokc): add part- and size-level cgo wrappers

Ten 1:1 vtable wrappers: GetParts, GetPart, SetPart, SetPartMode,
GetPartName, GetPartHash, GetPartInfo, GetPartPageRectangles,
GetDocumentSize, SetOutlineState. String returns flow through
copyAndFree. GetDocumentSize initialises the long* out-params to
0 before the cgo call so a guarded-out vtable entry yields a
well-defined (0, 0).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 2: `lok` backend extension + `fakeBackend`

**Files:**
- Modify: `lok/backend.go`
- Modify: `lok/real_backend.go`
- Modify: `lok/office_test.go`
- Modify: `lok/real_backend_test.go`

### Step 1: Extend `lok/backend.go`

Append to the `backend` interface:
```go
	DocumentGetParts(d documentHandle) int
	DocumentGetPart(d documentHandle) int
	DocumentSetPart(d documentHandle, n int)
	DocumentSetPartMode(d documentHandle, mode int)
	DocumentGetPartName(d documentHandle, n int) string
	DocumentGetPartHash(d documentHandle, n int) string
	DocumentGetPartInfo(d documentHandle, n int) string
	DocumentGetPartPageRectangles(d documentHandle) string
	DocumentGetDocumentSize(d documentHandle) (int64, int64)
	DocumentSetOutlineState(d documentHandle, column bool, level, index int, hidden bool)
```

### Step 2: Extend `lok/real_backend.go`

One-liner forwarders for each, using `mustDoc(d).d`.

### Step 3: Extend `fakeBackend` in `lok/office_test.go`

New fields (under the existing "View state" comment block or a new
block):
```go
	// partsCount convention: -1 = simulate LOK backend failure
	// (matches internal/lokc's return-on-NULL-pClass); 0+ = real
	// part count. Fresh `&fakeBackend{}` is a 0-part document.
	partsCount      int
	partActive      int
	partNames       map[int]string
	partHashes      map[int]string
	partInfos       map[int]string
	partRects       string
	docWidthTwips   int64
	docHeightTwips  int64
	lastPartMode    int

	lastOutlineCol    bool
	lastOutlineLevel  int
	lastOutlineIndex  int
	lastOutlineHidden bool
```

Methods — most are simple getters/setters. The map types allow
tests to inject per-part fixtures; nil-map reads return the zero
string which matches LOK's "unavailable" signal.

### Step 4: Extend `lok/real_backend_test.go`

Add `TestRealBackend_PartForwarding` using `NewFakeDocumentHandle`,
mirroring the Phase 4 view-forwarding test shape.

### Step 5: Run + commit

```bash
make all && make cover-gate
git add lok/backend.go lok/real_backend.go lok/office_test.go lok/real_backend_test.go
git commit -m "feat(lok): backend seam and fake for parts/size

backend interface grows 10 part methods. realBackend forwards
each to internal/lokc; fakeBackend carries explicit state
(partsCount, partActive, maps of per-part name/hash/info, twip
dimensions, outline-state capture fields) so tests exercise real
semantics.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: `Document.Parts` / `Part` / `SetPart` / `PartName` / `PartHash` (TDD)

**Files:**
- Create: `lok/part.go`
- Create: `lok/part_test.go`

### Step 1: Failing tests

Top of `lok/part_test.go`:
```go
//go:build linux || darwin

package lok

import (
	"errors"
	"testing"
)

func TestParts_ReturnsBackendCount(t *testing.T) {
	fb := &fakeBackend{partsCount: 3}
	_, doc := loadFakeDoc(t, fb)
	n, err := doc.Parts()
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("Parts=%d, want 3", n)
	}
}

func TestParts_BackendFailureErrors(t *testing.T) {
	// partsCount=-1 signals the backend failure path (matches lokc convention).
	fb := &fakeBackend{partsCount: -1}
	_, doc := loadFakeDoc(t, fb)
	_, err := doc.Parts()
	var lokErr *LOKError
	if !errors.As(err, &lokErr) {
		t.Errorf("want *LOKError, got %T %v", err, err)
	}
}

func TestPart_ReadsActive(t *testing.T) {
	_, doc := loadFakeDoc(t, &fakeBackend{partsCount: 4, partActive: 2})
	got, err := doc.Part()
	if err != nil {
		t.Fatal(err)
	}
	if got != 2 {
		t.Errorf("Part=%d, want 2", got)
	}
}

func TestSetPart_UpdatesActive(t *testing.T) {
	fb := &fakeBackend{partsCount: 4}
	_, doc := loadFakeDoc(t, fb)
	if err := doc.SetPart(2); err != nil {
		t.Fatal(err)
	}
	if fb.partActive != 2 {
		t.Errorf("partActive=%d, want 2", fb.partActive)
	}
}

func TestPartName_ReadsMap(t *testing.T) {
	fb := &fakeBackend{partNames: map[int]string{1: "Sheet2"}}
	_, doc := loadFakeDoc(t, fb)
	name, err := doc.PartName(1)
	if err != nil {
		t.Fatal(err)
	}
	if name != "Sheet2" {
		t.Errorf("PartName(1)=%q, want Sheet2", name)
	}
}

func TestPartHash_ReadsMap(t *testing.T) {
	fb := &fakeBackend{partHashes: map[int]string{0: "abc123"}}
	_, doc := loadFakeDoc(t, fb)
	hash, err := doc.PartHash(0)
	if err != nil {
		t.Fatal(err)
	}
	if hash != "abc123" {
		t.Errorf("PartHash(0)=%q", hash)
	}
}

func TestPartMethods_AfterCloseErrors(t *testing.T) {
	cases := []struct {
		name string
		call func(*Document) error
	}{
		{"Parts", func(d *Document) error { _, err := d.Parts(); return err }},
		{"Part", func(d *Document) error { _, err := d.Part(); return err }},
		{"SetPart", func(d *Document) error { return d.SetPart(0) }},
		{"SetPartMode", func(d *Document) error { return d.SetPartMode(0) }},
		{"PartName", func(d *Document) error { _, err := d.PartName(0); return err }},
		{"PartHash", func(d *Document) error { _, err := d.PartHash(0); return err }},
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

### Step 2: Implement `lok/part.go`

```go
//go:build linux || darwin

package lok

// Parts returns the number of parts (sheets/pages/slides).
func (d *Document) Parts() (int, error) {
	unlock, err := d.guard()
	if err != nil {
		return 0, err
	}
	defer unlock()
	n := d.office.be.DocumentGetParts(d.h)
	if n < 0 {
		return 0, &LOKError{Op: "Parts", Detail: "LOK returned -1"}
	}
	return n, nil
}

// Part returns the currently-active part index.
func (d *Document) Part() (int, error) {
	unlock, err := d.guard()
	if err != nil {
		return 0, err
	}
	defer unlock()
	n := d.office.be.DocumentGetPart(d.h)
	if n < 0 {
		return 0, &LOKError{Op: "Part", Detail: "LOK returned -1"}
	}
	return n, nil
}

// SetPart activates the part at index n.
func (d *Document) SetPart(n int) error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	d.office.be.DocumentSetPart(d.h, n)
	return nil
}

// SetPartMode switches the part-mode (Calc's "view" mode, etc.).
// Values are the LOK_PARTMODE_* enums from LibreOfficeKitEnums.h.
func (d *Document) SetPartMode(mode int) error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	d.office.be.DocumentSetPartMode(d.h, mode)
	return nil
}

// PartName returns the display name of the given part.
func (d *Document) PartName(n int) (string, error) {
	unlock, err := d.guard()
	if err != nil {
		return "", err
	}
	defer unlock()
	return d.office.be.DocumentGetPartName(d.h, n), nil
}

// PartHash returns the stable content hash of the given part.
func (d *Document) PartHash(n int) (string, error) {
	unlock, err := d.guard()
	if err != nil {
		return "", err
	}
	defer unlock()
	return d.office.be.DocumentGetPartHash(d.h, n), nil
}
```

### Step 3: Run + commit

```bash
make all && make cover-gate
git add lok/part.go lok/part_test.go
git commit -m "feat(lok): Document.Parts/Part/SetPart/PartName/PartHash

Basic part enumeration + activation + per-part name/hash lookup.
Parts and Part map LOK's -1 to *LOKError; SetPart and SetPartMode
are void on success; PartName and PartHash return '' for unknown
indices (matches LOK's NULL-from-vtable signal). Every method
uses the Phase 4 guard() helper for consistent ErrClosed.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: `PartInfo` + `DocumentSize` + `PartPageRectangles` (TDD)

**Files:**
- Modify: `lok/part.go`
- Modify: `lok/part_test.go`

### Step 1: Failing tests

```go
func TestPartInfo_UnwrapsJSON(t *testing.T) {
	fb := &fakeBackend{partInfos: map[int]string{0: `{"visible":true}`}}
	_, doc := loadFakeDoc(t, fb)
	raw, err := doc.PartInfo(0)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"visible":true}` {
		t.Errorf("PartInfo=%q", string(raw))
	}
}

func TestPartInfo_EmptyIsNil(t *testing.T) {
	// Writer/Calc docs legitimately return empty — not an error.
	_, doc := loadFakeDoc(t, &fakeBackend{})
	raw, err := doc.PartInfo(0)
	if err != nil {
		t.Fatalf("empty PartInfo: err=%v, want nil", err)
	}
	if raw != nil {
		t.Errorf("empty PartInfo: raw=%q, want nil", string(raw))
	}
}

func TestDocumentSize_Reads(t *testing.T) {
	fb := &fakeBackend{docWidthTwips: 12240, docHeightTwips: 15840}
	_, doc := loadFakeDoc(t, fb)
	w, h, err := doc.DocumentSize()
	if err != nil {
		t.Fatal(err)
	}
	if w != 12240 || h != 15840 {
		t.Errorf("DocumentSize=(%d, %d), want (12240, 15840)", w, h)
	}
}

func TestPartPageRectangles_Parses(t *testing.T) {
	fb := &fakeBackend{
		partRects: "0, 0, 12240, 15840; 0, 15840, 12240, 15840",
	}
	_, doc := loadFakeDoc(t, fb)
	rects, err := doc.PartPageRectangles()
	if err != nil {
		t.Fatal(err)
	}
	want := []TwipRect{
		{X: 0, Y: 0, W: 12240, H: 15840},
		{X: 0, Y: 15840, W: 12240, H: 15840},
	}
	if len(rects) != len(want) {
		t.Fatalf("got %d rects, want %d", len(rects), len(want))
	}
	for i := range want {
		if rects[i] != want[i] {
			t.Errorf("rect %d: got %+v, want %+v", i, rects[i], want[i])
		}
	}
}

func TestPartPageRectangles_EmptyString(t *testing.T) {
	_, doc := loadFakeDoc(t, &fakeBackend{partRects: ""})
	rects, err := doc.PartPageRectangles()
	if err != nil {
		t.Fatal(err)
	}
	if rects != nil {
		t.Errorf("PartPageRectangles on empty: got %v, want nil", rects)
	}
}

func TestPartPageRectangles_MalformedErrors(t *testing.T) {
	_, doc := loadFakeDoc(t, &fakeBackend{partRects: "abc, def, ghi, jkl"})
	_, err := doc.PartPageRectangles()
	var lokErr *LOKError
	if !errors.As(err, &lokErr) {
		t.Errorf("want *LOKError, got %T %v", err, err)
	}
}

func TestParsePartPageRectangles_Direct(t *testing.T) {
	// Direct test of the parser covers edge cases without going
	// through the fake.
	cases := []struct {
		in   string
		want []TwipRect
		err  bool
	}{
		{"", nil, false},
		{"0, 0, 100, 200", []TwipRect{{0, 0, 100, 200}}, false},
		{"0,0,100,200", []TwipRect{{0, 0, 100, 200}}, false},                                            // no spaces
		{"0, 0, 100, 200; 100, 0, 50, 200", []TwipRect{{0, 0, 100, 200}, {100, 0, 50, 200}}, false},     // multi
		{"0, 0, 100, 200;", []TwipRect{{0, 0, 100, 200}}, false},                                        // trailing ';' (LO sometimes emits this)
		{"-10, -20, 100, 200", []TwipRect{{-10, -20, 100, 200}}, false},                                 // negative origin is legal
		{"garbage", nil, true},                                                                          // unparseable
		{"1, 2, 3", nil, true},                                                                          // too few fields
	}
	for _, tc := range cases {
		got, err := parsePartPageRectangles(tc.in)
		if (err != nil) != tc.err {
			t.Errorf("input %q: err=%v, wantErr=%v", tc.in, err, tc.err)
			continue
		}
		if !tc.err && len(got) != len(tc.want) {
			t.Errorf("input %q: len=%d, want %d", tc.in, len(got), len(tc.want))
		}
	}
}
```

### Step 2: Append to `lok/part.go`

```go
import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// TwipRect is a rectangle in LOK's twip coordinates (1/1440 inch).
type TwipRect struct {
	X, Y, W, H int64
}

// PartInfo returns the part's LOK JSON metadata as json.RawMessage,
// or (nil, nil) when LOK returns an empty string. Writer and Calc
// documents legitimately return empty — only Impress populates
// per-part info in LOK 24.8. Callers that require populated info
// should check `raw == nil` and act accordingly; this is not an
// error condition at the binding level.
func (d *Document) PartInfo(n int) (json.RawMessage, error) {
	unlock, err := d.guard()
	if err != nil {
		return nil, err
	}
	defer unlock()
	raw := d.office.be.DocumentGetPartInfo(d.h, n)
	if raw == "" {
		return nil, nil
	}
	return json.RawMessage(raw), nil
}

// DocumentSize returns the document's (width, height) in twips.
func (d *Document) DocumentSize() (widthTwips, heightTwips int64, err error) {
	unlock, gerr := d.guard()
	if gerr != nil {
		return 0, 0, gerr
	}
	defer unlock()
	w, h := d.office.be.DocumentGetDocumentSize(d.h)
	return w, h, nil
}

// PartPageRectangles returns the page rectangles for the current
// part in twip coordinates. An empty LOK response yields nil, nil.
func (d *Document) PartPageRectangles() ([]TwipRect, error) {
	unlock, err := d.guard()
	if err != nil {
		return nil, err
	}
	defer unlock()
	raw := d.office.be.DocumentGetPartPageRectangles(d.h)
	if raw == "" {
		return nil, nil
	}
	return parsePartPageRectangles(raw)
}

// parsePartPageRectangles parses LOK's "x, y, w, h; x, y, w, h; …"
// format into a []TwipRect. Empty input yields (nil, nil).
// Malformed input surfaces as *LOKError.
func parsePartPageRectangles(s string) ([]TwipRect, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	groups := strings.Split(s, ";")
	out := make([]TwipRect, 0, len(groups))
	for _, g := range groups {
		g = strings.TrimSpace(g)
		if g == "" {
			continue
		}
		fields := strings.Split(g, ",")
		if len(fields) != 4 {
			return nil, &LOKError{Op: "PartPageRectangles", Detail: fmt.Sprintf("expected 4 fields, got %d: %q", len(fields), g)}
		}
		vals := [4]int64{}
		for i, f := range fields {
			v, err := strconv.ParseInt(strings.TrimSpace(f), 10, 64)
			if err != nil {
				return nil, &LOKError{Op: "PartPageRectangles", Detail: err.Error(), err: err}
			}
			vals[i] = v
		}
		out = append(out, TwipRect{X: vals[0], Y: vals[1], W: vals[2], H: vals[3]})
	}
	return out, nil
}
```

### Step 3: Run + commit

```bash
make all && make cover-gate
git add lok/part.go lok/part_test.go
git commit -m "feat(lok): PartInfo + DocumentSize + PartPageRectangles

PartInfo surfaces LOK's per-part JSON as json.RawMessage; empty
response is *LOKError. DocumentSize returns (widthTwips,
heightTwips, error) via the two-out-param cgo call. PartPage
Rectangles parses LOK's \"x, y, w, h; x, y, w, h; …\" string
format into []TwipRect; a direct unit test of the parser covers
the whitespace / malformed edge cases without going through the
fake.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 5: `SetOutlineState` (TDD)

**Files:**
- Modify: `lok/part.go`
- Modify: `lok/part_test.go`

### Step 1: Failing tests

```go
func TestSetOutlineState_PassesParams(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	if err := doc.SetOutlineState(true, 2, 5, true); err != nil {
		t.Fatal(err)
	}
	if !fb.lastOutlineCol || fb.lastOutlineLevel != 2 || fb.lastOutlineIndex != 5 || !fb.lastOutlineHidden {
		t.Errorf("outline state recorded (col=%v, lvl=%d, idx=%d, hidden=%v)",
			fb.lastOutlineCol, fb.lastOutlineLevel, fb.lastOutlineIndex, fb.lastOutlineHidden)
	}
}

func TestSetOutlineState_AfterCloseErrors(t *testing.T) {
	_, doc := loadFakeDoc(t, &fakeBackend{})
	doc.Close()
	if err := doc.SetOutlineState(false, 0, 0, false); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}
```

### Step 2: Append to `lok/part.go`

```go
// SetOutlineState toggles outline-group visibility. column=true for
// Calc column grouping, false for row grouping. level is the outline
// depth (1-based in LO's UI, 0-based in the header). index is the
// group index at that level. hidden collapses the group when true.
func (d *Document) SetOutlineState(column bool, level, index int, hidden bool) error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	d.office.be.DocumentSetOutlineState(d.h, column, level, index, hidden)
	return nil
}
```

### Step 3: Run + commit

```bash
make all && make cover-gate
git add lok/part.go lok/part_test.go
git commit -m "feat(lok): Document.SetOutlineState

Spec called for SetOutlineState(column, level, hidden); the
vendored LOK 24.8 header's setOutlineState takes (column bool,
level, index int, hidden bool). Plan follows the header.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 6: Integration test

**Files:**
- Modify: `lok/integration_test.go`

### Step 1: Append Part subtests to `TestIntegration_FullLifecycle`

```go
	// Part + size round-trip on doc.

	nParts, err := doc.Parts()
	if err != nil {
		t.Fatalf("Parts: %v", err)
	}
	if nParts < 1 {
		t.Fatalf("Parts returned %d; want >=1 for a loaded ODT", nParts)
	}

	activePart, err := doc.Part()
	if err != nil {
		t.Fatalf("Part: %v", err)
	}
	if activePart < 0 || activePart >= nParts {
		t.Errorf("Part out of range: got %d, want [0, %d)", activePart, nParts)
	}

	partName, err := doc.PartName(activePart)
	if err != nil {
		t.Errorf("PartName(%d): %v", activePart, err)
	}
	_ = partName // value is LO-specific; just assert no error

	partHash, err := doc.PartHash(activePart)
	if err != nil {
		t.Errorf("PartHash(%d): %v", activePart, err)
	}
	if partHash == "" {
		t.Log("PartHash empty; LO may not compute it for Writer docs")
	}

	if _, err := doc.PartInfo(activePart); err != nil {
		// PartInfo is only populated for Impress; Writer returns empty.
		t.Logf("PartInfo(%d): %v (expected for non-Impress docs)", activePart, err)
	}

	w, h, err := doc.DocumentSize()
	if err != nil {
		t.Fatalf("DocumentSize: %v", err)
	}
	if w <= 0 || h <= 0 {
		t.Errorf("DocumentSize=(%d, %d); want positive", w, h)
	}

	rects, err := doc.PartPageRectangles()
	if err != nil {
		t.Errorf("PartPageRectangles: %v", err)
	}
	if len(rects) == 0 {
		t.Log("PartPageRectangles empty; LO may defer until initializeForRendering")
	}

	if err := doc.SetPart(0); err != nil {
		t.Errorf("SetPart(0): %v", err)
	}
```

### Step 2: Run against real LO

```bash
LOK_PATH=/usr/lib64/libreoffice/program make test-integration
```

### Step 3: Commit

```bash
git add lok/integration_test.go
git commit -m "test(lok): integration tests for parts + sizing

Extends TestIntegration_FullLifecycle with a part round-trip on
the loaded ODT: Parts/Part/PartName/PartHash/PartInfo/
DocumentSize/PartPageRectangles/SetPart. PartInfo and
PartPageRectangles are logged rather than asserted to positive
values because LO's population of those for Writer documents
varies; the primary assertions are 'no error' and 'shape sane'.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 7: Final verification + PR

Standard sequence: `make clean`, `make all`, `make cover-gate`,
`make test-integration`, push, `gh pr create`.

---

## Acceptance criteria (adjusted for spec deviations)

- [ ] `Parts()`, `Part()`, `PartName`, `PartHash` return integer or
      string plus error; post-Close returns `ErrClosed`.
- [ ] `SetPart`, `SetPartMode`, `SetOutlineState` return `nil` on
      success, `ErrClosed` post-Close.
- [ ] `PartInfo` returns `json.RawMessage`; empty LOK response is
      `*LOKError`.
- [ ] `DocumentSize` returns two twip values plus error.
- [ ] `PartPageRectangles` returns `[]TwipRect`; malformed LOK
      response surfaces as `*LOKError`.
- [ ] Integration test round-trips every method against real LO.
- [ ] `make cover-gate` ≥ 90%.
- [ ] Nothing from Phase 6+ (Rendering) sneaks in.

When every box is ticked, `chore/parts-and-size` is ready to merge;
Phase 6's plan (Rendering) can begin.
