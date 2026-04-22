# Phase 4 — Views Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give `lok.Document` the ability to create, enumerate,
activate, and destroy views plus the per-view configurators LOK
24.8 exposes. Public API:

```go
type ViewID int

func (*Document) CreateView() (ViewID, error)
func (*Document) CreateViewWithOptions(opts string) (ViewID, error)
func (*Document) DestroyView(id ViewID) error
func (*Document) SetView(id ViewID) error
func (*Document) View() (ViewID, error)
func (*Document) Views() ([]ViewID, error)

// Per-view configurators
func (*Document) SetViewLanguage(id ViewID, lang string) error
func (*Document) SetViewReadOnly(id ViewID, readOnly bool) error
func (*Document) SetAccessibilityState(id ViewID, enabled bool) error
func (*Document) SetViewTimezone(id ViewID, tz string) error
```

The spec's §Phase 4 lists all of the above except `SetViewTimezone`;
the vendored LOK 24.8 header exposes it at the same tier as
`SetViewLanguage`, so picking it up here avoids a one-off follow-up.

The spec's table shows `View() ViewID` with no error return. The
plan returns `(ViewID, error)` so a closed-doc call can surface
`ErrClosed` instead of lying with a zero value. Same reasoning
applied to `Views() ([]ViewID, error)`. Flag in the PR body.

**Architecture:**
- `internal/lokc` gains thin 1:1 wrappers: `DocumentCreateView`,
  `DocumentCreateViewWithOptions`, `DocumentDestroyView`,
  `DocumentSetView`, `DocumentGetView`, `DocumentGetViewsCount`,
  `DocumentGetViewIds`, `DocumentSetViewLanguage`,
  `DocumentSetViewReadOnly`, `DocumentSetAccessibilityState`,
  `DocumentSetViewTimezone`. Each returns a raw scalar or a slice;
  the Go `lok` wrappers add the office mutex and error wrapping.
- `Views()` calls `DocumentGetViewsCount` to size a `make([]C.int, n)`,
  then `DocumentGetViewIds` to populate. Per the header, `getViewIds`
  takes a pre-allocated array + size and returns bool.
- `ViewID` is a named `int` type for clarity at the Go surface;
  converts to `C.int` at the cgo boundary.
- No new sentinel errors are required — `ErrClosed` already covers
  post-Close calls; `*LOKError{Op, Detail, err}` wraps backend
  failures (there aren't really any — the LOK view methods return
  `void` or `int` without an error channel, so we only surface
  "closed doc" and "invalid view ID" cases).

**Coverage gate:** unchanged at 90% across `./internal/lokc/...` +
`./lok/...`.

**Branching:** `chore/views`, branched from `main` after PR #8 merged.

---

## Files

| Path | Role |
|------|------|
| `internal/lokc/view.go` (create) | Cgo wrappers + handle-less helpers; `//go:build linux || darwin` |
| `internal/lokc/view_test.go` (create) | Unit tests via `NewFakeDocumentHandle` |
| `lok/view.go` (create) | `ViewID`, `Document.CreateView` / `DestroyView` / `SetView` / `View` / `Views` / `SetView*`; `//go:build linux || darwin` |
| `lok/view_test.go` (create) | Unit tests via `fakeBackend` |
| `lok/backend.go` (modify) | Extend `backend` interface with view methods |
| `lok/real_backend.go` (modify) | Wire the new methods through `internal/lokc` |
| `lok/real_backend_test.go` (modify) | Cover the new forwarding via `NewFakeDocumentHandle` |
| `lok/office_test.go` (modify) | Extend `fakeBackend` with view state |
| `lok/integration_test.go` (modify) | Add view round-trip subtest inside `TestIntegration_FullLifecycle` |

---

## Task 0: Branch prep

- [ ] **Step 1: Sync main**
  ```bash
  git checkout main && git pull --ff-only && git status --short
  ```
  Expected: empty, main at the post-PR-8 tip.

- [ ] **Step 2: Create branch**
  ```bash
  git checkout -b chore/views && git branch --show-current
  ```
  Expected: `chore/views`.

---

## Task 1: `internal/lokc` view wrappers (TDD)

**Files:**
- Create: `internal/lokc/view.go`
- Create: `internal/lokc/view_test.go`

### 1.1 Failing tests

