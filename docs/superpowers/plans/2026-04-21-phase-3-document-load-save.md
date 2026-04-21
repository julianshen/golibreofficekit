# Phase 3 — Document Load / Save Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the public `lok` package a `Document` type that loads
documents from disk (and from a reader via a temp file), reports their
type, saves them back, and closes them. Public API:

```go
type Document struct { /* ... */ }

type DocumentType int
const (
    TypeText         DocumentType = iota // LOK_DOCTYPE_TEXT
    TypeSpreadsheet                      // LOK_DOCTYPE_SPREADSHEET
    TypePresentation                     // LOK_DOCTYPE_PRESENTATION
    TypeDrawing                          // LOK_DOCTYPE_DRAWING
    TypeOther                            // LOK_DOCTYPE_OTHER
)

func (*Office) Load(path string, opts ...LoadOption) (*Document, error)
func (*Office) LoadFromReader(r io.Reader, filter string, opts ...LoadOption) (*Document, error)
func (*Document) Type() DocumentType
func (*Document) Save() error
func (*Document) SaveAs(path, format, filterOpts string) error
func (*Document) Close() error
```

**Architecture:**
- `internal/lokc` grows thin wrappers around `documentLoad`,
  `documentLoadWithOptions`, `Document.destroy`, `Document.saveAs`,
  `Document.getDocumentType`. Fake helpers from Task 1 of Phase 2
  are extended so tests can build fake `DocumentHandle`s.
- LOK has **no plain `save()` vtable entry** — only `saveAs`. The
  `Document.Save()` public method is implemented as `saveAs` to the
  document's original URL (cached on load). This is not in the spec;
  it's a practical necessity surfaced by header review. Documented
  in the Go-doc.
- LOK takes `file:///absolute/path` URLs, not raw filesystem paths.
  `Load`/`SaveAs` convert `filepath.Abs` + `url.URL{Scheme:"file", …}`
  before crossing into `lokc`. Callers still pass raw paths.
- `LoadFromReader` streams the reader to `os.CreateTemp("", "lokdoc-*")`,
  loads that path, and deletes the temp file in `Document.Close`.
- Every `Document` method locks the `Office`-wide mutex (LOK is
  not thread-safe) and rejects post-`Close` calls with `ErrClosed`.
- `Document.Close` calls LOK's `Document.destroy`, removes itself
  from the Office's live-documents map, and cleans up temp files.

**Tech Stack:** Go 1.23+, cgo, `net/url`, `os`, `path/filepath`.

**Coverage gate:** unchanged at 90% across `./internal/lokc/...` + `./lok/...`.

**Branching:** `chore/document-load-save`, branched from `main` after
PR #6 merged.

---

## Files

| Path | Role |
|------|------|
| `internal/lokc/document.go` (create) | Cgo wrappers: `DocumentHandle`, `DocumentLoad`, `DocumentLoadWithOptions`, `DocumentDestroy`, `DocumentSaveAs`, `DocumentGetType`; `//go:build linux || darwin` |
| `internal/lokc/document_test.go` (create) | Unit tests using a fake-LOK document-class vtable via helper |
| `internal/lokc/document_test_helper.go` (create) | C fake: `calloc(LibreOfficeKitDocument)` with NULL `pClass`; exports `NewFakeDocumentHandle`, `FreeFakeDocumentHandle` |
| `lok/document.go` (create) | `Document`, `DocumentType` enum + `String()`, `Load`, `LoadFromReader`, `Close`, `Type`, `Save`, `SaveAs`; `//go:build linux || darwin` |
| `lok/document_test.go` (create) | Unit tests using the fake backend extended for document methods |
| `lok/backend.go` (modify) | Extend `backend` interface with `DocumentLoad`, `DocumentLoadWithOptions`, `DocumentDestroy`, `DocumentSaveAs`, `DocumentGetType` and a new `documentHandle` branded type |
| `lok/real_backend.go` (modify) | Wire the new backend methods to `internal/lokc`; add `realDocumentHandle` brand |
| `lok/real_backend_test.go` (modify) | Cover the new forwarding methods through `lokc.NewFakeDocumentHandle` |
| `lok/integration_test.go` (modify) | Add document-round-trip subtests inside `TestIntegration_FullLifecycle` |
| `testdata/hello.odt` (create) | Tiny ODT fixture, ≤4 KB; committed as binary |

---

## Task 0: Branch prep

- [ ] **Step 1: Sync main**

  ```bash
  git checkout main && git pull --ff-only && git status --short
  ```
  Expected: empty, main at the post-PR-6 tip.

- [ ] **Step 2: Create branch**

  ```bash
  git checkout -b chore/document-load-save && git branch --show-current
  ```
  Expected: `chore/document-load-save`.

---

## Task 1: lokc Document wrappers (TDD)

**Files:**
- Create: `internal/lokc/document.go`
- Create: `internal/lokc/document_test.go`
- Create: `internal/lokc/document_test_helper.go`

### 1.1 Failing test

