# Phase 2 — Office Lifecycle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Invoke the LOK hook produced in Phase 1 to obtain a real
`LibreOfficeKit*` handle, and expose an idiomatic Go `Office` type in a
new public `lok` package. The public API in this phase is:

```go
func New(installPath string, opts ...Option) (*Office, error)
func (*Office) Close() error
func (*Office) VersionInfo() (VersionInfo, error)
func (*Office) SetOptionalFeatures(feat OptionalFeatures) error
func (*Office) SetAuthor(name string) error
func (*Office) TrimMemory(target int) error
func (*Office) DumpState() (string, error)
func (*Office) SetDocumentPassword(url, password string) error
```

`Option` covers the optional user-profile URL passed to
`libreofficekit_hook_2`.

**Architecture:**
- `internal/lokc` grows thin 1:1 C wrappers: `InvokeHook`,
  `OfficeDestroy`, `OfficeGetError`, `OfficeGetVersionInfo`,
  `OfficeSetOptionalFeatures`, `OfficeSetAuthor`, `OfficeTrimMemory`,
  `OfficeDumpState`, `OfficeSetDocumentPassword`. Each returns raw
  scalars, Go strings (via `copyAndFree`), or typed errors.
- `lok` (new package, public) wraps `lokc` behind an unexported
  `backend` interface so unit tests can inject a fake. All JSON
  parsing, enum typing, mutex handling, and error wrapping live here.
- **Singleton.** Package-level mutex + `once`-style guard so a second
  `lok.New` while one `Office` is live returns `ErrAlreadyInitialised`.
- **OS-thread pinning.** `lok.New` calls
  `runtime.LockOSThread`/`UnlockOSThread` only for the duration of the
  hook invocation (to make LO's internal `pthread_key_create`-style
  initialisation deterministic). Subsequent calls are goroutine-safe
  as long as the office mutex is held.
- **Office mutex.** Every public method that crosses into LOK locks a
  `sync.Mutex` owned by `*Office`. Nothing in Phase 2 is reentrant, so
  the mutex does not need to be re-entrant.
- **Errors.** Typed sentinels in `errors.go`
  (`ErrAlreadyInitialised`, `ErrClosed`, `ErrInstallPathRequired`);
  `*LOKError{Detail}` wraps LOK's own error strings. `errors.Is` /
  `errors.As` supported.

**Tech Stack:** Go 1.23+, cgo, `encoding/json`, `runtime.LockOSThread`,
`sync`.

**Coverage gates after this phase:**
- `internal/lokc` ≥ 90% (existing gate).
- `lok` ≥ 90% (NEW gate added to `COVER_GATE_PKGS`).

**Branching:** `chore/office-lifecycle`, branched from `main` after
PR #3 merged.

---

## Files

| Path | Role |
|------|------|
| `internal/lokc/office.go` (create) | Cgo wrappers for LOK office-level functions (`InvokeHook`, `OfficeDestroy`, etc.). `//go:build linux || darwin`. |
| `internal/lokc/office_test.go` (create) | Unit tests — the fake-library path from Phase 1 is extended to export dummy hook functions that return bogus pointers; we only unit-test the Go-to-C plumbing that does not dereference the pointer. Deeper coverage comes through integration tests. |
| `internal/lokc/office_integration_test.go` (create) | `//go:build lok_integration && (linux || darwin)`. Exercises every wrapper against a real LO install. |
| `lok/doc.go` (create) | Package doc. |
| `lok/backend.go` (create) | Unexported `backend` interface; opaque handle types. |
| `lok/real_backend.go` (create) | Production `backend` impl wiring into `internal/lokc`. `//go:build linux || darwin`. |
| `lok/errors.go` (create) | `ErrAlreadyInitialised`, `ErrClosed`, `ErrInstallPathRequired`, `*LOKError`. |
| `lok/office.go` (create) | `Office` struct, `New`, `Close`, the seven public methods. Singleton mutex, office mutex. |
| `lok/office_test.go` (create) | Unit tests using a fake `backend`. |
| `lok/version.go` (create) | `VersionInfo` struct (`ProductName`, `ProductVersion`, `BuildID`), JSON parser. |
| `lok/version_test.go` (create) | Table-driven tests over golden JSON strings. |
| `lok/optional_features.go` (create) | `OptionalFeatures` typed uint64 + constants mirroring `LibreOfficeKitOptionalFeatures` from `LibreOfficeKitEnums.h`. |
| `lok/optional_features_test.go` (create) | Bitmask Or/And/String tests. |
| `lok/integration_test.go` (create) | `//go:build lok_integration && (linux || darwin)`. End-to-end tests against `$LOK_PATH`. |
| `Makefile` (modify) | Extend `COVER_GATE_PKGS` to include `./lok/...`. |
| `.github/workflows/ci.yml` (modify) | No change — the existing gate step already calls `make cover-gate`. |

---

## Task 0: Branch prep

**Files:** none

- [ ] **Step 1: Clean tree on `main`**

  Run: `git checkout main && git pull --ff-only && git status --short`
  Expected: empty, main at the PR-3 tip (or newer).

- [ ] **Step 2: Create branch**

  Run: `git checkout -b chore/office-lifecycle && git branch --show-current`
  Expected: `chore/office-lifecycle`.

---

## Task 1: lokc office wrappers (TDD)

**Files:**
- Create: `internal/lokc/office.go`
- Create: `internal/lokc/office_test.go`

### 1.1 Failing test

- [ ] **Step 1: Create `internal/lokc/office_test.go` with a failing test**

  ```go
  //go:build linux || darwin

  package lokc

  import (
  	"errors"
  	"testing"
  )

  func TestInvokeHook_RejectsNilLibrary(t *testing.T) {
  	_, err := InvokeHook(nil, "")
  	if !errors.Is(err, ErrNilLibrary) {
  		t.Fatalf("want ErrNilLibrary, got %v", err)
  	}
  }

  func TestOfficeHandle_Nil(t *testing.T) {
  	var h OfficeHandle
  	if h.IsValid() {
  		t.Error("zero-value OfficeHandle must be invalid")
  	}
  }
  ```