- [ ] **Step 1: Create `internal/lokc/view_test.go`**

  ```go
  //go:build linux || darwin

  package lokc

  import "testing"

  func TestDocumentView_NilHandleAreNoOps(t *testing.T) {
  	var d DocumentHandle
  	if got := DocumentCreateView(d); got != -1 {
  		t.Errorf("CreateView on nil: got %d, want -1", got)
  	}
  	if got := DocumentCreateViewWithOptions(d, "foo=1"); got != -1 {
  		t.Errorf("CreateViewWithOptions on nil: got %d, want -1", got)
  	}
  	if got := DocumentGetView(d); got != -1 {
  		t.Errorf("GetView on nil: got %d, want -1", got)
  	}
  	if got := DocumentGetViewsCount(d); got != 0 {
  		t.Errorf("GetViewsCount on nil: got %d, want 0", got)
  	}
  	if ids := DocumentGetViewIds(d); ids != nil {
  		t.Errorf("GetViewIds on nil: got %v, want nil", ids)
  	}
  	// Void wrappers must not panic.
  	DocumentDestroyView(d, 0)
  	DocumentSetView(d, 0)
  	DocumentSetViewLanguage(d, 0, "en-US")
  	DocumentSetViewReadOnly(d, 0, true)
  	DocumentSetAccessibilityState(d, 0, true)
  	DocumentSetViewTimezone(d, 0, "UTC")
  }

  func TestDocumentView_FakeHandle_SafeNoOps(t *testing.T) {
  	d := NewFakeDocumentHandle()
  	t.Cleanup(func() { FreeFakeDocumentHandle(d) })

  	// pClass is NULL — C guards short-circuit every call. We're only
  	// verifying the Go-side CString/free/cgo-call path doesn't crash.
  	DocumentCreateView(d)
  	DocumentCreateViewWithOptions(d, "a=1")
  	DocumentDestroyView(d, 0)
  	DocumentSetView(d, 0)
  	DocumentGetView(d)
  	DocumentGetViewsCount(d)
  	DocumentGetViewIds(d)
  	DocumentSetViewLanguage(d, 0, "en-US")
  	DocumentSetViewReadOnly(d, 0, true)
  	DocumentSetAccessibilityState(d, 0, true)
  	DocumentSetViewTimezone(d, 0, "UTC")
  }
  ```

- [ ] **Step 2: Run — red**

  Run: `go test ./internal/lokc/... -run DocumentView`
  Expected: undefined symbols for every wrapper. Report output.

### 1.2 Implement