- [ ] **Step 1: Write `internal/lokc/document_test.go`**

  ```go
  //go:build linux || darwin

  package lokc

  import "testing"

  func TestDocumentHandle_Nil(t *testing.T) {
  	var d DocumentHandle
  	if d.IsValid() {
  		t.Error("zero-value DocumentHandle must be invalid")
  	}
  }

  func TestDocumentWrappers_NilAreNoOps(t *testing.T) {
  	var d DocumentHandle
  	if got := DocumentGetType(d); got != 0 {
  		t.Errorf("DocumentGetType on nil: got %d, want 0", got)
  	}
  	if err := DocumentSaveAs(d, "file:///tmp/x.odt", "", ""); err == nil {
  		t.Error("DocumentSaveAs on nil must error")
  	}
  	DocumentDestroy(d) // must not panic
  }

  func TestDocumentWrappers_FakeHandle(t *testing.T) {
  	d := NewFakeDocumentHandle()
  	t.Cleanup(func() { FreeFakeDocumentHandle(d) })

  	// pClass == NULL in the fake, C wrappers short-circuit. Exercises
  	// the Go-side CString/free/cgo-call path on every wrapper.
  	_ = DocumentGetType(d)
  	_ = DocumentSaveAs(d, "file:///tmp/x.odt", "odt", "")
  	DocumentDestroy(d)
  }
  ```

- [ ] **Step 2: Run — red**

  Run: `go test ./internal/lokc/... -run 'Document'`
  Expected: build errors: `DocumentHandle`, `IsValid`, `DocumentGetType`,
  `DocumentSaveAs`, `DocumentDestroy`, `NewFakeDocumentHandle`,
  `FreeFakeDocumentHandle` undefined.

### 1.2 Implement

- [ ] **Step 3: Create `internal/lokc/document.go`**

  ```go
  //go:build linux || darwin

  package lokc

  /*
  #cgo CFLAGS: -I${SRCDIR}/../../third_party/lok -DLOK_USE_UNSTABLE_API
  #include <stdlib.h>
  #include "LibreOfficeKit/LibreOfficeKit.h"

  static LibreOfficeKitDocument* go_document_load(LibreOfficeKit* p, const char* url) {
      if (p == NULL || p->pClass == NULL || p->pClass->documentLoad == NULL) return NULL;
      return p->pClass->documentLoad(p, url);
  }

  static LibreOfficeKitDocument* go_document_load_with_options(LibreOfficeKit* p, const char* url, const char* options) {
      if (p == NULL || p->pClass == NULL || p->pClass->documentLoadWithOptions == NULL) return NULL;
      return p->pClass->documentLoadWithOptions(p, url, options);
  }

  static int go_document_get_type(LibreOfficeKitDocument* d) {
      if (d == NULL || d->pClass == NULL || d->pClass->getDocumentType == NULL) return -1;
      return d->pClass->getDocumentType(d);
  }

  static int go_document_save_as(LibreOfficeKitDocument* d, const char* url, const char* format, const char* filterOptions) {
      if (d == NULL || d->pClass == NULL || d->pClass->saveAs == NULL) return 0;
      return d->pClass->saveAs(d, url, format, filterOptions);
  }

  static void go_document_destroy(LibreOfficeKitDocument* d) {
      if (d == NULL || d->pClass == NULL || d->pClass->destroy == NULL) return;
      d->pClass->destroy(d);
  }
  */
  import "C"

  import (
  	"errors"
  	"unsafe"
  )

  // ErrSaveFailed is returned when LOK's saveAs returns 0 (false).
  var ErrSaveFailed = errors.New("lokc: saveAs returned failure")

  // DocumentHandle is an opaque pointer to a LibreOfficeKitDocument*.
  type DocumentHandle struct {
  	p *C.struct__LibreOfficeKitDocument
  }

  // IsValid reports whether the handle points at a live document.
  func (d DocumentHandle) IsValid() bool { return d.p != nil }

  // DocumentLoad calls pClass->documentLoad with the given file:// URL.
  // Returns an invalid handle if LO rejects the URL; caller should
  // then read OfficeGetError for the reason.
  func DocumentLoad(h OfficeHandle, url string) DocumentHandle {
  	if !h.IsValid() {
  		return DocumentHandle{}
  	}
  	curl := C.CString(url)
  	defer C.free(unsafe.Pointer(curl))
  	return DocumentHandle{p: C.go_document_load(h.p, curl)}
  }

  // DocumentLoadWithOptions forwards to pClass->documentLoadWithOptions.
  func DocumentLoadWithOptions(h OfficeHandle, url, options string) DocumentHandle {
  	if !h.IsValid() {
  		return DocumentHandle{}
  	}
  	curl := C.CString(url)
  	defer C.free(unsafe.Pointer(curl))
  	var copts *C.char
  	if options != "" {
  		copts = C.CString(options)
  		defer C.free(unsafe.Pointer(copts))
  	}
  	return DocumentHandle{p: C.go_document_load_with_options(h.p, curl, copts)}
  }

  // DocumentGetType returns the LOK_DOCTYPE_* integer, or -1 if the
  // handle or vtable is unavailable.
  func DocumentGetType(d DocumentHandle) int {
  	if !d.IsValid() {
  		return 0
  	}
  	return int(C.go_document_get_type(d.p))
  }

  // DocumentSaveAs forwards to pClass->saveAs. Returns ErrSaveFailed
  // on a zero return from LOK, ErrNilLibrary-style nil-handle error
  // on an invalid handle.
  func DocumentSaveAs(d DocumentHandle, url, format, filterOptions string) error {
  	if !d.IsValid() {
  		return ErrNilLibrary // re-used sentinel, semantics match
  	}
  	curl := C.CString(url)
  	defer C.free(unsafe.Pointer(curl))
  	var cformat *C.char
  	if format != "" {
  		cformat = C.CString(format)
  		defer C.free(unsafe.Pointer(cformat))
  	}
  	var cfilter *C.char
  	if filterOptions != "" {
  		cfilter = C.CString(filterOptions)
  		defer C.free(unsafe.Pointer(cfilter))
  	}
  	if C.go_document_save_as(d.p, curl, cformat, cfilter) == 0 {
  		return ErrSaveFailed
  	}
  	return nil
  }

  // DocumentDestroy is idempotent on a zero handle.
  func DocumentDestroy(d DocumentHandle) {
  	if !d.IsValid() {
  		return
  	}
  	C.go_document_destroy(d.p)
  }
  ```