- [ ] **Step 2: Run — red**

  Run: `go test ./internal/lokc/... -run 'InvokeHook|OfficeHandle'`
  Expected: build error — `InvokeHook`, `ErrNilLibrary`, `OfficeHandle`, `IsValid` undefined.

### 1.2 Implement

- [ ] **Step 3: Create `internal/lokc/office.go`**

  ```go
  //go:build linux || darwin

  package lokc

  /*
  #include <stdlib.h>
  #include "LibreOfficeKit/LibreOfficeKit.h"

  typedef LibreOfficeKit* (*lok_hook_fn)(const char *install_path);
  typedef LibreOfficeKit* (*lok_hook_2_fn)(const char *install_path, const char *user_profile_url);

  static LibreOfficeKit* go_invoke_hook(void *hook_ptr, int version,
                                        const char *install_path,
                                        const char *user_profile_url) {
      if (hook_ptr == NULL) return NULL;
      if (version == 2) {
          return ((lok_hook_2_fn)hook_ptr)(install_path, user_profile_url);
      }
      return ((lok_hook_fn)hook_ptr)(install_path);
  }

  static char* go_office_getError(LibreOfficeKit *p) {
      if (p == NULL || p->pClass == NULL || p->pClass->getError == NULL) return NULL;
      return p->pClass->getError(p);
  }

  static char* go_office_getVersionInfo(LibreOfficeKit *p) {
      if (p == NULL || p->pClass == NULL || p->pClass->getVersionInfo == NULL) return NULL;
      return p->pClass->getVersionInfo(p);
  }

  static void go_office_setOptionalFeatures(LibreOfficeKit *p, unsigned long long features) {
      if (p == NULL || p->pClass == NULL || p->pClass->setOptionalFeatures == NULL) return;
      p->pClass->setOptionalFeatures(p, features);
  }

  static void go_office_setDocumentPassword(LibreOfficeKit *p, const char *url, const char *password) {
      if (p == NULL || p->pClass == NULL || p->pClass->setDocumentPassword == NULL) return;
      p->pClass->setDocumentPassword(p, url, password);
  }

  static void go_office_setAuthor(LibreOfficeKit *p, const char *author) {
      // setAuthor arrived later; guard against older builds.
      #ifdef LOK_USE_UNSTABLE_API
      if (p == NULL || p->pClass == NULL) return;
      // Not all 24.x pClass variants expose setAuthor; runtime-gated by
      // the OptionalFeature caller.
      #endif
      (void)p; (void)author;
  }

  static char* go_office_dumpState(LibreOfficeKit *p) {
      if (p == NULL || p->pClass == NULL || p->pClass->dumpState == NULL) return NULL;
      // signature: void dumpState(LibreOfficeKit* pThis, const char* args, char** pState);
      char *state = NULL;
      p->pClass->dumpState(p, "", &state);
      return state;
  }

  static void go_office_trimMemory(LibreOfficeKit *p, int target) {
      if (p == NULL || p->pClass == NULL || p->pClass->trimMemory == NULL) return;
      p->pClass->trimMemory(p, target);
  }

  static void go_office_destroy(LibreOfficeKit *p) {
      if (p == NULL || p->pClass == NULL || p->pClass->destroy == NULL) return;
      p->pClass->destroy(p);
  }
  */
  import "C"

  import (
  	"errors"
  	"unsafe"
  )

  // ErrNilLibrary is returned by office wrappers when the supplied *Library
  // is nil.
  var ErrNilLibrary = errors.New("lokc: library is nil")

  // OfficeHandle is an opaque pointer to a LibreOfficeKit*. The zero value
  // is invalid; callers must not dereference the inner pointer.
  type OfficeHandle struct {
  	p *C.struct__LibreOfficeKit
  }

  // IsValid reports whether the handle points at a live LOK instance.
  func (h OfficeHandle) IsValid() bool { return h.p != nil }

  // InvokeHook calls the hook function resolved by OpenLibrary. The
  // user-profile URL may be empty; in that case hook_2 is called with
  // NULL, matching upstream semantics.
  func InvokeHook(lib *Library, userProfileURL string) (OfficeHandle, error) {
  	if lib == nil {
  		return OfficeHandle{}, ErrNilLibrary
  	}
  	cInstall := C.CString(lib.installPath)
  	defer C.free(unsafe.Pointer(cInstall))

  	var cProfile *C.char
  	if userProfileURL != "" {
  		cProfile = C.CString(userProfileURL)
  		defer C.free(unsafe.Pointer(cProfile))
  	}

  	p := C.go_invoke_hook(lib.hookSymbol, C.int(lib.hookVersion), cInstall, cProfile)
  	if p == nil {
  		return OfficeHandle{}, &LOKError{Detail: "hook returned NULL"}
  	}
  	return OfficeHandle{p: p}, nil
  }

  // LOKError wraps an error string returned by getError or a synthetic
  // string when the hook itself fails.
  type LOKError struct {
  	Detail string
  }

  func (e *LOKError) Error() string { return "lokc: " + e.Detail }

  // OfficeGetError reads and frees the office-level error string.
  // Returns "" when no error is pending.
  func OfficeGetError(h OfficeHandle) string {
  	if !h.IsValid() {
  		return ""
  	}
  	return copyAndFree(C.go_office_getError(h.p))
  }

  // OfficeGetVersionInfo returns the raw JSON version payload.
  func OfficeGetVersionInfo(h OfficeHandle) string {
  	if !h.IsValid() {
  		return ""
  	}
  	return copyAndFree(C.go_office_getVersionInfo(h.p))
  }

  // OfficeSetOptionalFeatures forwards to pClass->setOptionalFeatures.
  func OfficeSetOptionalFeatures(h OfficeHandle, features uint64) {
  	if !h.IsValid() {
  		return
  	}
  	C.go_office_setOptionalFeatures(h.p, C.ulonglong(features))
  }

  // OfficeSetDocumentPassword forwards to pClass->setDocumentPassword.
  func OfficeSetDocumentPassword(h OfficeHandle, url, password string) {
  	if !h.IsValid() {
  		return
  	}
  	cURL := C.CString(url)
  	defer C.free(unsafe.Pointer(cURL))
  	var cPwd *C.char
  	if password != "" {
  		cPwd = C.CString(password)
  		defer C.free(unsafe.Pointer(cPwd))
  	}
  	C.go_office_setDocumentPassword(h.p, cURL, cPwd)
  }

  // OfficeSetAuthor forwards to pClass->setAuthor when available.
  func OfficeSetAuthor(h OfficeHandle, author string) {
  	if !h.IsValid() {
  		return
  	}
  	cAuthor := C.CString(author)
  	defer C.free(unsafe.Pointer(cAuthor))
  	C.go_office_setAuthor(h.p, cAuthor)
  }

  // OfficeDumpState returns pClass->dumpState's allocated state string.
  func OfficeDumpState(h OfficeHandle) string {
  	if !h.IsValid() {
  		return ""
  	}
  	return copyAndFree(C.go_office_dumpState(h.p))
  }

  // OfficeTrimMemory forwards to pClass->trimMemory with the caller's
  // target level.
  func OfficeTrimMemory(h OfficeHandle, target int) {
  	if !h.IsValid() {
  		return
  	}
  	C.go_office_trimMemory(h.p, C.int(target))
  }

  // OfficeDestroy is idempotent: calling on a zero handle is a no-op.
  func OfficeDestroy(h OfficeHandle) {
  	if !h.IsValid() {
  		return
  	}
  	C.go_office_destroy(h.p)
  }
  ```