- [ ] **Step 3: Create `internal/lokc/view.go`**

  ```go
  //go:build linux || darwin

  package lokc

  /*
  #cgo CFLAGS: -I${SRCDIR}/../../third_party/lok -DLOK_USE_UNSTABLE_API
  #include <stdbool.h>
  #include <stdlib.h>
  #include "LibreOfficeKit/LibreOfficeKit.h"

  static int go_doc_create_view(LibreOfficeKitDocument* d) {
      if (d == NULL || d->pClass == NULL || d->pClass->createView == NULL) return -1;
      return d->pClass->createView(d);
  }
  static int go_doc_create_view_with_options(LibreOfficeKitDocument* d, const char* opts) {
      if (d == NULL || d->pClass == NULL || d->pClass->createViewWithOptions == NULL) return -1;
      return d->pClass->createViewWithOptions(d, opts);
  }
  static void go_doc_destroy_view(LibreOfficeKitDocument* d, int id) {
      if (d == NULL || d->pClass == NULL || d->pClass->destroyView == NULL) return;
      d->pClass->destroyView(d, id);
  }
  static void go_doc_set_view(LibreOfficeKitDocument* d, int id) {
      if (d == NULL || d->pClass == NULL || d->pClass->setView == NULL) return;
      d->pClass->setView(d, id);
  }
  static int go_doc_get_view(LibreOfficeKitDocument* d) {
      if (d == NULL || d->pClass == NULL || d->pClass->getView == NULL) return -1;
      return d->pClass->getView(d);
  }
  static int go_doc_get_views_count(LibreOfficeKitDocument* d) {
      if (d == NULL || d->pClass == NULL || d->pClass->getViewsCount == NULL) return 0;
      return d->pClass->getViewsCount(d);
  }
  static bool go_doc_get_view_ids(LibreOfficeKitDocument* d, int* buf, size_t n) {
      if (d == NULL || d->pClass == NULL || d->pClass->getViewIds == NULL) return false;
      return d->pClass->getViewIds(d, buf, n);
  }
  static void go_doc_set_view_language(LibreOfficeKitDocument* d, int id, const char* lang) {
      if (d == NULL || d->pClass == NULL || d->pClass->setViewLanguage == NULL) return;
      d->pClass->setViewLanguage(d, id, lang);
  }
  static void go_doc_set_view_read_only(LibreOfficeKitDocument* d, int id, bool ro) {
      if (d == NULL || d->pClass == NULL || d->pClass->setViewReadOnly == NULL) return;
      d->pClass->setViewReadOnly(d, id, ro);
  }
  static void go_doc_set_accessibility_state(LibreOfficeKitDocument* d, int id, bool en) {
      if (d == NULL || d->pClass == NULL || d->pClass->setAccessibilityState == NULL) return;
      d->pClass->setAccessibilityState(d, id, en);
  }
  static void go_doc_set_view_timezone(LibreOfficeKitDocument* d, int id, const char* tz) {
      if (d == NULL || d->pClass == NULL || d->pClass->setViewTimezone == NULL) return;
      d->pClass->setViewTimezone(d, id, tz);
  }
  */
  import "C"

  import "unsafe"

  // DocumentCreateView returns the new view ID, or -1 if the document
  // is invalid / the vtable entry is missing.
  func DocumentCreateView(d DocumentHandle) int {
  	if !d.IsValid() {
  		return -1
  	}
  	return int(C.go_doc_create_view(d.p))
  }

  // DocumentCreateViewWithOptions forwards the raw options string.
  // An empty string is passed through as a zero-length C string
  // (not NULL) because LO's NULL-tolerance is undocumented for this
  // entry and we prefer the safer convention.
  func DocumentCreateViewWithOptions(d DocumentHandle, options string) int {
  	if !d.IsValid() {
  		return -1
  	}
  	copts := C.CString(options)
  	defer C.free(unsafe.Pointer(copts))
  	return int(C.go_doc_create_view_with_options(d.p, copts))
  }

  // DocumentDestroyView is idempotent on a zero handle / missing
  // vtable; the guarded C wrapper handles both.
  func DocumentDestroyView(d DocumentHandle, id int) {
  	if !d.IsValid() {
  		return
  	}
  	C.go_doc_destroy_view(d.p, C.int(id))
  }

  // DocumentSetView activates the given view on the document.
  func DocumentSetView(d DocumentHandle, id int) {
  	if !d.IsValid() {
  		return
  	}
  	C.go_doc_set_view(d.p, C.int(id))
  }

  // DocumentGetView returns the active view ID, or -1.
  func DocumentGetView(d DocumentHandle) int {
  	if !d.IsValid() {
  		return -1
  	}
  	return int(C.go_doc_get_view(d.p))
  }

  // DocumentGetViewsCount returns the number of live views, 0 on nil.
  func DocumentGetViewsCount(d DocumentHandle) int {
  	if !d.IsValid() {
  		return 0
  	}
  	return int(C.go_doc_get_views_count(d.p))
  }

  // DocumentGetViewIds returns the IDs of live views in document
  // order. Returns nil if the handle is invalid, the vtable is
  // missing, LOK reports failure, or no views are live. A negative
  // count from LOK (shouldn't happen but the API returns int not
  // size_t) is treated as "no data" and yields nil.
  func DocumentGetViewIds(d DocumentHandle) []int {
  	if !d.IsValid() {
  		return nil
  	}
  	n := int(C.go_doc_get_views_count(d.p))
  	if n <= 0 {
  		return nil
  	}
  	buf := make([]C.int, n)
  	if !bool(C.go_doc_get_view_ids(d.p, (*C.int)(unsafe.Pointer(&buf[0])), C.size_t(n))) {
  		return nil
  	}
  	out := make([]int, n)
  	for i, v := range buf {
  		out[i] = int(v)
  	}
  	return out
  }

  // DocumentSetViewLanguage / ReadOnly / AccessibilityState / Timezone
  // forward to the identically-named pClass entries.
  func DocumentSetViewLanguage(d DocumentHandle, id int, lang string) {
  	if !d.IsValid() {
  		return
  	}
  	c := C.CString(lang)
  	defer C.free(unsafe.Pointer(c))
  	C.go_doc_set_view_language(d.p, C.int(id), c)
  }

  func DocumentSetViewReadOnly(d DocumentHandle, id int, readOnly bool) {
  	if !d.IsValid() {
  		return
  	}
  	C.go_doc_set_view_read_only(d.p, C.int(id), C.bool(readOnly))
  }

  func DocumentSetAccessibilityState(d DocumentHandle, id int, enabled bool) {
  	if !d.IsValid() {
  		return
  	}
  	C.go_doc_set_accessibility_state(d.p, C.int(id), C.bool(enabled))
  }

  func DocumentSetViewTimezone(d DocumentHandle, id int, tz string) {
  	if !d.IsValid() {
  		return
  	}
  	c := C.CString(tz)
  	defer C.free(unsafe.Pointer(c))
  	C.go_doc_set_view_timezone(d.p, C.int(id), c)
  }
  ```