- [ ] **Step 4: Create `internal/lokc/document_test_helper.go`**

  ```go
  //go:build linux || darwin

  package lokc

  /*
  #include <stdlib.h>
  #include "LibreOfficeKit/LibreOfficeKit.h"

  static LibreOfficeKitDocument* newFakeDoc(void) {
      return (LibreOfficeKitDocument*)calloc(1, sizeof(LibreOfficeKitDocument));
  }

  static void freeFakeDoc(LibreOfficeKitDocument* d) { free(d); }
  */
  import "C"

  // NewFakeDocumentHandle returns a DocumentHandle backed by a calloc'd
  // LibreOfficeKitDocument with pClass == NULL. The wrappers' C-side
  // guards make every call a safe no-op while the Go-side CString/free
  // path runs, so tests can unit-test the Go layer without real LO.
  func NewFakeDocumentHandle() DocumentHandle {
  	return DocumentHandle{p: C.newFakeDoc()}
  }

  // FreeFakeDocumentHandle releases the backing memory.
  func FreeFakeDocumentHandle(d DocumentHandle) {
  	if d.p != nil {
  		C.freeFakeDoc(d.p)
  	}
  }
  ```

- [ ] **Step 5: Run tests**

  Run: `go test -race ./internal/lokc/...`
  Expected: PASS.

- [ ] **Step 6: Coverage gate**

  Run: `make cover-gate`
  Expected: ≥ 90% (gate covers `internal/lokc` + `lok`). Report total.