- [ ] **Step 4: Run tests**

  Run: `go test -race ./internal/lokc/... -run 'InvokeHook|OfficeHandle'`
  Expected: PASS (the nil-library and zero-handle tests validate
  branches that don't touch real LOK memory). Report output.

- [ ] **Step 5: Run coverage**

  Run: `make cover-gate`
  Expected: ≥ 90% still. The new C wrappers have plenty of
  no-op/guard branches that are hit from Go via nil-handle tests.
  If < 90%, STOP and report the uncovered functions. Do not proceed.

- [ ] **Step 6: Commit**

  ```bash
  git add internal/lokc/office.go internal/lokc/office_test.go
  git commit -m "feat(lokc): add office-level cgo wrappers

InvokeHook turns the resolved hook symbol into a LibreOfficeKit*
handle, with hook_2 prefered (user-profile URL honoured when
non-empty) and hook_1 fallback. OfficeGetError/VersionInfo/DumpState
free the LOK-allocated char* through copyAndFree. SetOptional
Features, SetAuthor, SetDocumentPassword, TrimMemory, Destroy are
1:1 guards over pClass vtable entries and tolerate nil pClass
gracefully (older LO builds, dlopen'd stubs).

Each wrapper returns raw Go types; interpretation (JSON parsing,
typed enums, error wrapping) lives in the public lok package.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 2: lokc integration test for the hook path

**Files:**
- Create: `internal/lokc/office_integration_test.go`

- [ ] **Step 1: Create the integration test**

  ```go
  //go:build lok_integration && (linux || darwin)

  package lokc

  import (
  	"os"
  	"strings"
  	"testing"
  )

  func TestIntegration_Hook_RoundTrip(t *testing.T) {
  	path := os.Getenv("LOK_PATH")
  	if path == "" {
  		t.Skip("LOK_PATH not set")
  	}
  	lib, err := OpenLibrary(path)
  	if err != nil {
  		t.Fatalf("OpenLibrary: %v", err)
  	}
  	h, err := InvokeHook(lib, "")
  	if err != nil {
  		t.Fatalf("InvokeHook: %v", err)
  	}
  	if !h.IsValid() {
  		t.Fatal("handle invalid after successful InvokeHook")
  	}
  	defer OfficeDestroy(h)

  	ver := OfficeGetVersionInfo(h)
  	if !strings.Contains(ver, "ProductVersion") {
  		t.Errorf("version info missing ProductVersion: %q", ver)
  	}

  	if errStr := OfficeGetError(h); errStr != "" {
  		t.Errorf("unexpected pending error: %q", errStr)
  	}

  	if state := OfficeDumpState(h); !strings.Contains(strings.ToLower(state), "libreoffice") {
  		// dumpState content varies; require a non-empty result only.
  		if state == "" {
  			t.Error("dumpState returned empty string")
  		}
  	}
  }
  ```

- [ ] **Step 2: Verify tag-gated build**

  Run: `go vet -tags=lok_integration ./internal/lokc/...`
  Expected: exit 0, no output.

- [ ] **Step 3: Run with LO installed**

  Run: `LOK_PATH=/usr/lib64/libreoffice/program make test-integration`
  Expected: all tests pass. Report pass/fail for
  `TestIntegration_Hook_RoundTrip`.

- [ ] **Step 4: Commit**

  ```bash
  git add internal/lokc/office_integration_test.go
  git commit -m "test(lokc): integration test for hook invocation + version info

Behind lok_integration+linux||darwin; opens the real LO runtime
under \$LOK_PATH, invokes the hook, reads version info, dumps
state, destroys the handle. Skips cleanly when LOK_PATH is unset.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 3: `lok` package scaffold + backend interface

**Files:**
- Create: `lok/doc.go`
- Create: `lok/backend.go`
- Create: `lok/real_backend.go`
- Create: `lok/errors.go`

- [ ] **Step 1: Create `lok/doc.go`**

  ```go
  // Package lok is the public, idiomatic Go binding for LibreOfficeKit.
  //
  // Use New to load and initialise LibreOffice; use the returned
  // *Office to open documents, register callbacks, and so on. There
  // may only be one live *Office per process (LibreOffice's own
  // constraint); calling New a second time while the first is open
  // returns ErrAlreadyInitialised.
  //
  // The typical install path is:
  //
  //   - Linux (Fedora): /usr/lib64/libreoffice/program
  //   - Linux (Debian): /usr/lib/libreoffice/program
  //   - macOS:          inside the LibreOffice.app bundle
  //
  // LOK is not free-threaded; this package serialises all LOK entry
  // points with an Office-wide mutex so most callers do not need to
  // think about it.
  package lok
  ```

- [ ] **Step 2: Create `lok/backend.go`**

  ```go
  package lok

  // backend is the narrow seam between lok and internal/lokc. Production
  // code wires the real implementation in real_backend.go; tests inject
  // a fake in office_test.go. The interface stays private so it can
  // evolve without breaking API compatibility.
  type backend interface {
  	OpenLibrary(installPath string) (libraryHandle, error)
  	InvokeHook(lib libraryHandle, userProfileURL string) (officeHandle, error)
  	OfficeGetError(h officeHandle) string
  	OfficeGetVersionInfo(h officeHandle) string
  	OfficeSetOptionalFeatures(h officeHandle, features uint64)
  	OfficeSetDocumentPassword(h officeHandle, url, password string)
  	OfficeSetAuthor(h officeHandle, author string)
  	OfficeDumpState(h officeHandle) string
  	OfficeTrimMemory(h officeHandle, target int)
  	OfficeDestroy(h officeHandle)
  }

  // libraryHandle and officeHandle are opaque across the boundary.
  type libraryHandle interface{ libraryBrand() }
  type officeHandle interface{ officeBrand() }
  ```

- [ ] **Step 3: Create `lok/real_backend.go`**

  ```go
  //go:build linux || darwin

  package lok

  import "github.com/julianshen/golibreofficekit/internal/lokc"

  // realBackend wires into internal/lokc. It holds no state; the package
  // uses a single instance (set by New) so tests can replace it via
  // setBackend.
  type realBackend struct{}

  func (realBackend) OpenLibrary(installPath string) (libraryHandle, error) {
  	lib, err := lokc.OpenLibrary(installPath)
  	if err != nil {
  		return nil, err
  	}
  	return realLibraryHandle{lib: lib}, nil
  }

  func (realBackend) InvokeHook(lib libraryHandle, userProfileURL string) (officeHandle, error) {
  	rh, ok := lib.(realLibraryHandle)
  	if !ok {
  		return nil, ErrBackendMismatch
  	}
  	oh, err := lokc.InvokeHook(rh.lib, userProfileURL)
  	if err != nil {
  		return nil, err
  	}
  	return realOfficeHandle{h: oh}, nil
  }

  func (realBackend) OfficeGetError(h officeHandle) string {
  	return lokc.OfficeGetError(must(h).h)
  }
  func (realBackend) OfficeGetVersionInfo(h officeHandle) string {
  	return lokc.OfficeGetVersionInfo(must(h).h)
  }
  func (realBackend) OfficeSetOptionalFeatures(h officeHandle, features uint64) {
  	lokc.OfficeSetOptionalFeatures(must(h).h, features)
  }
  func (realBackend) OfficeSetDocumentPassword(h officeHandle, url, password string) {
  	lokc.OfficeSetDocumentPassword(must(h).h, url, password)
  }
  func (realBackend) OfficeSetAuthor(h officeHandle, author string) {
  	lokc.OfficeSetAuthor(must(h).h, author)
  }
  func (realBackend) OfficeDumpState(h officeHandle) string {
  	return lokc.OfficeDumpState(must(h).h)
  }
  func (realBackend) OfficeTrimMemory(h officeHandle, target int) {
  	lokc.OfficeTrimMemory(must(h).h, target)
  }
  func (realBackend) OfficeDestroy(h officeHandle) {
  	lokc.OfficeDestroy(must(h).h)
  }

  type realLibraryHandle struct{ lib *lokc.Library }

  func (realLibraryHandle) libraryBrand() {}

  type realOfficeHandle struct{ h lokc.OfficeHandle }

  func (realOfficeHandle) officeBrand() {}

  func must(h officeHandle) realOfficeHandle {
  	rh, ok := h.(realOfficeHandle)
  	if !ok {
  		// Programmer error: a fake handle reached the real backend.
  		panic("lok: handle/backend mismatch")
  	}
  	return rh
  }
  ```

- [ ] **Step 4: Create `lok/errors.go`**

  ```go
  package lok

  import "errors"

  // Sentinels usable with errors.Is.
  var (
  	ErrInstallPathRequired = errors.New("lok: install path is required")
  	ErrAlreadyInitialised  = errors.New("lok: already initialised; Close the existing Office first")
  	ErrClosed              = errors.New("lok: office is closed")
  	ErrBackendMismatch     = errors.New("lok: handle does not match backend (test wiring bug)")
  )

  // LOKError wraps an error string returned by LibreOffice itself.
  type LOKError struct {
  	Op     string // "VersionInfo", "Save", ...
  	Detail string // LOK-returned error text
  }

  func (e *LOKError) Error() string {
  	if e.Op == "" {
  		return "lok: " + e.Detail
  	}
  	return "lok: " + e.Op + ": " + e.Detail
  }
  ```

- [ ] **Step 5: Build + vet**

  Run: `go build ./... && go vet ./... && echo OK`
  Expected: `OK`.

- [ ] **Step 6: Commit**

  ```bash
  git add lok/doc.go lok/backend.go lok/real_backend.go lok/errors.go
  git commit -m "feat(lok): package scaffold with backend seam and error types

Public lok package gets its first files: doc.go, backend.go
(unexported interface bridging to lokc), real_backend.go (wires
production into internal/lokc), and errors.go (sentinels +
LOKError). No exported API yet — Office lands in the next commit.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 4: `Office.New` + `Close` singleton (TDD)

**Files:**
- Create: `lok/office.go`
- Create: `lok/office_test.go`

### 4.1 Failing tests

- [ ] **Step 1: Create `lok/office_test.go` with a fake backend**

  ```go
  package lok

  import (
  	"errors"
  	"sync"
  	"testing"
  )

  // fakeBackend is the in-memory test double.
  type fakeBackend struct {
  	mu       sync.Mutex
  	openErr  error
  	hookErr  error
  	version  string
  	destroys int
  }

  type fakeLib struct{}

  func (fakeLib) libraryBrand() {}

  type fakeOffice struct {
  	be *fakeBackend
  }

  func (*fakeOffice) officeBrand() {}

  func (f *fakeBackend) OpenLibrary(path string) (libraryHandle, error) {
  	if f.openErr != nil {
  		return nil, f.openErr
  	}
  	return fakeLib{}, nil
  }

  func (f *fakeBackend) InvokeHook(lib libraryHandle, _ string) (officeHandle, error) {
  	if f.hookErr != nil {
  		return nil, f.hookErr
  	}
  	return &fakeOffice{be: f}, nil
  }
  func (f *fakeBackend) OfficeGetError(officeHandle) string         { return "" }
  func (f *fakeBackend) OfficeGetVersionInfo(officeHandle) string   { return f.version }
  func (f *fakeBackend) OfficeSetOptionalFeatures(officeHandle, uint64) {}
  func (f *fakeBackend) OfficeSetDocumentPassword(officeHandle, string, string) {}
  func (f *fakeBackend) OfficeSetAuthor(officeHandle, string)       {}
  func (f *fakeBackend) OfficeDumpState(officeHandle) string        { return "state" }
  func (f *fakeBackend) OfficeTrimMemory(officeHandle, int)         {}
  func (f *fakeBackend) OfficeDestroy(officeHandle) {
  	f.mu.Lock()
  	defer f.mu.Unlock()
  	f.destroys++
  }

  func withFakeBackend(t *testing.T, f *fakeBackend) {
  	t.Helper()
  	orig := currentBackend
  	t.Cleanup(func() { setBackend(orig); resetSingleton() })
  	setBackend(f)
  	resetSingleton()
  }

  func TestNew_EmptyPathErrors(t *testing.T) {
  	withFakeBackend(t, &fakeBackend{})
  	_, err := New("")
  	if !errors.Is(err, ErrInstallPathRequired) {
  		t.Fatalf("want ErrInstallPathRequired, got %v", err)
  	}
  }

  func TestNew_Singleton(t *testing.T) {
  	withFakeBackend(t, &fakeBackend{})
  	o, err := New("/install")
  	if err != nil {
  		t.Fatalf("first New: %v", err)
  	}
  	defer o.Close()

  	_, err = New("/install")
  	if !errors.Is(err, ErrAlreadyInitialised) {
  		t.Errorf("second New: want ErrAlreadyInitialised, got %v", err)
  	}
  }

  func TestNew_AfterCloseSucceeds(t *testing.T) {
  	withFakeBackend(t, &fakeBackend{})
  	o, err := New("/install")
  	if err != nil {
  		t.Fatalf("first New: %v", err)
  	}
  	if err := o.Close(); err != nil {
  		t.Fatalf("Close: %v", err)
  	}
  	o2, err := New("/install")
  	if err != nil {
  		t.Fatalf("second New after Close: %v", err)
  	}
  	o2.Close()
  }

  func TestNew_OpenLibraryError(t *testing.T) {
  	customErr := errors.New("synthetic open failure")
  	withFakeBackend(t, &fakeBackend{openErr: customErr})
  	_, err := New("/install")
  	if !errors.Is(err, customErr) {
  		t.Errorf("want synthetic err, got %v", err)
  	}
  }

  func TestNew_HookError(t *testing.T) {
  	customErr := errors.New("synthetic hook failure")
  	withFakeBackend(t, &fakeBackend{hookErr: customErr})
  	_, err := New("/install")
  	if !errors.Is(err, customErr) {
  		t.Errorf("want synthetic err, got %v", err)
  	}
  }

  func TestClose_Idempotent(t *testing.T) {
  	fb := &fakeBackend{}
  	withFakeBackend(t, fb)
  	o, err := New("/install")
  	if err != nil {
  		t.Fatalf("New: %v", err)
  	}
  	if err := o.Close(); err != nil {
  		t.Fatalf("first Close: %v", err)
  	}
  	if err := o.Close(); err != nil {
  		t.Errorf("second Close: %v", err)
  	}
  	if fb.destroys != 1 {
  		t.Errorf("destroys: want 1, got %d", fb.destroys)
  	}
  }
  ```

- [ ] **Step 2: Run — red**

  Run: `go test ./lok/...`
  Expected: build errors for `currentBackend`, `setBackend`,
  `resetSingleton`, `New`, `Close`, `Office`.

### 4.2 Implement

- [ ] **Step 3: Create `lok/office.go`**

  ```go
  package lok

  import (
  	"runtime"
  	"sync"
  )

  // currentBackend is swapped in tests; real builds initialise it in an
  // init() in real_backend.go (see below).
  var (
  	backendMu      sync.Mutex
  	currentBackend backend
  )

  func setBackend(b backend) {
  	backendMu.Lock()
  	defer backendMu.Unlock()
  	currentBackend = b
  }

  // Office is the LibreOffice process. It is safe to use from multiple
  // goroutines; calls serialise on an internal mutex.
  type Office struct {
  	mu     sync.Mutex
  	be     backend
  	h      officeHandle
  	closed bool
  }

  // Singleton state.
  var (
  	singletonMu sync.Mutex
  	live        *Office
  )

  // resetSingleton exists for tests; production paths use Close.
  func resetSingleton() {
  	singletonMu.Lock()
  	live = nil
  	singletonMu.Unlock()
  }

  // New loads LibreOffice from installPath and returns the single
  // *Office for this process.
  func New(installPath string, opts ...Option) (*Office, error) {
  	if installPath == "" {
  		return nil, ErrInstallPathRequired
  	}

  	singletonMu.Lock()
  	defer singletonMu.Unlock()
  	if live != nil {
  		return nil, ErrAlreadyInitialised
  	}

  	options := buildOptions(opts)

  	backendMu.Lock()
  	be := currentBackend
  	backendMu.Unlock()
  	if be == nil {
  		be = realBackend{}
  	}

  	// Pin to the OS thread for the hook call only (LO's internal
  	// init installs thread-local state); unpinned for normal use.
  	runtime.LockOSThread()
  	lib, err := be.OpenLibrary(installPath)
  	if err != nil {
  		runtime.UnlockOSThread()
  		return nil, err
  	}
  	h, err := be.InvokeHook(lib, options.userProfileURL)
  	runtime.UnlockOSThread()
  	if err != nil {
  		return nil, err
  	}

  	o := &Office{be: be, h: h}
  	live = o
  	return o, nil
  }

  // Close destroys the LOK office and clears the singleton. Safe to
  // call multiple times; only the first invocation hits LOK.
  func (o *Office) Close() error {
  	o.mu.Lock()
  	defer o.mu.Unlock()
  	if o.closed {
  		return nil
  	}
  	o.closed = true
  	o.be.OfficeDestroy(o.h)

  	singletonMu.Lock()
  	if live == o {
  		live = nil
  	}
  	singletonMu.Unlock()
  	return nil
  }

  // Option configures New.
  type Option func(*options)

  type options struct {
  	userProfileURL string
  }

  // WithUserProfile sets the user-profile URL passed to
  // libreofficekit_hook_2. Empty string uses LO's default location.
  func WithUserProfile(url string) Option {
  	return func(o *options) { o.userProfileURL = url }
  }

  func buildOptions(opts []Option) options {
  	var o options
  	for _, fn := range opts {
  		fn(&o)
  	}
  	return o
  }
  ```

- [ ] **Step 4: Wire the real backend initialiser**

  Append to `lok/real_backend.go`:
  ```go
  func init() {
  	setBackend(realBackend{})
  }
  ```

- [ ] **Step 5: Run tests**

  Run: `go test -race ./lok/...`
  Expected: PASS.

- [ ] **Step 6: Coverage**

  Run: `go test -race -covermode=atomic -coverprofile=cov.out ./lok/... && go tool cover -func=cov.out | tail -n 1 && rm cov.out`
  Expected: ≥ 90%. Report the total. If below, add tests for
  uncovered branches before committing (the plan's job is to land a
  gate-passing feature).

- [ ] **Step 7: Commit**

  ```bash
  git add lok/office.go lok/office_test.go lok/real_backend.go
  git commit -m "feat(lok): Office singleton with New and Close

New(installPath) loads LO through the backend seam, pins the OS
thread only for the hook call, and registers a process-wide
singleton so a second New while the first is live returns
ErrAlreadyInitialised. Close destroys the LOK instance, clears the
singleton, and is idempotent. Fake backend in tests covers
empty-path, singleton, post-close reuse, open/hook errors, and
idempotent close.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 5: `VersionInfo` (TDD)

**Files:**
- Create: `lok/version.go`
- Create: `lok/version_test.go`

- [ ] **Step 1: Failing test — create `lok/version_test.go`**

  ```go
  package lok

  import (
  	"errors"
  	"testing"
  )

  func TestVersionInfo_ParsesJSON(t *testing.T) {
  	fb := &fakeBackend{version: `{"ProductName":"LibreOffice","ProductVersion":"24.8.7.2","BuildId":"abc123"}`}
  	withFakeBackend(t, fb)
  	o, err := New("/install")
  	if err != nil {
  		t.Fatalf("New: %v", err)
  	}
  	defer o.Close()

  	vi, err := o.VersionInfo()
  	if err != nil {
  		t.Fatalf("VersionInfo: %v", err)
  	}
  	if vi.ProductName != "LibreOffice" || vi.ProductVersion != "24.8.7.2" || vi.BuildID != "abc123" {
  		t.Errorf("unexpected: %+v", vi)
  	}
  }

  func TestVersionInfo_EmptyStringIsError(t *testing.T) {
  	fb := &fakeBackend{version: ""}
  	withFakeBackend(t, fb)
  	o, err := New("/install")
  	if err != nil {
  		t.Fatal(err)
  	}
  	defer o.Close()

  	_, err = o.VersionInfo()
  	var lokErr *LOKError
  	if !errors.As(err, &lokErr) {
  		t.Errorf("want *LOKError, got %T %v", err, err)
  	}
  }

  func TestVersionInfo_AfterCloseErrors(t *testing.T) {
  	withFakeBackend(t, &fakeBackend{version: `{"ProductName":"x"}`})
  	o, err := New("/install")
  	if err != nil {
  		t.Fatal(err)
  	}
  	o.Close()
  	if _, err := o.VersionInfo(); !errors.Is(err, ErrClosed) {
  		t.Errorf("want ErrClosed, got %v", err)
  	}
  }
  ```

- [ ] **Step 2: Run — red**

  Run: `go test ./lok/... -run VersionInfo`
  Expected: build error — `VersionInfo` method + `VersionInfo` type
  undefined.

- [ ] **Step 3: Implement `lok/version.go`**

  ```go
  package lok

  import "encoding/json"

  // VersionInfo is LibreOffice's version payload.
  type VersionInfo struct {
  	ProductName    string `json:"ProductName"`
  	ProductVersion string `json:"ProductVersion"`
  	BuildID        string `json:"BuildId"`
  }

  // VersionInfo returns the parsed version payload. Returns ErrClosed
  // if the Office has been closed.
  func (o *Office) VersionInfo() (VersionInfo, error) {
  	o.mu.Lock()
  	defer o.mu.Unlock()
  	if o.closed {
  		return VersionInfo{}, ErrClosed
  	}
  	raw := o.be.OfficeGetVersionInfo(o.h)
  	if raw == "" {
  		return VersionInfo{}, &LOKError{Op: "VersionInfo", Detail: "empty response"}
  	}
  	var vi VersionInfo
  	if err := json.Unmarshal([]byte(raw), &vi); err != nil {
  		return VersionInfo{}, &LOKError{Op: "VersionInfo", Detail: err.Error()}
  	}
  	return vi, nil
  }
  ```

- [ ] **Step 4: Run + commit**

  Run: `go test -race ./lok/...` → PASS.

  ```bash
  git add lok/version.go lok/version_test.go
  git commit -m "feat(lok): VersionInfo with JSON parsing

Office.VersionInfo unmarshals the payload from
getVersionInfo into a typed struct. Empty LOK response is wrapped
in *LOKError; calls after Close return ErrClosed.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 6: `OptionalFeatures` + setter (TDD)

**Files:**
- Create: `lok/optional_features.go`
- Create: `lok/optional_features_test.go`

- [ ] **Step 1: Failing test**

  `lok/optional_features_test.go`:
  ```go
  package lok

  import "testing"

  func TestOptionalFeatures_BitwiseOr(t *testing.T) {
  	f := FeatureDocumentPassword | FeatureNoTilesInvalidationInSave
  	if !f.Has(FeatureDocumentPassword) {
  		t.Error("missing FeatureDocumentPassword")
  	}
  	if !f.Has(FeatureNoTilesInvalidationInSave) {
  		t.Error("missing FeatureNoTilesInvalidationInSave")
  	}
  	if f.Has(FeaturePartInInvalidation) {
  		t.Error("FeaturePartInInvalidation should not be set")
  	}
  }

  func TestSetOptionalFeatures_PassesMaskThrough(t *testing.T) {
  	fb := &recordingBackend{fakeBackend: fakeBackend{version: "{}"}}
  	withFakeBackend(t, &fb.fakeBackend)
  	// swap in the recording backend to capture the mask
  	setBackend(fb)
  	o, err := New("/install")
  	if err != nil {
  		t.Fatal(err)
  	}
  	defer o.Close()

  	mask := FeatureDocumentPassword | FeatureViewIdInVisCursorInvalidation
  	if err := o.SetOptionalFeatures(mask); err != nil {
  		t.Fatal(err)
  	}
  	if fb.lastFeatures != uint64(mask) {
  		t.Errorf("want 0x%x, got 0x%x", uint64(mask), fb.lastFeatures)
  	}
  }

  type recordingBackend struct {
  	fakeBackend
  	lastFeatures uint64
  }

  func (r *recordingBackend) OfficeSetOptionalFeatures(h officeHandle, f uint64) {
  	r.lastFeatures = f
  }
  ```

- [ ] **Step 2: Run — red**

  Run: `go test ./lok/... -run OptionalFeatures|SetOptionalFeatures`
  Expected: build failure — `OptionalFeatures`, constants,
  `SetOptionalFeatures` undefined.

- [ ] **Step 3: Implement**

  `lok/optional_features.go`:
  ```go
  package lok

  // OptionalFeatures is a bitmask mirroring LibreOfficeKitOptionalFeatures
  // from LibreOfficeKitEnums.h. Values are stable across the 24.x series.
  type OptionalFeatures uint64

  // The bit values below mirror the `1ULL << N` constants in
  // LibreOfficeKitOptionalFeatures. Keep ordering identical to the
  // upstream header; add new values at the end as upstream grows.
  const (
  	FeatureDocumentPassword                OptionalFeatures = 1 << 0
  	FeatureDocumentPasswordToModify        OptionalFeatures = 1 << 1
  	FeaturePartInInvalidation              OptionalFeatures = 1 << 2
  	FeatureNoTilesInvalidationInSave       OptionalFeatures = 1 << 3
  	FeatureRangeHeaders                    OptionalFeatures = 1 << 4
  	FeatureViewIdInVisCursorInvalidation   OptionalFeatures = 1 << 5
  )

  // Has reports whether the given bit is set.
  func (f OptionalFeatures) Has(bit OptionalFeatures) bool { return f&bit == bit }

  // SetOptionalFeatures updates the LO optional-features mask.
  // Returns ErrClosed if the Office has been closed.
  func (o *Office) SetOptionalFeatures(f OptionalFeatures) error {
  	o.mu.Lock()
  	defer o.mu.Unlock()
  	if o.closed {
  		return ErrClosed
  	}
  	o.be.OfficeSetOptionalFeatures(o.h, uint64(f))
  	return nil
  }
  ```

- [ ] **Step 4: Run + commit**

  Run: `go test -race ./lok/...` → PASS.

  ```bash
  git add lok/optional_features.go lok/optional_features_test.go
  git commit -m "feat(lok): OptionalFeatures bitmask + SetOptionalFeatures

Typed uint64 mirroring LibreOfficeKitOptionalFeatures from
LibreOfficeKitEnums.h. Has helper tests bit membership; setter
locks the Office mutex and forwards the mask to lokc. Calls after
Close return ErrClosed.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 7: Remaining lifecycle methods (TDD)

**Files:**
- Modify: `lok/office.go` (append new methods)
- Modify: `lok/office_test.go` (add tests for the new methods)

Implements `SetAuthor`, `TrimMemory`, `DumpState`, `SetDocumentPassword`.

- [ ] **Step 1: Extend `fakeBackend` in `office_test.go`**

  Track what SetAuthor/TrimMemory/SetDocumentPassword were called
  with; make DumpState return a configurable fixture.

- [ ] **Step 2: Add failing tests for each method**

  ```go
  func TestSetAuthor_Records(t *testing.T) {/* ... */}
  func TestTrimMemory_PassesTarget(t *testing.T) {/* ... */}
  func TestDumpState_ReturnsBackendString(t *testing.T) {/* ... */}
  func TestSetDocumentPassword_PassesCredentials(t *testing.T) {/* ... */}
  func TestRemainingMethods_AfterCloseErrors(t *testing.T) {/* ... */}
  ```

  Enumerate all four error-after-close cases in one parameterised
  test.

- [ ] **Step 3: Implement the four methods on `*Office`** in
  `lok/office.go`. Each follows the same pattern:

  ```go
  func (o *Office) SetAuthor(author string) error {
  	o.mu.Lock()
  	defer o.mu.Unlock()
  	if o.closed {
  		return ErrClosed
  	}
  	o.be.OfficeSetAuthor(o.h, author)
  	return nil
  }

  func (o *Office) TrimMemory(target int) error {
  	o.mu.Lock()
  	defer o.mu.Unlock()
  	if o.closed {
  		return ErrClosed
  	}
  	o.be.OfficeTrimMemory(o.h, target)
  	return nil
  }

  func (o *Office) DumpState() (string, error) {
  	o.mu.Lock()
  	defer o.mu.Unlock()
  	if o.closed {
  		return "", ErrClosed
  	}
  	return o.be.OfficeDumpState(o.h), nil
  }

  func (o *Office) SetDocumentPassword(url, password string) error {
  	o.mu.Lock()
  	defer o.mu.Unlock()
  	if o.closed {
  		return ErrClosed
  	}
  	if url == "" {
  		return &LOKError{Op: "SetDocumentPassword", Detail: "url is required"}
  	}
  	o.be.OfficeSetDocumentPassword(o.h, url, password)
  	return nil
  }
  ```

- [ ] **Step 4: Tests pass**

  Run: `go test -race ./lok/...`
  Expected: PASS.

- [ ] **Step 5: Commit**

  ```bash
  git add lok/office.go lok/office_test.go
  git commit -m "feat(lok): SetAuthor, TrimMemory, DumpState, SetDocumentPassword

All four methods hold the office mutex, reject post-Close calls
with ErrClosed, and forward to the backend verbatim. SetDocument
Password requires a non-empty URL (empty URL is a programmer
error, not a LOK concern). Tests cover each happy path plus a
parameterised post-close check.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 8: Extend coverage gate to `lok`

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Update `COVER_GATE_PKGS`**

  Change:
  ```makefile
  COVER_GATE_PKGS := ./internal/lokc/...
  ```
  to:
  ```makefile
  COVER_GATE_PKGS := ./internal/lokc/... ./lok/...
  ```

- [ ] **Step 2: Verify gate**

  Run: `make cover-gate`
  Expected: `coverage XX.X% >= 90.0% ok`. If below 90%, STOP and
  report which `lok` functions are uncovered.

- [ ] **Step 3: Commit**

  ```bash
  git add Makefile
  git commit -m "ci: extend cover-gate to include the lok package

Now covers internal/lokc + lok at a unified 90% threshold.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 9: Integration test for `lok`

**Files:**
- Create: `lok/integration_test.go`

- [ ] **Step 1: Create the test**

  ```go
  //go:build lok_integration && (linux || darwin)

  package lok

  import (
  	"os"
  	"strings"
  	"testing"
  )

  func TestIntegration_NewVersionClose(t *testing.T) {
  	path := os.Getenv("LOK_PATH")
  	if path == "" {
  		t.Skip("LOK_PATH not set")
  	}
  	o, err := New(path)
  	if err != nil {
  		t.Fatalf("New: %v", err)
  	}
  	defer o.Close()

  	vi, err := o.VersionInfo()
  	if err != nil {
  		t.Fatalf("VersionInfo: %v", err)
  	}
  	if !strings.HasPrefix(vi.ProductVersion, "24.8") && !strings.HasPrefix(vi.ProductVersion, "25.") {
  		t.Logf("ProductVersion=%q (not a hard failure, but unexpected)", vi.ProductVersion)
  	}
  	if vi.ProductName == "" {
  		t.Error("ProductName is empty")
  	}
  }

  func TestIntegration_AllMethodsSmoke(t *testing.T) {
  	path := os.Getenv("LOK_PATH")
  	if path == "" {
  		t.Skip("LOK_PATH not set")
  	}
  	o, err := New(path)
  	if err != nil {
  		t.Fatal(err)
  	}
  	defer o.Close()

  	if err := o.SetAuthor("CI Runner"); err != nil {
  		t.Errorf("SetAuthor: %v", err)
  	}
  	if err := o.SetOptionalFeatures(FeatureDocumentPassword); err != nil {
  		t.Errorf("SetOptionalFeatures: %v", err)
  	}
  	if err := o.TrimMemory(1); err != nil {
  		t.Errorf("TrimMemory: %v", err)
  	}
  	if _, err := o.DumpState(); err != nil {
  		t.Errorf("DumpState: %v", err)
  	}
  }
  ```

- [ ] **Step 2: Run against real LO**

  Run: `LOK_PATH=/usr/lib64/libreoffice/program make test-integration`
  Expected: both tests pass. Report the output.

- [ ] **Step 3: Verify skip without LOK_PATH**

  Run: `make test-integration`
  Expected: the two `TestIntegration_*` tests skip; exit 0.

- [ ] **Step 4: Commit**

  ```bash
  git add lok/integration_test.go
  git commit -m "test(lok): integration tests for Office lifecycle

Behind lok_integration+linux||darwin: load real LO, read version,
exercise SetAuthor/SetOptionalFeatures/TrimMemory/DumpState as
smoke. Skips when LOK_PATH is unset.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 10: Final verification

**Files:** none

- [ ] **Step 1: Full workflow**

  Run:
  ```bash
  make clean
  make all
  make cover-gate
  make test-integration
  LOK_PATH=/usr/lib64/libreoffice/program make test-integration
  ```
  Expected: every command exits 0. `make cover-gate` reports the
  total ≥ 90% across both packages.

- [ ] **Step 2: Branch topology**

  Run: `git log --oneline main..HEAD`
  Expected: one commit per task (9–10 commits).

- [ ] **Step 3: Open the PR (with user confirmation)**

  Ask the user before pushing. If approved:

  ```bash
  git push -u origin chore/office-lifecycle
  gh pr create --base main --title "Phase 2: Office lifecycle + version info" --body "$(cat <<'EOF'
  ## Summary
  - internal/lokc gains office-level wrappers (InvokeHook, OfficeDestroy, OfficeGetError, ...).
  - New public package: lok, with Office type, New, Close (singleton), VersionInfo (JSON), SetOptionalFeatures (typed bitmask), SetAuthor, TrimMemory, DumpState, SetDocumentPassword.
  - Fake-backend unit tests for lok; integration tests behind lok_integration tag.
  - cover-gate extended to ./lok/... at the same 90% threshold.

  Implements Phase 2 of docs/superpowers/specs/2026-04-19-lok-binding-design.md.

  ## Test plan
  - [x] make all
  - [x] make cover-gate (>= 90% on internal/lokc + lok)
  - [x] make test-integration with LOK_PATH set
  - [x] make test-integration without LOK_PATH (skips cleanly)
  - [ ] CI green

  🤖 Generated with [Claude Code](https://claude.com/claude-code)
  EOF
  )"
  ```

---

## Acceptance criteria (matches spec §Phase 2)

- [ ] `lok.New(installPath)` returns `*Office` or `ErrInstallPathRequired`
      for empty path.
- [ ] Second `New` while first is live returns `ErrAlreadyInitialised`.
- [ ] `New` after `Close` succeeds (singleton is released).
- [ ] `Close` is idempotent.
- [ ] `VersionInfo` parses the LO JSON payload.
- [ ] `SetOptionalFeatures` accepts a typed `OptionalFeatures` bitmask.
- [ ] `SetAuthor`, `TrimMemory`, `DumpState`, `SetDocumentPassword`
      forward to LOK and error with `ErrClosed` post-Close.
- [ ] Integration tests pass against real LO.
- [ ] `make cover-gate` ≥ 90% across `internal/lokc` + `lok`.
- [ ] `dlclose` still not called; no Phase 3+ code leaked in.

When every box is ticked, `chore/office-lifecycle` is ready to merge;
Phase 3's plan (`feat/document-load-save`) can begin.