- [ ] **Step 4: Run tests — green**

  Run: `go test -race ./internal/lokc/...`
  Expected: PASS.

- [ ] **Step 5: Cover-gate**

  Run: `make cover-gate`
  Expected: ≥ 90% combined. Report total.

- [ ] **Step 6: Commit**

  ```bash
  git add internal/lokc/view.go internal/lokc/view_test.go
  git commit -m "feat(lokc): add view-level cgo wrappers

DocumentCreateView{,WithOptions}, DocumentDestroyView, SetView,
GetView, GetViewsCount, GetViewIds, SetViewLanguage, SetViewReadOnly,
SetAccessibilityState, SetViewTimezone. Each is a guarded 1:1
wrapper over the pClass vtable entry. GetViewIds sizes via
getViewsCount then reads through a single getViewIds call.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 2: `lok` backend extension + fakeBackend

**Files:**
- Modify: `lok/backend.go`
- Modify: `lok/real_backend.go`
- Modify: `lok/real_backend_test.go`
- Modify: `lok/office_test.go`

### Step 1: Extend `lok/backend.go`

Append to the `backend` interface:
```go
	DocumentCreateView(d documentHandle) int
	DocumentCreateViewWithOptions(d documentHandle, options string) int
	DocumentDestroyView(d documentHandle, id int)
	DocumentSetView(d documentHandle, id int)
	DocumentGetView(d documentHandle) int
	DocumentGetViewsCount(d documentHandle) int
	DocumentGetViewIds(d documentHandle) []int
	DocumentSetViewLanguage(d documentHandle, id int, lang string)
	DocumentSetViewReadOnly(d documentHandle, id int, readOnly bool)
	DocumentSetAccessibilityState(d documentHandle, id int, enabled bool)
	DocumentSetViewTimezone(d documentHandle, id int, tz string)
```

### Step 2: Extend `lok/real_backend.go`

For each, pattern:
```go
func (realBackend) DocumentCreateView(d documentHandle) int {
	return lokc.DocumentCreateView(mustDoc(d).d)
}
// ... etc for every method
```

### Step 3: Extend `fakeBackend` in `lok/office_test.go`

Add state fields to capture calls:
```go
	viewsNextID      int // monotonic IDs for the fake, starting at 1000 to stay visually distinct from real LO view IDs (which start at 0) in test output
	viewsLive        []int
	viewActive       int
	viewCreateErr    bool // if true, CreateView returns -1 (fake signal)
	lastViewLang     string
	lastViewReadOnly bool
	lastViewA11y     bool
	lastViewTimezone string
	lastViewLangID   int
```

Add methods — a minimal but realistic fake:
```go
func (f *fakeBackend) DocumentCreateView(documentHandle) int {
	if f.viewCreateErr {
		return -1
	}
	if f.viewsNextID == 0 {
		f.viewsNextID = 1000
	}
	id := f.viewsNextID
	f.viewsNextID++
	f.viewsLive = append(f.viewsLive, id)
	f.viewActive = id
	return id
}

func (f *fakeBackend) DocumentCreateViewWithOptions(d documentHandle, _ string) int {
	return f.DocumentCreateView(d)
}

func (f *fakeBackend) DocumentDestroyView(_ documentHandle, id int) {
	for i, v := range f.viewsLive {
		if v == id {
			f.viewsLive = append(f.viewsLive[:i], f.viewsLive[i+1:]...)
			break
		}
	}
	if f.viewActive == id && len(f.viewsLive) > 0 {
		f.viewActive = f.viewsLive[0]
	} else if f.viewActive == id {
		f.viewActive = -1
	}
}

func (f *fakeBackend) DocumentSetView(_ documentHandle, id int) {
	f.viewActive = id
}

func (f *fakeBackend) DocumentGetView(documentHandle) int       { return f.viewActive }
func (f *fakeBackend) DocumentGetViewsCount(documentHandle) int { return len(f.viewsLive) }

func (f *fakeBackend) DocumentGetViewIds(documentHandle) []int {
	if len(f.viewsLive) == 0 {
		return nil
	}
	out := make([]int, len(f.viewsLive))
	copy(out, f.viewsLive)
	return out
}

func (f *fakeBackend) DocumentSetViewLanguage(_ documentHandle, id int, lang string) {
	f.lastViewLangID = id
	f.lastViewLang = lang
}