- [ ] **Step 7: Commit**

  ```bash
  git add internal/lokc/document.go internal/lokc/document_test.go internal/lokc/document_test_helper.go
  git commit -m "feat(lokc): add document-level cgo wrappers

DocumentLoad/DocumentLoadWithOptions call documentLoad(WithOptions)
on the office vtable and return an opaque DocumentHandle.
DocumentGetType, DocumentSaveAs, DocumentDestroy are 1:1 guards
over pClass vtable entries with the same nil-tolerant pattern as
the office wrappers. ErrSaveFailed surfaces a zero return from
LOK's saveAs. Tests cover nil-handle and fake-pClass paths; real
round-trip is in the lok integration test.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 2: `lok` Document type + backend extension (TDD)

**Files:**
- Create: `lok/document.go`
- Create: `lok/document_test.go`
- Modify: `lok/backend.go`
- Modify: `lok/real_backend.go`
- Modify: `lok/real_backend_test.go`
- Modify: `lok/office_test.go` (extend `fakeBackend` with document methods)

Introduces `Document`, `DocumentType`, `LoadOption` — no public methods yet except `Type()`.

### 2.1 Backend seam extension

- [ ] **Step 1: Extend `lok/backend.go`**

  Append to the `backend` interface:
  ```go
  	DocumentLoad(h officeHandle, url string) (documentHandle, error)
  	DocumentLoadWithOptions(h officeHandle, url, options string) (documentHandle, error)
  	DocumentGetType(d documentHandle) int
  	DocumentSaveAs(d documentHandle, url, format, filterOptions string) error
  	DocumentDestroy(d documentHandle)
  ```

  Add the new branded handle:
  ```go
  type documentHandle interface{ documentBrand() }
  ```

- [ ] **Step 2: Extend `lok/real_backend.go`**

  ```go
  type realDocumentHandle struct{ d lokc.DocumentHandle }

  func (realDocumentHandle) documentBrand() {}

  func (realBackend) DocumentLoad(h officeHandle, url string) (documentHandle, error) {
  	doc := lokc.DocumentLoad(must(h).h, url)
  	if !doc.IsValid() {
  		// Defer to caller to surface lokc.OfficeGetError for the reason.
  		return nil, &LOKError{Op: "Load", Detail: "documentLoad returned NULL"}
  	}
  	return realDocumentHandle{d: doc}, nil
  }

  func (realBackend) DocumentLoadWithOptions(h officeHandle, url, options string) (documentHandle, error) {
  	doc := lokc.DocumentLoadWithOptions(must(h).h, url, options)
  	if !doc.IsValid() {
  		return nil, &LOKError{Op: "Load", Detail: "documentLoadWithOptions returned NULL"}
  	}
  	return realDocumentHandle{d: doc}, nil
  }

  func (realBackend) DocumentGetType(d documentHandle) int {
  	return lokc.DocumentGetType(mustDoc(d).d)
  }

  func (realBackend) DocumentSaveAs(d documentHandle, url, format, filterOptions string) error {
  	return lokc.DocumentSaveAs(mustDoc(d).d, url, format, filterOptions)
  }

  func (realBackend) DocumentDestroy(d documentHandle) {
  	lokc.DocumentDestroy(mustDoc(d).d)
  }

  func mustDoc(d documentHandle) realDocumentHandle {
  	rd, ok := d.(realDocumentHandle)
  	if !ok {
  		panic("lok: document handle does not match real backend")
  	}
  	return rd
  }
  ```

### 2.2 Fake backend extension

- [ ] **Step 3: Extend `fakeBackend` in `lok/office_test.go`**

  Add fields:
  ```go
  	loadErr       error
  	saveErr       error
  	lastLoadURL   string
  	lastLoadOpts  string
  	lastSaveURL   string
  	lastSaveFmt   string
  	lastSaveOpts  string
  	docDestroys   int
  	docType       int // returned by DocumentGetType
  ```

  Add methods:
  ```go
  type fakeDoc struct {
  	be  *fakeBackend
  	url string
  }

  func (*fakeDoc) documentBrand() {}

  func (f *fakeBackend) DocumentLoad(_ officeHandle, url string) (documentHandle, error) {
  	if f.loadErr != nil {
  		return nil, f.loadErr
  	}
  	f.lastLoadURL = url
  	return &fakeDoc{be: f, url: url}, nil
  }

  func (f *fakeBackend) DocumentLoadWithOptions(_ officeHandle, url, opts string) (documentHandle, error) {
  	if f.loadErr != nil {
  		return nil, f.loadErr
  	}
  	f.lastLoadURL = url
  	f.lastLoadOpts = opts
  	return &fakeDoc{be: f, url: url}, nil
  }

  func (f *fakeBackend) DocumentGetType(documentHandle) int { return f.docType }

  func (f *fakeBackend) DocumentSaveAs(d documentHandle, url, format, opts string) error {
  	if f.saveErr != nil {
  		return f.saveErr
  	}
  	f.lastSaveURL = url
  	f.lastSaveFmt = format
  	f.lastSaveOpts = opts
  	return nil
  }

  func (f *fakeBackend) DocumentDestroy(documentHandle) {
  	f.mu.Lock()
  	defer f.mu.Unlock()
  	f.docDestroys++
  }
  ```

### 2.3 Document type

- [ ] **Step 4: Create `lok/document.go` (skeleton — methods land in Tasks 3-5)**

  ```go
  //go:build linux || darwin

  package lok

  import "sync"

  // DocumentType mirrors LOK_DOCTYPE_* from LibreOfficeKitEnums.h.
  type DocumentType int

  const (
  	TypeText         DocumentType = iota // LOK_DOCTYPE_TEXT
  	TypeSpreadsheet                      // LOK_DOCTYPE_SPREADSHEET
  	TypePresentation                     // LOK_DOCTYPE_PRESENTATION
  	TypeDrawing                          // LOK_DOCTYPE_DRAWING
  	TypeOther                            // LOK_DOCTYPE_OTHER
  )

  // String gives a human label for the type; unknown integers fall
  // back to "DocumentType(n)".
  func (t DocumentType) String() string {
  	switch t {
  	case TypeText:
  		return "Text"
  	case TypeSpreadsheet:
  		return "Spreadsheet"
  	case TypePresentation:
  		return "Presentation"
  	case TypeDrawing:
  		return "Drawing"
  	case TypeOther:
  		return "Other"
  	}
  	return fmt.Sprintf("DocumentType(%d)", int(t))
  }

  // Document is a single loaded document. A process may hold many at
  // once, but every method serialises on the parent Office's mutex
  // because LOK is not thread-safe.
  type Document struct {
  	office    *Office
  	h         documentHandle
  	origURL   string // cached for Save()
  	tempPath  string // non-empty when created by LoadFromReader
  	closeOnce sync.Once
  	closed    bool
  }

  // LoadOption configures Load / LoadFromReader.
  type LoadOption func(*loadOptions)

  type loadOptions struct {
  	password    string
  	readOnly    bool
  	lang        string
  	filterOpts  string // passed as the "options" parameter to documentLoadWithOptions
  }

  // WithPassword attaches a password for a password-protected document.
  // Wired through Office.SetDocumentPassword before the load call.
  func WithPassword(pw string) LoadOption {
  	return func(o *loadOptions) { o.password = pw }
  }

  // WithReadOnly opens the document read-only.
  func WithReadOnly() LoadOption {
  	return func(o *loadOptions) { o.readOnly = true }
  }

  // WithLanguage sets the document language tag (e.g. "en-US").
  func WithLanguage(lang string) LoadOption {
  	return func(o *loadOptions) { o.lang = lang }
  }

  // WithFilterOptions passes raw filter options through to
  // documentLoadWithOptions. Use for LOK-specific flags not covered
  // by the typed options above.
  func WithFilterOptions(opts string) LoadOption {
  	return func(o *loadOptions) { o.filterOpts = opts }
  }

  func buildLoadOptions(opts []LoadOption) loadOptions {
  	var lo loadOptions
  	for _, fn := range opts {
  		fn(&lo)
  	}
  	return lo
  }

  // Type returns the document's LOK type, or TypeOther on a closed doc.
  func (d *Document) Type() DocumentType {
  	d.office.mu.Lock()
  	defer d.office.mu.Unlock()
  	if d.closed {
  		return TypeOther
  	}
  	return DocumentType(d.office.be.DocumentGetType(d.h))
  }
  ```

  Note the `fmt` import for `String()`. Add it.

- [ ] **Step 5: Create minimal `lok/document_test.go` covering Type and the String method**

  ```go
  //go:build linux || darwin

  package lok

  import "testing"

  func TestDocumentType_String(t *testing.T) {
  	cases := []struct {
  		t    DocumentType
  		want string
  	}{
  		{TypeText, "Text"},
  		{TypeSpreadsheet, "Spreadsheet"},
  		{TypePresentation, "Presentation"},
  		{TypeDrawing, "Drawing"},
  		{TypeOther, "Other"},
  		{DocumentType(99), "DocumentType(99)"},
  	}
  	for _, tc := range cases {
  		if got := tc.t.String(); got != tc.want {
  			t.Errorf("%d: got %q, want %q", tc.t, got, tc.want)
  		}
  	}
  }
  ```

  The Type() method itself is covered indirectly in Task 3's
  `TestLoad_*` tests.

- [ ] **Step 6: Run + commit**

  Run: `go test -race ./lok/...` → PASS.

  ```bash
  git add lok/backend.go lok/real_backend.go lok/real_backend_test.go lok/office_test.go lok/document.go lok/document_test.go
  git commit -m "feat(lok): add Document type and backend seam for load/save