func (f *fakeBackend) DocumentSetViewReadOnly(_ documentHandle, _ int, ro bool) {
	f.lastViewReadOnly = ro
}

func (f *fakeBackend) DocumentSetAccessibilityState(_ documentHandle, _ int, en bool) {
	f.lastViewA11y = en
}

func (f *fakeBackend) DocumentSetViewTimezone(_ documentHandle, _ int, tz string) {
	f.lastViewTimezone = tz
}
```

### Step 4: Extend `lok/real_backend_test.go`

Append a test that exercises the forwarding via `NewFakeDocumentHandle`:
```go
func TestRealBackend_ViewForwarding(t *testing.T) {
	rb := realBackend{}
	fakeDoc := lokc.NewFakeDocumentHandle()
	defer lokc.FreeFakeDocumentHandle(fakeDoc)
	rdoc := realDocumentHandle{d: fakeDoc}

	// All calls must short-circuit in C and not panic. Return values
	// reflect the guarded-NULL path: CreateView/GetView → -1,
	// GetViewsCount → 0, GetViewIds → nil.
	if got := rb.DocumentCreateView(rdoc); got != -1 {
		t.Errorf("CreateView: got %d, want -1", got)
	}
	if got := rb.DocumentCreateViewWithOptions(rdoc, "x=1"); got != -1 {
		t.Errorf("CreateViewWithOptions: got %d, want -1", got)
	}
	if got := rb.DocumentGetView(rdoc); got != -1 {
		t.Errorf("GetView: got %d, want -1", got)
	}
	if got := rb.DocumentGetViewsCount(rdoc); got != 0 {
		t.Errorf("GetViewsCount: got %d, want 0", got)
	}
	if got := rb.DocumentGetViewIds(rdoc); got != nil {
		t.Errorf("GetViewIds: got %v, want nil", got)
	}
	rb.DocumentDestroyView(rdoc, 0)
	rb.DocumentSetView(rdoc, 0)
	rb.DocumentSetViewLanguage(rdoc, 0, "en-US")
	rb.DocumentSetViewReadOnly(rdoc, 0, true)
	rb.DocumentSetAccessibilityState(rdoc, 0, true)
	rb.DocumentSetViewTimezone(rdoc, 0, "UTC")
}
```

### Step 5: Run + Commit

```bash
make all && make cover-gate
git add lok/backend.go lok/real_backend.go lok/real_backend_test.go lok/office_test.go
git commit -m "feat(lok): backend seam and fake for view operations

backend interface grows 11 view methods; realBackend forwards
each to internal/lokc; fakeBackend carries viewsLive + viewActive
state so the Document view tests can exercise real semantics
(create returns monotonic IDs; Destroy removes from the list; Set
updates the active view; Get* reads back).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: `Document.CreateView` / `DestroyView` / `SetView` / `View` / `Views` (TDD)

**Files:**
- Create: `lok/view.go`
- Create: `lok/view_test.go`

### Step 1: Failing tests — `lok/view_test.go`

```go
//go:build linux || darwin

package lok

import (
	"errors"
	"testing"
)

func loadFakeDoc(t *testing.T, fb *fakeBackend) (*Office, *Document) {
	t.Helper()
	withFakeBackend(t, fb)
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

func TestCreateView_AllocatesID(t *testing.T) {
	_, doc := loadFakeDoc(t, &fakeBackend{})
	id, err := doc.CreateView()
	if err != nil {
		t.Fatal(err)
	}
	if id < 0 {
		t.Errorf("CreateView: got %d, want non-negative", id)
	}
}

func TestCreateView_BackendFailureErrors(t *testing.T) {
	_, doc := loadFakeDoc(t, &fakeBackend{viewCreateErr: true})
	_, err := doc.CreateView()
	var lokErr *LOKError
	if !errors.As(err, &lokErr) {
		t.Errorf("want *LOKError, got %T %v", err, err)
	}
}

func TestCreateViewWithOptions_PassesThrough(t *testing.T) {
	_, doc := loadFakeDoc(t, &fakeBackend{})
	id, err := doc.CreateViewWithOptions("Language=de-DE")
	if err != nil {
		t.Fatal(err)
	}
	if id < 0 {
		t.Errorf("CreateViewWithOptions: got %d", id)
	}
}

func TestSetView_UpdatesActiveView(t *testing.T) {
	_, doc := loadFakeDoc(t, &fakeBackend{})
	id1, _ := doc.CreateView()
	id2, _ := doc.CreateView()
	if err := doc.SetView(id1); err != nil {
		t.Fatal(err)
	}
	got, err := doc.View()
	if err != nil {
		t.Fatal(err)
	}
	if got != id1 {
		t.Errorf("View()=%d after SetView(%d)", got, id1)
	}
	_ = id2
}

func TestViews_ListsLiveIDs(t *testing.T) {
	_, doc := loadFakeDoc(t, &fakeBackend{})
	a, _ := doc.CreateView()
	b, _ := doc.CreateView()
	ids, err := doc.Views()
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != a || ids[1] != b {
		t.Errorf("Views=%v, want [%d %d]", ids, a, b)
	}
}

func TestDestroyView_RemovesFromList(t *testing.T) {
	_, doc := loadFakeDoc(t, &fakeBackend{})
	a, _ := doc.CreateView()
	b, _ := doc.CreateView()
	if err := doc.DestroyView(a); err != nil {
		t.Fatal(err)
	}
	ids, err := doc.Views()
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != b {
		t.Errorf("after Destroy(%d), Views=%v, want [%d]", a, ids, b)
	}
}

func TestView_AfterCloseErrors(t *testing.T) {
	o, doc := loadFakeDoc(t, &fakeBackend{})
	_, _ = doc.CreateView()
	doc.Close()
	if _, err := doc.View(); !errors.Is(err, ErrClosed) {
		t.Errorf("View after Close: want ErrClosed, got %v", err)
	}
	if _, err := doc.Views(); !errors.Is(err, ErrClosed) {
		t.Errorf("Views after Close: want ErrClosed, got %v", err)
	}
	if err := doc.SetView(0); !errors.Is(err, ErrClosed) {
		t.Errorf("SetView after Close: want ErrClosed, got %v", err)
	}
	if err := doc.DestroyView(0); !errors.Is(err, ErrClosed) {
		t.Errorf("DestroyView after Close: want ErrClosed, got %v", err)
	}
	if _, err := doc.CreateView(); !errors.Is(err, ErrClosed) {
		t.Errorf("CreateView after Close: want ErrClosed, got %v", err)
	}
	_ = o
}
```

### Step 2: Run — red

Expect build errors for `CreateView`, `CreateViewWithOptions`, `DestroyView`, `SetView`, `View`, `Views`.

### Step 3: Create `lok/view.go`

```go
//go:build linux || darwin

package lok

// ViewID is a LibreOfficeKit view identifier. LOK uses `int` —
// ViewID exists for self-documenting call sites.
type ViewID int

// CreateView creates a new view on the document and returns its ID.
// Returns ErrClosed on a closed document; wraps a backend error as
// *LOKError if LOK rejects the call.
func (d *Document) CreateView() (ViewID, error) {
	d.office.mu.Lock()
	defer d.office.mu.Unlock()
	if d.closed {
		return 0, ErrClosed
	}
	id := d.office.be.DocumentCreateView(d.h)
	if id < 0 {
		return 0, &LOKError{Op: "CreateView", Detail: "LOK returned -1"}
	}
	return ViewID(id), nil
}

// CreateViewWithOptions forwards a raw options string to
// pClass->createViewWithOptions. Same error contract as CreateView.
func (d *Document) CreateViewWithOptions(options string) (ViewID, error) {
	d.office.mu.Lock()
	defer d.office.mu.Unlock()
	if d.closed {
		return 0, ErrClosed
	}
	id := d.office.be.DocumentCreateViewWithOptions(d.h, options)
	if id < 0 {
		return 0, &LOKError{Op: "CreateViewWithOptions", Detail: "LOK returned -1"}
	}
	return ViewID(id), nil
}

// DestroyView removes the view. LOK returns void, so errors surface
// only from the closed-doc check.
func (d *Document) DestroyView(id ViewID) error {
	d.office.mu.Lock()
	defer d.office.mu.Unlock()
	if d.closed {
		return ErrClosed
	}
	d.office.be.DocumentDestroyView(d.h, int(id))
	return nil
}

// SetView activates the view. LOK returns void; caller should
// confirm via View() if the ID is trusted.
func (d *Document) SetView(id ViewID) error {
	d.office.mu.Lock()
	defer d.office.mu.Unlock()
	if d.closed {
		return ErrClosed
	}
	d.office.be.DocumentSetView(d.h, int(id))
	return nil
}

// View returns the currently-active view ID.
func (d *Document) View() (ViewID, error) {
	d.office.mu.Lock()
	defer d.office.mu.Unlock()
	if d.closed {
		return 0, ErrClosed
	}
	return ViewID(d.office.be.DocumentGetView(d.h)), nil
}

// Views returns the IDs of all live views in document order.
func (d *Document) Views() ([]ViewID, error) {
	d.office.mu.Lock()
	defer d.office.mu.Unlock()
	if d.closed {
		return nil, ErrClosed
	}
	raw := d.office.be.DocumentGetViewIds(d.h)
	if raw == nil {
		return nil, nil
	}
	out := make([]ViewID, len(raw))
	for i, v := range raw {
		out[i] = ViewID(v)
	}
	return out, nil
}
```