backend interface grows Document{Load,LoadWithOptions,GetType,
SaveAs,Destroy}; fakeBackend captures the arguments for test
assertions. Document type carries a reference to its parent Office
(for mutex sharing) and caches origURL for a future Save. Type(),
String(), and LoadOption functional-options (Password, ReadOnly,
Language, FilterOptions) land here; Load and Save are next
commits.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 3: `Office.Load` + `Document.Close` + `Document.Type` (TDD)

**Files:**
- Modify: `lok/document.go` (add `Load`, `Close`)
- Modify: `lok/document_test.go` (load/close/type tests)

### Step 1: Failing tests

Append to `lok/document_test.go`:

```go
func TestLoad_EmptyPathErrors(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, err := New("/install")
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	if _, err := o.Load(""); !errors.Is(err, ErrPathRequired) {
		t.Errorf("want ErrPathRequired, got %v", err)
	}
}

func TestLoad_PassesFileURL(t *testing.T) {
	fb := &fakeBackend{docType: int(TypeSpreadsheet)}
	withFakeBackend(t, fb)
	o, err := New("/install")
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	doc, err := o.Load("/tmp/hello.ods")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	t.Cleanup(func() { doc.Close() })

	if !strings.HasPrefix(fb.lastLoadURL, "file://") {
		t.Errorf("Load URL not file://-prefixed: %q", fb.lastLoadURL)
	}
	if !strings.HasSuffix(fb.lastLoadURL, "/tmp/hello.ods") {
		t.Errorf("Load URL tail wrong: %q", fb.lastLoadURL)
	}
	if doc.Type() != TypeSpreadsheet {
		t.Errorf("Type()=%v, want Spreadsheet", doc.Type())
	}
}

func TestLoad_WithOptions_UsesLoadWithOptions(t *testing.T) {
	fb := &fakeBackend{}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()

	doc, err := o.Load("/tmp/x.odt", WithPassword("hunter2"), WithReadOnly())
	if err != nil {
		t.Fatal(err)
	}
	defer doc.Close()
	if fb.lastPwdPassword != "hunter2" {
		t.Error("password not forwarded to SetDocumentPassword")
	}
	if fb.lastLoadOpts == "" {
		t.Error("LoadWithOptions not invoked for options case")
	}
}

func TestLoad_BackendError(t *testing.T) {
	errSynth := errors.New("synthetic")
	fb := &fakeBackend{loadErr: errSynth}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	_, err := o.Load("/tmp/x.odt")
	if !errors.Is(err, errSynth) {
		t.Errorf("want synthetic, got %v", err)
	}
}

func TestDocument_Close_Idempotent(t *testing.T) {
	fb := &fakeBackend{}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	if err := doc.Close(); err != nil {
		t.Fatal(err)
	}
	if err := doc.Close(); err != nil {
		t.Error(err)
	}
	if fb.docDestroys != 1 {
		t.Errorf("docDestroys: want 1, got %d", fb.docDestroys)
	}
}

func TestDocument_TypeAfterCloseReturnsOther(t *testing.T) {
	fb := &fakeBackend{docType: int(TypeText)}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	doc.Close()
	if got := doc.Type(); got != TypeOther {
		t.Errorf("Type after Close: got %v, want Other", got)
	}
}
```