### Step 4: Run tests — green

Run: `go test -race ./lok/...` → PASS.

### Step 5: Commit

```bash
git add lok/view.go lok/view_test.go
git commit -m "feat(lok): Document.CreateView/DestroyView/SetView/View/Views

Public view-management API. CreateView and CreateViewWithOptions
wrap the LOK calls; a -1 return is surfaced as *LOKError. Views
returns a typed []ViewID slice sized via GetViewsCount before
GetViewIds populates. Every method locks the Office mutex and
returns ErrClosed after Document.Close.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: Per-view configurators (TDD)

**Files:**
- Modify: `lok/view.go` (append)
- Modify: `lok/view_test.go` (append)

### Step 1: Failing tests

```go
func TestSetViewLanguage_Records(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	id, _ := doc.CreateView()
	if err := doc.SetViewLanguage(id, "de-DE"); err != nil {
		t.Fatal(err)
	}
	if fb.lastViewLang != "de-DE" || fb.lastViewLangID != int(id) {
		t.Errorf("SetViewLanguage recorded (id=%d lang=%q)", fb.lastViewLangID, fb.lastViewLang)
	}
}

func TestSetViewReadOnly_Records(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	id, _ := doc.CreateView()
	if err := doc.SetViewReadOnly(id, true); err != nil {
		t.Fatal(err)
	}
	if !fb.lastViewReadOnly {
		t.Error("SetViewReadOnly(true) not recorded")
	}
}

func TestSetAccessibilityState_Records(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	id, _ := doc.CreateView()
	if err := doc.SetAccessibilityState(id, true); err != nil {
		t.Fatal(err)
	}
	if !fb.lastViewA11y {
		t.Error("SetAccessibilityState(true) not recorded")
	}
}

func TestSetViewTimezone_Records(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	id, _ := doc.CreateView()
	if err := doc.SetViewTimezone(id, "Europe/Berlin"); err != nil {
		t.Fatal(err)
	}
	if fb.lastViewTimezone != "Europe/Berlin" {
		t.Errorf("SetViewTimezone: got %q", fb.lastViewTimezone)
	}
}

func TestViewConfigurators_AfterCloseErrors(t *testing.T) {
	cases := []struct {
		name string
		call func(*Document) error
	}{
		{"SetViewLanguage", func(d *Document) error { return d.SetViewLanguage(0, "x") }},
		{"SetViewReadOnly", func(d *Document) error { return d.SetViewReadOnly(0, true) }},
		{"SetAccessibilityState", func(d *Document) error { return d.SetAccessibilityState(0, true) }},
		{"SetViewTimezone", func(d *Document) error { return d.SetViewTimezone(0, "UTC") }},
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

### Step 2: Implement — append to `lok/view.go`

```go
// SetViewLanguage sets the UI language tag for a specific view.
func (d *Document) SetViewLanguage(id ViewID, lang string) error {
	d.office.mu.Lock()
	defer d.office.mu.Unlock()
	if d.closed {
		return ErrClosed
	}
	d.office.be.DocumentSetViewLanguage(d.h, int(id), lang)
	return nil
}

// SetViewReadOnly toggles the read-only state of a specific view.
func (d *Document) SetViewReadOnly(id ViewID, readOnly bool) error {
	d.office.mu.Lock()
	defer d.office.mu.Unlock()
	if d.closed {
		return ErrClosed
	}
	d.office.be.DocumentSetViewReadOnly(d.h, int(id), readOnly)
	return nil
}

// SetAccessibilityState turns the per-view accessibility pipeline
// (a11y tree generation, focus reporting) on or off.
func (d *Document) SetAccessibilityState(id ViewID, enabled bool) error {
	d.office.mu.Lock()
	defer d.office.mu.Unlock()
	if d.closed {
		return ErrClosed
	}
	d.office.be.DocumentSetAccessibilityState(d.h, int(id), enabled)
	return nil
}

// SetViewTimezone sets the IANA tz name (e.g. "Europe/Berlin") for
// the given view. Empty string falls back to LO's default.
func (d *Document) SetViewTimezone(id ViewID, tz string) error {
	d.office.mu.Lock()
	defer d.office.mu.Unlock()
	if d.closed {
		return ErrClosed
	}
	d.office.be.DocumentSetViewTimezone(d.h, int(id), tz)
	return nil
}
```

### Step 3: Commit

```bash
git add lok/view.go lok/view_test.go
git commit -m "feat(lok): per-view SetViewLanguage/ReadOnly/A11yState/Timezone

Four additional per-view configurators matching LOK 24.8's vtable
entries. Each locks the Office mutex, rejects post-Close calls
with ErrClosed, and forwards the raw arguments to the backend —
LOK itself returns void so no error surface beyond closed-doc.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 5: Integration test

**Files:**
- Modify: `lok/integration_test.go`

### Step 1: Append view subtests to `TestIntegration_FullLifecycle`

After the existing document round-trip blocks, add:

```go
	// --- View round-trip ---

	initialView, err := doc.View()
	if err != nil {
		t.Fatalf("View (initial): %v", err)
	}
	initialViews, err := doc.Views()
	if err != nil {
		t.Fatalf("Views (initial): %v", err)
	}
	if len(initialViews) != 1 {
		t.Logf("initial views: %d (LO may vary)", len(initialViews))
	}

	newView, err := doc.CreateView()
	if err != nil {
		t.Fatalf("CreateView: %v", err)
	}
	if newView == initialView {
		t.Errorf("CreateView returned same ID as initial view")
	}

	views, err := doc.Views()
	if err != nil {
		t.Fatalf("Views: %v", err)
	}
	var found bool
	for _, v := range views {
		if v == newView {
			found = true
		}
	}
	if !found {
		t.Errorf("new view %d not in Views() list %v", newView, views)
	}

	if err := doc.SetView(newView); err != nil {
		t.Errorf("SetView: %v", err)
	}
	if got, _ := doc.View(); got != newView {
		t.Errorf("View()=%d after SetView(%d)", got, newView)
	}

	if err := doc.SetViewReadOnly(newView, true); err != nil {
		t.Errorf("SetViewReadOnly: %v", err)
	}
	if err := doc.SetViewLanguage(newView, "en-US"); err != nil {
		t.Errorf("SetViewLanguage: %v", err)
	}
	if err := doc.SetAccessibilityState(newView, false); err != nil {
		t.Errorf("SetAccessibilityState: %v", err)
	}
	if err := doc.SetViewTimezone(newView, "UTC"); err != nil {
		t.Errorf("SetViewTimezone: %v", err)
	}

	if err := doc.DestroyView(newView); err != nil {
		t.Errorf("DestroyView: %v", err)
	}
```

### Step 2: Run against real LO

```bash
LOK_PATH=/usr/lib64/libreoffice/program make test-integration
```
Expected: deterministic pass.

### Step 3: Commit

```bash
git add lok/integration_test.go
git commit -m "test(lok): integration tests for view round-trip

Extends TestIntegration_FullLifecycle with the full view API:
initial View + Views, CreateView, SetView + View verification,
all four per-view configurators, DestroyView. Everything shares
the single New/Close pair per LO's per-process singleton.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 6: Final verification + PR

- [ ] **Step 1: Full workflow**
  ```bash
  make clean
  make all
  make cover-gate
  make test-integration
  LOK_PATH=/usr/lib64/libreoffice/program make test-integration
  ```
  Expected: every command exits 0. Gate ≥ 90%.

- [ ] **Step 2: Branch topology**
  `git log --oneline main..HEAD` — 5 commits above main.

- [ ] **Step 3: Open the PR**
  ```bash
  git push -u origin chore/views
  gh pr create --base main --title "Phase 4: Views" --body "..."
  ```

---

## Acceptance criteria (matches spec §Phase 4)

- [ ] `CreateView` and `CreateViewWithOptions` return a non-negative
      `ViewID` on success; `LOKError` wrapping `-1` from LOK.
- [ ] `DestroyView`/`SetView`/`SetViewLanguage`/`SetViewReadOnly`/
      `SetAccessibilityState`/`SetViewTimezone` return `nil` on
      success, `ErrClosed` post-Close.
- [ ] `View()` returns the active view ID.
- [ ] `Views()` returns `[]ViewID` of live views.
- [ ] Integration test exercises the full flow against real LO.
- [ ] `make cover-gate` still ≥ 90%.
- [ ] Nothing from Phase 5+ (Parts, Rendering, Events) sneaks in.

When every box is ticked, `chore/views` is ready to merge; Phase 5's
plan (Parts & sizing) can begin.