Add an import for `strings` in the test file.

### Step 2: Run — red

Expect build errors for `Load`, `Close`, `ErrPathRequired`.

### Step 3: Implement

Add to `lok/errors.go`:
```go
var ErrPathRequired = errors.New("lok: document path is required")
```

Add to `lok/document.go`:
```go
import (
	"net/url"
	"path/filepath"
)

// Load opens a document from the filesystem. The path is converted
// to a file:// URL before being passed to LOK. Variadic options
// switch to documentLoadWithOptions when any option that needs the
// options string is present; otherwise documentLoad is used.
func (o *Office) Load(path string, opts ...LoadOption) (*Document, error) {
	if path == "" {
		return nil, ErrPathRequired
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return nil, ErrClosed
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, &LOKError{Op: "Load", Detail: err.Error(), err: err}
	}
	fileURL := (&url.URL{Scheme: "file", Path: abs}).String()

	lo := buildLoadOptions(opts)
	if lo.password != "" {
		o.be.OfficeSetDocumentPassword(o.h, fileURL, lo.password)
	}

	optsStr := composeLoadOptions(lo)
	var h documentHandle
	if optsStr != "" {
		h, err = o.be.DocumentLoadWithOptions(o.h, fileURL, optsStr)
	} else {
		h, err = o.be.DocumentLoad(o.h, fileURL)
	}
	if err != nil {
		return nil, err
	}

	doc := &Document{
		office:  o,
		h:       h,
		origURL: fileURL,
	}
	return doc, nil
}

// composeLoadOptions turns the typed LoadOption values into the raw
// options string LOK accepts at documentLoadWithOptions.
//
// Supported tokens:
//   - "ReadOnly=1"
//   - "Language=<tag>"
//   - raw filterOpts appended verbatim if set
//
// LOK accepts comma-separated key=value pairs. If no typed option is
// set and filterOpts is empty, the returned string is "" and the
// caller uses the simpler documentLoad instead.
func composeLoadOptions(lo loadOptions) string {
	var parts []string
	if lo.readOnly {
		parts = append(parts, "ReadOnly=1")
	}
	if lo.lang != "" {
		parts = append(parts, "Language="+lo.lang)
	}
	if lo.filterOpts != "" {
		parts = append(parts, lo.filterOpts)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ",")
}

// Close destroys the LOK document handle and cleans up any temp file
// created by LoadFromReader. Idempotent.
func (d *Document) Close() error {
	d.closeOnce.Do(func() {
		d.office.mu.Lock()
		defer d.office.mu.Unlock()
		d.office.be.DocumentDestroy(d.h)
		d.closed = true
		if d.tempPath != "" {
			_ = os.Remove(d.tempPath)
		}
	})
	return nil
}
```

Add the `os` and `strings` imports as needed.

### Step 4: Run + commit

Run: `go test -race ./lok/...` → PASS.

```bash
git add lok/document.go lok/document_test.go lok/errors.go
git commit -m "feat(lok): Office.Load + Document.Close + Type

Load converts the caller's filesystem path to a file:// URL via
filepath.Abs + net/url, serialises on the Office mutex, wires
WithPassword through SetDocumentPassword, and dispatches to
documentLoadWithOptions when any option that needs the options
string is set. Close is idempotent via sync.Once, destroys the
LOK handle, and removes the LoadFromReader temp file (none in this
commit). Type returns TypeOther after Close rather than panicking.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: `Document.SaveAs` + `Save` (TDD)

**Files:**
- Modify: `lok/document.go`
- Modify: `lok/document_test.go`

### Step 1: Failing tests

```go
func TestSaveAs_PassesFileURL(t *testing.T) {
	fb := &fakeBackend{}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()

	if err := doc.SaveAs("/tmp/x.pdf", "pdf", "SkipImages=1"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(fb.lastSaveURL, "/tmp/x.pdf") {
		t.Errorf("SaveAs URL tail: %q", fb.lastSaveURL)
	}
	if fb.lastSaveFmt != "pdf" {
		t.Errorf("SaveAs format: %q", fb.lastSaveFmt)
	}
	if fb.lastSaveOpts != "SkipImages=1" {
		t.Errorf("SaveAs opts: %q", fb.lastSaveOpts)
	}
}

func TestSaveAs_EmptyPathErrors(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()
	if err := doc.SaveAs("", "", ""); !errors.Is(err, ErrPathRequired) {
		t.Errorf("want ErrPathRequired, got %v", err)
	}
}

func TestSaveAs_BackendError(t *testing.T) {
	fb := &fakeBackend{saveErr: errors.New("synthetic save")}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()
	if err := doc.SaveAs("/tmp/x.pdf", "pdf", ""); err == nil {
		t.Fatal("expected backend error to surface")
	}
}

func TestSaveAs_AfterCloseErrors(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	doc.Close()
	if err := doc.SaveAs("/tmp/x.pdf", "pdf", ""); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestSave_ReusesOrigURL(t *testing.T) {
	fb := &fakeBackend{}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()
	if err := doc.Save(); err != nil {
		t.Fatal(err)
	}
	if fb.lastSaveURL != doc.origURL {
		t.Errorf("Save: URL=%q, want %q", fb.lastSaveURL, doc.origURL)
	}
	if fb.lastSaveFmt != "" {
		t.Errorf("Save: format should be empty, got %q", fb.lastSaveFmt)
	}
}
```

### Step 2: Implement

```go
// SaveAs writes the document to path in the given LO format (e.g.
// "odt", "pdf", "docx", "png"). filterOpts passes extra
// filter-specific tokens verbatim (e.g. "SkipImages=1"). An empty
// path returns ErrPathRequired; calls on a closed document return
// ErrClosed.
func (d *Document) SaveAs(path, format, filterOpts string) error {
	if path == "" {
		return ErrPathRequired
	}
	d.office.mu.Lock()
	defer d.office.mu.Unlock()
	if d.closed {
		return ErrClosed
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return &LOKError{Op: "SaveAs", Detail: err.Error(), err: err}
	}
	fileURL := (&url.URL{Scheme: "file", Path: abs}).String()
	if err := d.office.be.DocumentSaveAs(d.h, fileURL, format, filterOpts); err != nil {
		return &LOKError{Op: "SaveAs", Detail: err.Error(), err: err}
	}
	return nil
}

// Save re-saves the document to its original URL. LOK has no
// dedicated save() vtable entry, so Save is implemented as saveAs
// to origURL with no format/filter changes.
func (d *Document) Save() error {
	d.office.mu.Lock()
	origURL := d.origURL
	closed := d.closed
	d.office.mu.Unlock()
	if closed {
		return ErrClosed
	}
	// Re-lock inside SaveAs logic by calling through the backend
	// directly. Keep origURL conversion out — it's already a file://
	// URL from Load.
	d.office.mu.Lock()
	defer d.office.mu.Unlock()
	if d.closed {
		return ErrClosed
	}
	if err := d.office.be.DocumentSaveAs(d.h, origURL, "", ""); err != nil {
		return &LOKError{Op: "Save", Detail: err.Error(), err: err}
	}
	return nil
}
```

### Step 3: Commit

```bash
git add lok/document.go lok/document_test.go
git commit -m "feat(lok): Document.SaveAs + Save

SaveAs converts path → file:// URL and forwards format/filterOpts
to the backend; empty path → ErrPathRequired; post-Close → ErrClosed;
backend errors are wrapped as *LOKError. Save re-saves to the
document's cached origURL with empty format/opts because LOK has
no dedicated save() vtable entry in the 24.8 header.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 5: `Office.LoadFromReader` (TDD)

**Files:**
- Modify: `lok/document.go`
- Modify: `lok/document_test.go`

### Step 1: Failing tests

```go
func TestLoadFromReader_WritesTempFileAndCleansUp(t *testing.T) {
	fb := &fakeBackend{}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()

	content := []byte("PK fake ODT bytes")
	doc, err := o.LoadFromReader(bytes.NewReader(content), "odt")
	if err != nil {
		t.Fatal(err)
	}
	tempPath := doc.tempPath
	if tempPath == "" {
		t.Fatal("tempPath not set")
	}
	if _, err := os.Stat(tempPath); err != nil {
		t.Errorf("temp file not present: %v", err)
	}
	if err := doc.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Errorf("temp file not removed: %v", err)
	}
}

func TestLoadFromReader_EmptyFilterFallsBackToDefault(t *testing.T) {
	fb := &fakeBackend{}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	doc, err := o.LoadFromReader(bytes.NewReader([]byte("x")), "")
	if err != nil {
		t.Fatal(err)
	}
	defer doc.Close()
	// Empty filter → no filename extension hint. Implementation
	// should still produce a working temp path.
	if doc.tempPath == "" {
		t.Error("tempPath not set")
	}
}

func TestLoadFromReader_ReaderError(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	_, err := o.LoadFromReader(iotest.ErrReader(errors.New("boom")), "odt")
	if err == nil {
		t.Fatal("expected reader error to surface")
	}
}
```

Add imports: `bytes`, `io`, `os`, `testing/iotest`.

### Step 2: Implement

```go
// LoadFromReader streams r into a temp file under os.TempDir, then
// calls Load on that path. filter is used as the temp file's
// extension when non-empty (so LO auto-detects the format); pass
// "" to let LO guess from content. The temp file is removed in
// Document.Close.
//
// Accepts the same LoadOption values as Load.
func (o *Office) LoadFromReader(r io.Reader, filter string, opts ...LoadOption) (*Document, error) {
	suffix := ""
	if filter != "" {
		suffix = "." + filter
	}
	f, err := os.CreateTemp("", "lokdoc-*"+suffix)
	if err != nil {
		return nil, &LOKError{Op: "LoadFromReader", Detail: err.Error(), err: err}
	}
	tempPath := f.Name()
	if _, err := io.Copy(f, r); err != nil {
		_ = f.Close()
		_ = os.Remove(tempPath)
		return nil, &LOKError{Op: "LoadFromReader", Detail: err.Error(), err: err}
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tempPath)
		return nil, &LOKError{Op: "LoadFromReader", Detail: err.Error(), err: err}
	}
	doc, err := o.Load(tempPath, opts...)
	if err != nil {
		_ = os.Remove(tempPath)
		return nil, err
	}
	doc.tempPath = tempPath
	return doc, nil
}
```

### Step 3: Commit

```bash
git add lok/document.go lok/document_test.go
git commit -m "feat(lok): Office.LoadFromReader (temp-file streaming)

LoadFromReader streams the reader to os.CreateTemp under TempDir
with an optional filter-based extension so LO auto-detects format,
loads that path via Load, and records the temp path on the
returned Document so Close deletes it. Errors from CreateTemp,
io.Copy, and the inner Load are wrapped as *LOKError / propagate.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 6: Integration tests against real LO + fixture

**Files:**
- Create: `testdata/hello.odt`
- Modify: `lok/integration_test.go`

### Step 1: Commit a tiny ODT fixture

Produce a minimal ODT locally (one-time) with:
```bash
echo "Hello from Phase 3." | soffice --headless --convert-to odt --outdir testdata /dev/stdin
mv testdata/stdin.odt testdata/hello.odt
```
Verify ≤ 4 KB:
```bash
wc -c testdata/hello.odt
```

Commit as binary.

### Step 2: Extend `TestIntegration_FullLifecycle`

All document tests share the same Office (per the singleton-per-process
rule — see the file's header note):

```go
	// --- Phase 3 document round-trip ---

	fixture, err := filepath.Abs("../testdata/hello.odt")
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	doc, err := o.Load(fixture)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	t.Cleanup(func() { doc.Close() })

	if got := doc.Type(); got != TypeText {
		t.Errorf("Type()=%v, want Text", got)
	}

	outPath := filepath.Join(t.TempDir(), "roundtrip.odt")
	if err := doc.SaveAs(outPath, "odt", ""); err != nil {
		t.Errorf("SaveAs: %v", err)
	}
	if st, err := os.Stat(outPath); err != nil {
		t.Errorf("SaveAs output missing: %v", err)
	} else if st.Size() == 0 {
		t.Error("SaveAs output is zero bytes")
	}

	pdfPath := filepath.Join(t.TempDir(), "out.pdf")
	if err := doc.SaveAs(pdfPath, "pdf", ""); err != nil {
		t.Errorf("SaveAs pdf: %v", err)
	}

	// LoadFromReader round-trip on the same Office.
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	doc2, err := o.LoadFromReader(bytes.NewReader(data), "odt")
	if err != nil {
		t.Fatalf("LoadFromReader: %v", err)
	}
	defer doc2.Close()
	if got := doc2.Type(); got != TypeText {
		t.Errorf("reader-loaded Type()=%v, want Text", got)
	}
```

Add imports: `bytes`, `os`, `path/filepath`.

### Step 3: Run against real LO

Run: `LOK_PATH=/usr/lib64/libreoffice/program make test-integration`
Expected: `TestIntegration_FullLifecycle` passes end-to-end.

### Step 4: Commit

```bash
git add testdata/hello.odt lok/integration_test.go
git commit -m "test(lok): integration tests for document load/save/type

Commits a tiny ODT fixture (testdata/hello.odt) and extends
TestIntegration_FullLifecycle with document subtests: Load →
Type → SaveAs (odt + pdf) → LoadFromReader. Everything shares
the single Office per process.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 7: Final verification

- [ ] **Step 1: Full workflow**

  ```bash
  make clean
  make all
  make cover-gate
  make test-integration
  LOK_PATH=/usr/lib64/libreoffice/program make test-integration
  ```
  Expected: every command exits 0. Gate at ≥ 90% across both packages.

- [ ] **Step 2: Branch topology**

  `git log --oneline main..HEAD` — 6 commits above main.

- [ ] **Step 3: Open the PR (with user confirmation)**

  ```bash
  git push -u origin chore/document-load-save
  gh pr create --base main --title "Phase 3: Document load/save + type" --body "..."
  ```

---

## Acceptance criteria (matches spec §Phase 3)

- [ ] `Office.Load(path, opts...)` returns `*Document` or
      `ErrPathRequired`/`ErrClosed`/backend error.
- [ ] `Office.LoadFromReader(r, filter, opts...)` streams to temp,
      loads, cleans up on `Close`.
- [ ] `Document.Type()` returns the LOK document type.
- [ ] `Document.Save()` re-saves to origURL (implemented via
      `saveAs` since LOK has no `save` vtable entry).
- [ ] `Document.SaveAs(path, format, filterOpts)` produces a
      non-empty output file.
- [ ] `Document.Close()` idempotent; cleans up temp files.
- [ ] Integration test round-trips the ODT fixture.
- [ ] `make cover-gate` ≥ 90% on `internal/lokc` + `lok`.
- [ ] Nothing from Phase 4+ (Views, Parts, Rendering) sneaks in.

When every box is ticked, `chore/document-load-save` is ready to merge;
Phase 4's plan (Views) can begin.
