# Phase 1 — dlopen Loader Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Vendor the LibreOfficeKit 24.8 C headers and implement the
runtime-dlopen loader in `internal/lokc`: open `libsofficeapp.so` from
a caller-supplied install path, resolve `libreofficekit_hook_2` (with
`libreofficekit_hook` as a fallback), and hold the resulting function
pointer on a typed Go value. Invoking the hook to produce a
`LibreOfficeKit*` is deferred to Phase 2.

**Architecture:** Headers vendored under `third_party/lok/` with a
pinned `VERSION` file. `internal/lokc` uses **one** cgo file,
`dlopen_unix.go` (`//go:build linux || darwin`), that holds the cgo
preamble AND the dlopen/dlsym Go wrappers — a single translation
unit, one `import "C"`. `loader.go` is pure Go and composes those
wrappers into an `OpenLibrary` facade. A separate `errstr.go` holds
the copy-and-free helper used from Phase 2 onward (it is a cgo file
because it needs `C.free`). Unit tests exercise error paths using
`dlopen(NULL)` (the main-program handle) on both Linux and macOS,
plus a trivially-compiled fake `.so`/`.dylib` built via the system
`cc`; the integration test (`-tags=lok_integration`) opens a real
LibreOffice install.

**Tech Stack:** Go 1.23+, cgo, `dlopen`/`dlsym` via `<dlfcn.h>`,
vendored upstream headers, `cc` (for building the test fixture .so).

**Coverage gate activates for `internal/lokc`:** from this phase
onward, Makefile `cover-gate` target fails if coverage of
`internal/lokc` falls below 90%. `lok` package still does not exist
and remains ungated until Phase 2.

**Branching:** `chore/dlopen-loader`, branched from `main` after PR #1
merged.

---

## Files

| Path | Role |
|------|------|
| `third_party/lok/LibreOfficeKit/LibreOfficeKit.h` | Vendored from LO 24.8 `include/LibreOfficeKit/` |
| `third_party/lok/LibreOfficeKit/LibreOfficeKitEnums.h` | Vendored |
| `third_party/lok/LibreOfficeKit/LibreOfficeKitInit.h` | Vendored (we use our own dlopen; this header is vendored for Phase 2) |
| `third_party/lok/LICENSE` | MPL-2.0 upstream notice |
| `third_party/lok/VERSION` | Pinned LO release tag (`libreoffice-24.8.7.2`) |
| `internal/lokc/doc.go` | Package comment |
| `internal/lokc/dlopen_unix.go` | Single cgo file: preamble + Go wrappers (`dlOpen`, `dlSym`) returning `unsafe.Pointer` + typed `DLError`; `//go:build linux || darwin` |
| `internal/lokc/dlopen_test.go` | Unit tests for the wrappers |
| `internal/lokc/loader.go` | Public `Library` type, `OpenLibrary(installPath string)`, `openWithPath` (testable, symbols parameterised) |
| `internal/lokc/loader_test.go` | Unit tests using `cc`-compiled fake `.so` |
| `internal/lokc/errstr.go` | `copyAndFree(cs *C.char) string` |
| `internal/lokc/errstr_test.go` | Round-trip test |
| `internal/lokc/loader_integration_test.go` | `//go:build lok_integration`; opens `$LOK_PATH` |
| `Makefile` | Add `cover-gate` target enforcing ≥90% on `internal/lokc` |
| `.github/workflows/ci.yml` | Call `make cover-gate` in the unit-test job |

Existing files unchanged except the Makefile and CI workflow
additions.

---

## Task 0: Branch prep

**Files:** none

- [ ] **Step 1: Confirm clean tree on `main`**

  Run: `git checkout main && git pull --ff-only && git status --short`
  Expected: empty output, branch `main` is at the post-PR-1 tip.

- [ ] **Step 2: Create `chore/dlopen-loader`**

  Run: `git checkout -b chore/dlopen-loader && git branch --show-current`
  Expected: `chore/dlopen-loader`.

---

## Task 1: Vendor LOK headers

**Files:**
- Create: `third_party/lok/LibreOfficeKit/LibreOfficeKit.h`
- Create: `third_party/lok/LibreOfficeKit/LibreOfficeKitEnums.h`
- Create: `third_party/lok/LibreOfficeKit/LibreOfficeKitInit.h`
- Create: `third_party/lok/LICENSE`
- Create: `third_party/lok/VERSION`

- [ ] **Step 1: Create directory**

  Run: `mkdir -p third_party/lok/LibreOfficeKit && test -d third_party/lok/LibreOfficeKit && echo OK`
  Expected: `OK`.

- [ ] **Step 2: Fetch the three headers from the LO 24.8.7.2 tag**

  Run:
  ```bash
  TAG=libreoffice-24.8.7.2
  BASE=https://raw.githubusercontent.com/LibreOffice/core/$TAG/include/LibreOfficeKit
  for f in LibreOfficeKit.h LibreOfficeKitEnums.h LibreOfficeKitInit.h; do
    curl -fsSL "$BASE/$f" -o "third_party/lok/LibreOfficeKit/$f"
    echo "fetched $f ($(wc -l < third_party/lok/LibreOfficeKit/$f) lines)"
  done
  ```
  Expected: three `fetched <name> (N lines)` messages with N > 0.

  If the tag does not exist (repo moved, release differs), STOP and
  report the HTTP error. Do not paper over with a different tag.

- [ ] **Step 3: Write `third_party/lok/VERSION`**

  Contents:
  ```
  libreoffice-24.8.7.2
  ```
  (One line, no trailing text beyond the newline.)

- [ ] **Step 4: Write `third_party/lok/LICENSE`**

  LO core is MPL-2.0. Contents — the standard MPL-2.0 short notice:
  ```
  LibreOfficeKit headers vendored from the LibreOffice core project
  (https://github.com/LibreOffice/core), tag libreoffice-24.8.7.2.

  Licensed under the Mozilla Public License, v. 2.0. A copy of the
  MPL-2.0 is available at https://mozilla.org/MPL/2.0/. See the header
  files themselves for per-file attribution.
  ```

- [ ] **Step 5: Verify build is still green**

  Run: `make all`
  Expected: exits 0 (the vendored `.h` files do not affect the Go
  build yet).

- [ ] **Step 6: Commit**

  ```bash
  git add third_party/lok
  git commit -m "chore(lok): vendor LibreOfficeKit 24.8.7.2 headers

Fetched LibreOfficeKit.h, LibreOfficeKitEnums.h, LibreOfficeKitInit.h
from libreoffice/core tag libreoffice-24.8.7.2. Pinned under
third_party/lok/VERSION. LICENSE notes upstream MPL-2.0; per-file
copyright stays in the headers.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 2: cgo preamble + dlopen/dlsym wrappers (TDD)

**Files:**
- Create: `internal/lokc/doc.go`
- Create: `internal/lokc/dlopen_unix.go` (single cgo file)
- Create: `internal/lokc/dlopen_test.go`

### 2.1 Failing test first

- [ ] **Step 1: Create `internal/lokc/doc.go`**

  ```go
  // Package lokc is the thin cgo layer beneath the public lok package.
  //
  // It wraps libc dlopen/dlsym so callers can load the LibreOfficeKit
  // runtime at process start. The package is internal; the public,
  // idiomatic API lives under the lok package (added in Phase 2).
  package lokc
  ```

- [ ] **Step 2: Create `internal/lokc/dlopen_test.go` with a failing test**

  ```go
  package lokc

  import (
  	"errors"
  	"testing"
  )

  func TestDLOpen_MissingFileReturnsError(t *testing.T) {
  	_, err := dlOpen("/this/path/does/not/exist.so")
  	if err == nil {
  		t.Fatal("expected error for missing .so, got nil")
  	}
  	var dlerr *DLError
  	if !errors.As(err, &dlerr) {
  		t.Fatalf("expected *DLError, got %T (%v)", err, err)
  	}
  	if dlerr.Op != "dlopen" {
  		t.Errorf("Op: want %q, got %q", "dlopen", dlerr.Op)
  	}
  }
  ```

- [ ] **Step 3: Run the test and see it fail**

  Run: `go test ./internal/lokc/...`
  Expected: build failure — `dlOpen` and `DLError` are undefined. That
  is the red state.

### 2.2 Make it pass

- [ ] **Step 4: Create `internal/lokc/dlopen_unix.go` (single cgo file)**

  ```go
  //go:build linux || darwin

  package lokc

  /*
  #cgo LDFLAGS: -ldl
  #include <dlfcn.h>
  #include <stdlib.h>

  // Wrapper so Go can call dlopen without preprocessor macros.
  static void *go_dlopen(const char *path, int flag) {
  	// An empty Go string arrives here as a zero-length C string. Pass
  	// NULL instead so the main-program handle is returned on both
  	// Linux and macOS (macOS dlopen("") returns NULL+error).
  	if (path != NULL && path[0] == '\0') {
  		return dlopen(NULL, flag);
  	}
  	return dlopen(path, flag);
  }

  static void *go_dlsym(void *handle, const char *name) {
  	// Clear any pending error first so a genuine NULL return is
  	// disambiguated from a stale dlerror.
  	(void)dlerror();
  	return dlsym(handle, name);
  }

  static char *go_dlerror(void) {
  	return dlerror();
  }
  */
  import "C"

  import (
  	"fmt"
  	"unsafe"
  )

  // DLError is returned when dlopen or dlsym fails.
  type DLError struct {
  	Op     string // "dlopen" or "dlsym"
  	Target string // path for dlopen, symbol name for dlsym
  	Detail string // dlerror() output
  }

  func (e *DLError) Error() string {
  	return fmt.Sprintf("%s %q: %s", e.Op, e.Target, e.Detail)
  }

  // dlOpen resolves a shared library path via dlopen(RTLD_LAZY|RTLD_LOCAL).
  // An empty path opens the main-program handle (portable across Linux
  // and macOS via the NULL-translation in go_dlopen).
  //
  // Callers must not dlclose: LibreOffice's static init cannot be re-run
  // cleanly within the same process.
  func dlOpen(path string) (unsafe.Pointer, error) {
  	cpath := C.CString(path)
  	defer C.free(unsafe.Pointer(cpath))

  	handle := C.go_dlopen(cpath, C.RTLD_LAZY|C.RTLD_LOCAL)
  	if handle == nil {
  		return nil, &DLError{Op: "dlopen", Target: path, Detail: lastDLError()}
  	}
  	return unsafe.Pointer(handle), nil
  }

  // dlSym resolves a symbol in a handle obtained from dlOpen.
  func dlSym(handle unsafe.Pointer, symbol string) (unsafe.Pointer, error) {
  	if handle == nil {
  		return nil, &DLError{Op: "dlsym", Target: symbol, Detail: "handle is nil"}
  	}
  	csym := C.CString(symbol)
  	defer C.free(unsafe.Pointer(csym))

  	ptr := C.go_dlsym(handle, csym)
  	if ptr == nil {
  		return nil, &DLError{Op: "dlsym", Target: symbol, Detail: lastDLError()}
  	}
  	return unsafe.Pointer(ptr), nil
  }

  func lastDLError() string {
  	cs := C.go_dlerror()
  	if cs == nil {
  		return "(no dlerror)"
  	}
  	return C.GoString(cs)
  }
  ```

  Everything lives in one cgo file: one preamble, one `import "C"`.
  `loader.go` (Task 3) is pure Go.

- [ ] **Step 5: Run the failing test; expect it to pass now**

  Run: `go test -race ./internal/lokc/...`
  Expected: `PASS`, exit 0.

### 2.3 Extend coverage to success paths

- [ ] **Step 6: Add success-path tests**

  Append to `dlopen_test.go`:

  ```go
  func TestDLOpen_EmptyPathOpensMainProgram(t *testing.T) {
  	// dlOpen("") translates to dlopen(NULL), which returns a handle
  	// to the main program on both Linux and macOS; libc symbols are
  	// resolvable through it.
  	handle, err := dlOpen("")
  	if err != nil {
  		t.Fatalf("dlOpen(\"\"): %v", err)
  	}
  	if handle == nil {
  		t.Fatal("handle is nil")
  	}
  }

  func TestDLSym_FindsMalloc(t *testing.T) {
  	handle, err := dlOpen("")
  	if err != nil {
  		t.Skip("cannot open self:", err)
  	}
  	p, err := dlSym(handle, "malloc")
  	if err != nil {
  		t.Fatalf("dlsym malloc: %v", err)
  	}
  	if p == nil {
  		t.Fatal("malloc resolved to nil")
  	}
  }

  func TestDLSym_NilHandleErrors(t *testing.T) {
  	_, err := dlSym(nil, "malloc")
  	if err == nil {
  		t.Fatal("expected error for nil handle")
  	}
  	var dlerr *DLError
  	if !errors.As(err, &dlerr) || dlerr.Op != "dlsym" {
  		t.Errorf("want *DLError Op=dlsym, got %T %v", err, err)
  	}
  }

  func TestDLSym_MissingSymbolErrors(t *testing.T) {
  	handle, err := dlOpen("")
  	if err != nil {
  		t.Skip("cannot open self:", err)
  	}
  	_, err = dlSym(handle, "definitely_not_a_real_symbol_zzz")
  	if err == nil {
  		t.Fatal("expected error for missing symbol")
  	}
  }
  ```

- [ ] **Step 7: Run tests, confirm all green**

  Run: `go test -race -covermode=atomic -coverprofile=cov.out ./internal/lokc/... && go tool cover -func=cov.out | tail -n 1 && rm cov.out`
  Expected: `PASS`. Coverage on this file alone will not yet hit
  the 90% gate (Task 3–5 add the remaining tested code paths); that
  is fine, the gate is checked only at Task 6.

- [ ] **Step 8: Commit**

  ```bash
  git add internal/lokc/doc.go internal/lokc/dlopen_unix.go internal/lokc/dlopen_test.go
  git commit -m "feat(lokc): add cgo dlopen/dlsym wrappers

Single cgo file wrapping dlopen(RTLD_LAZY|RTLD_LOCAL) and dlsym
with a typed DLError. Empty Go path translates to dlopen(NULL) so
the main-program handle is returned portably on Linux and macOS.
Unit-tested against dlopen(NULL) + libc symbols so no external
runtime is required; integration tests in Phase 1 cover the real
LO library path behind the lok_integration tag.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 3: `OpenLibrary` facade (TDD)

**Files:**
- Create: `internal/lokc/loader.go`
- Create: `internal/lokc/loader_test.go`

### 3.1 Failing tests first

- [ ] **Step 1: Create `internal/lokc/loader_test.go`**

  ```go
  package lokc

  import (
  	"errors"
  	"os"
  	"os/exec"
  	"path/filepath"
  	"runtime"
  	"testing"
  )

  func TestOpenLibrary_EmptyPathErrors(t *testing.T) {
  	_, err := OpenLibrary("")
  	if !errors.Is(err, ErrInstallPathRequired) {
  		t.Fatalf("want ErrInstallPathRequired, got %v", err)
  	}
  }

  func TestOpenLibrary_MissingFileErrors(t *testing.T) {
  	_, err := OpenLibrary("/nonexistent/install/path")
  	if err == nil {
  		t.Fatal("expected error for missing install dir")
  	}
  	var dlerr *DLError
  	if !errors.As(err, &dlerr) || dlerr.Op != "dlopen" {
  		t.Errorf("want *DLError Op=dlopen, got %T %v", err, err)
  	}
  }

  // buildFakeSO compiles an empty shared library named libsofficeapp.{so,dylib}
  // into a fresh temp dir. Returns that dir (suitable for OpenLibrary).
  // Skips the test if `cc` is not installed. Accepts zero or more hook
  // symbols to export.
  func buildFakeSO(t *testing.T, hookSymbols ...string) string {
  	t.Helper()
  	cc, err := exec.LookPath("cc")
  	if err != nil {
  		t.Skip("cc not available:", err)
  	}
  	tmp := t.TempDir()
  	src := filepath.Join(tmp, "fake.c")
  	body := "void _lok_fake_marker(void) {}\n"
  	for _, sym := range hookSymbols {
  		body += "void " + sym + "(void) {}\n"
  	}
  	if err := os.WriteFile(src, []byte(body), 0o644); err != nil {
  		t.Fatal(err)
  	}
  	soName := "libsofficeapp.so"
  	ccArgs := []string{"-shared", "-fPIC"}
  	if runtime.GOOS == "darwin" {
  		soName = "libsofficeapp.dylib"
  		ccArgs = []string{"-dynamiclib"}
  	}
  	so := filepath.Join(tmp, soName)
  	out, err := exec.Command(cc, append(ccArgs, "-o", so, src)...).CombinedOutput()
  	if err != nil {
  		t.Fatalf("cc build failed: %v\n%s", err, out)
  	}
  	return tmp
  }

  func TestOpenLibrary_MissingHookSymbolErrors(t *testing.T) {
  	dir := buildFakeSO(t) // no hook symbols
  	_, err := OpenLibrary(dir)
  	if err == nil {
  		t.Fatal("expected error for .so without hook symbols")
  	}
  	var dlerr *DLError
  	if !errors.As(err, &dlerr) || dlerr.Op != "dlsym" {
  		t.Errorf("want *DLError Op=dlsym, got %T %v", err, err)
  	}
  }

  func TestOpenLibrary_ResolvesHook2(t *testing.T) {
  	// Fake exports only libreofficekit_hook_2 — should resolve with version 2.
  	dir := buildFakeSO(t, "libreofficekit_hook_2")
  	lib, err := OpenLibrary(dir)
  	if err != nil {
  		t.Fatalf("OpenLibrary: %v", err)
  	}
  	if lib.HookVersion() != 2 {
  		t.Errorf("HookVersion: want 2, got %d", lib.HookVersion())
  	}
  }

  func TestOpenLibrary_PrefersHook2WhenBothExist(t *testing.T) {
  	// Both symbols present — OpenLibrary must prefer hook_2.
  	dir := buildFakeSO(t, "libreofficekit_hook_2", "libreofficekit_hook")
  	lib, err := OpenLibrary(dir)
  	if err != nil {
  		t.Fatalf("OpenLibrary: %v", err)
  	}
  	if lib.HookVersion() != 2 {
  		t.Errorf("with both symbols present, HookVersion: want 2, got %d", lib.HookVersion())
  	}
  }

  func TestOpenLibrary_FallsBackToHook1(t *testing.T) {
  	dir := buildFakeSO(t, "libreofficekit_hook")
  	lib, err := OpenLibrary(dir)
  	if err != nil {
  		t.Fatalf("OpenLibrary: %v", err)
  	}
  	if lib.HookVersion() != 1 {
  		t.Errorf("HookVersion: want 1, got %d", lib.HookVersion())
  	}
  }
  ```

- [ ] **Step 2: Run and observe red**

  Run: `go test ./internal/lokc/... -run OpenLibrary`
  Expected: build failure — `OpenLibrary`, `ErrInstallPathRequired`,
  `*Library`, `HookVersion` are undefined. Red.

### 3.2 Implement

- [ ] **Step 3: Create `internal/lokc/loader.go`**

  ```go
  //go:build linux || darwin

  package lokc

  import (
  	"errors"
  	"path/filepath"
  	"runtime"
  	"unsafe"
  )

  // ErrInstallPathRequired is returned by OpenLibrary when installPath is empty.
  var ErrInstallPathRequired = errors.New("lokc: install path is required")

  // Library is a dlopen'd LibreOffice runtime with a resolved hook symbol.
  // Close is intentionally not provided: LibreOffice's static initialisers
  // cannot be re-run within the same process.
  type Library struct {
  	installPath string
  	handle      unsafe.Pointer
  	hookSymbol  unsafe.Pointer
  	hookVersion int // 2 for libreofficekit_hook_2, 1 for libreofficekit_hook
  }

  // InstallPath returns the path that was passed to OpenLibrary.
  func (l *Library) InstallPath() string { return l.installPath }

  // HookVersion returns 2 for libreofficekit_hook_2, 1 for libreofficekit_hook.
  func (l *Library) HookVersion() int { return l.hookVersion }

  // HookSymbol returns the resolved function pointer. Callers must cast
  // to the right signature (done in Phase 2).
  func (l *Library) HookSymbol() unsafe.Pointer { return l.hookSymbol }

  // OpenLibrary dlopens <installPath>/libsofficeapp.so and resolves
  // libreofficekit_hook_2 (falling back to libreofficekit_hook). It does
  // NOT invoke the hook; that is Phase 2.
  func OpenLibrary(installPath string) (*Library, error) {
  	if installPath == "" {
  		return nil, ErrInstallPathRequired
  	}
  	return openWithPath(
  		filepath.Join(installPath, soFilename()),
  		installPath,
  		"libreofficekit_hook_2",
  		"libreofficekit_hook",
  	)
  }

  func soFilename() string {
  	if runtime.GOOS == "darwin" {
  		return "libsofficeapp.dylib"
  	}
  	return "libsofficeapp.so"
  }

  // openWithPath is the test-facing seam: it takes an absolute library
  // path plus ordered symbol candidates and tries them in sequence.
  func openWithPath(libPath, installPath, preferredSym, fallbackSym string) (*Library, error) {
  	handle, err := dlOpen(libPath)
  	if err != nil {
  		return nil, err
  	}
  	if sym, err := dlSym(handle, preferredSym); err == nil {
  		return &Library{installPath: installPath, handle: handle, hookSymbol: sym, hookVersion: 2}, nil
  	}
  	sym, err := dlSym(handle, fallbackSym)
  	if err != nil {
  		return nil, err
  	}
  	return &Library{installPath: installPath, handle: handle, hookSymbol: sym, hookVersion: 1}, nil
  }
  ```

- [ ] **Step 4: Run tests; confirm all pass**

  Run: `go test -race ./internal/lokc/...`
  Expected: all tests PASS. Skips only on machines without `cc`.

- [ ] **Step 5: Commit**

  ```bash
  git add internal/lokc/loader.go internal/lokc/loader_test.go
  git commit -m "feat(lokc): add OpenLibrary loader with hook-symbol fallback

OpenLibrary(installPath) joins libsofficeapp.{so,dylib} onto the
path, dlopens it, and resolves libreofficekit_hook_2 with a
libreofficekit_hook fallback. Does not invoke the hook — that is
Phase 2. Unit tests cover empty-path, missing-file, missing-symbol,
hook_2 preferred, and hook_1 fallback paths using a cc-built fake
.so; tests skip cleanly if cc is unavailable.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 4: `errstr` helper (TDD)

**Files:**
- Create: `internal/lokc/errstr.go`
- Create: `internal/lokc/errstr_test.go`

- [ ] **Step 1: Write the failing test**

  `internal/lokc/errstr_test.go`:
  ```go
  package lokc

  /*
  #include <stdlib.h>
  */
  import "C"

  import (
  	"testing"
  	"unsafe"
  )

  func TestCopyAndFree_RoundTrip(t *testing.T) {
  	// Heap-allocate a C string the same way LOK does (malloc'd, caller frees).
  	msg := C.CString("hello, lok")
  	// copyAndFree must free msg and return the Go string.
  	got := copyAndFree((*C.char)(unsafe.Pointer(msg)))
  	if got != "hello, lok" {
  		t.Errorf("got %q", got)
  	}
  }

  func TestCopyAndFree_Nil(t *testing.T) {
  	if got := copyAndFree(nil); got != "" {
  		t.Errorf("nil input: got %q want \"\"", got)
  	}
  }
  ```

- [ ] **Step 2: Run test — red**

  Run: `go test ./internal/lokc/... -run CopyAndFree`
  Expected: build error (`copyAndFree` undefined).

- [ ] **Step 3: Implement `errstr.go`**

  ```go
  //go:build linux || darwin

  package lokc

  /*
  #include <stdlib.h>
  */
  import "C"

  import "unsafe"

  // copyAndFree copies a C string into a Go string and frees the
  // original with free(3). Safe on nil input (returns "").
  //
  // LOK returns char* from getError / getVersionInfo / etc. that the
  // caller owns; every wrapper that sees such a pointer should pass it
  // through here so the free cannot be forgotten.
  func copyAndFree(cs *C.char) string {
  	if cs == nil {
  		return ""
  	}
  	defer C.free(unsafe.Pointer(cs))
  	return C.GoString(cs)
  }
  ```

- [ ] **Step 4: Run — green**

  Run: `go test -race ./internal/lokc/...`
  Expected: all tests PASS.

- [ ] **Step 5: Commit**

  ```bash
  git add internal/lokc/errstr.go internal/lokc/errstr_test.go
  git commit -m "feat(lokc): add copyAndFree for LOK-owned C strings

Centralises the copy-then-free pattern for char* values returned by
getError/getVersionInfo/etc. so future wrappers cannot forget to
free. Nil-safe.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 5: Integration test

**Files:**
- Create: `internal/lokc/loader_integration_test.go`

- [ ] **Step 1: Create the integration test**

  `internal/lokc/loader_integration_test.go`:
  ```go
  //go:build lok_integration

  package lokc

  import (
  	"os"
  	"testing"
  )

  func TestIntegration_OpenRealLOK(t *testing.T) {
  	path := os.Getenv("LOK_PATH")
  	if path == "" {
  		t.Skip("LOK_PATH not set")
  	}
  	lib, err := OpenLibrary(path)
  	if err != nil {
  		t.Fatalf("OpenLibrary(%q): %v", path, err)
  	}
  	if lib.HookSymbol() == nil {
  		t.Fatal("hook symbol is nil")
  	}
  	if v := lib.HookVersion(); v != 1 && v != 2 {
  		t.Errorf("HookVersion: want 1 or 2, got %d", v)
  	}
  	if lib.InstallPath() != path {
  		t.Errorf("InstallPath: want %q, got %q", path, lib.InstallPath())
  	}
  }
  ```

- [ ] **Step 2: Verify it builds under the tag**

  Run: `go vet -tags=lok_integration ./internal/lokc/...`
  Expected: exit 0, no output.

- [ ] **Step 3: Run integration if LO is installed**

  Run: `LOK_PATH=/usr/lib64/libreoffice/program make test-integration`
  (Fedora path; adjust for Ubuntu/macOS.)
  Expected: the integration test passes. If `LOK_PATH` is not set the
  test skips — still a green run.

- [ ] **Step 4: Verify it skips without LOK_PATH**

  Run: `make test-integration`
  Expected: `--- SKIP: TestIntegration_OpenRealLOK (0.00s)` with a
  `LOK_PATH not set` line.

- [ ] **Step 5: Commit**

  ```bash
  git add internal/lokc/loader_integration_test.go
  git commit -m "test(lokc): add loader integration test behind lok_integration tag

Opens the real LibreOffice runtime from \$LOK_PATH, verifies a hook
symbol resolves and the version is 1 or 2. Skips cleanly when
LOK_PATH is unset so plain 'go test ./...' stays green without
LibreOffice installed.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 6: Coverage gate + Makefile/CI update

**Files:**
- Modify: `Makefile`
- Modify: `.github/workflows/ci.yml`

### 6.1 Makefile target

- [ ] **Step 1: Add `cover-gate` target**

  Append to `Makefile`. The snippet declares its own
  `.PHONY: cover-gate` line, so no edit to the existing `.PHONY`
  header at the top of the file is required.

  ```makefile
  # Fail if internal/lokc coverage falls below 90%. Extend to other
  # packages as they arrive (lok from Phase 2 onward).
  COVER_GATE_PKGS := ./internal/lokc/...
  COVER_GATE_MIN  := 90.0

  .PHONY: cover-gate
  cover-gate:
  	$(GO) test -covermode=atomic -coverprofile=$(COVER_OUT) $(COVER_GATE_PKGS)
  	@total=$$( $(GO) tool cover -func=$(COVER_OUT) | awk '/^total:/ {print $$3}' | tr -d '%' ); \
  	if [ -z "$$total" ]; then \
  	  echo "cover-gate: no 'total:' line in $(COVER_OUT) — is the profile empty?" >&2; \
  	  exit 2; \
  	fi; \
  	awk -v t="$$total" -v m="$(COVER_GATE_MIN)" 'BEGIN { \
  	  if (t+0 < m+0) { printf "coverage %.1f%% < %.1f%% (gate)\n", t, m; exit 1 } \
  	  printf "coverage %.1f%% >= %.1f%% ok\n", t, m \
  	}'
  ```

  Add `cover-gate` to the `.PHONY` line at the top.

- [ ] **Step 2: Verify the gate passes**

  Run: `make cover-gate`
  Expected: `coverage XX.X% >= 90.0% ok`, exit 0.

- [ ] **Step 3: Verify the gate fails when forced**

  Run:
  ```bash
  make COVER_GATE_MIN=99.9 cover-gate; echo exit=$?
  ```
  Expected: the script prints `coverage XX.X% < 99.9% (gate)` and
  exits 1.

### 6.2 CI step

- [ ] **Step 4: Ensure `cc` is available in CI and add `cover-gate`**

  `ubuntu-24.04` ships `build-essential`, so `cc` is present. The
  `buildFakeSO` helper needs it; if a future runner image drops it,
  the fake-.so tests will `t.Skip` and coverage will silently fall
  below the gate. Add an explicit install step up-front, before
  `actions/setup-go@v5`:

  ```yaml
      - name: Ensure cc is available
        run: |
          if ! command -v cc >/dev/null 2>&1; then
            sudo apt-get update
            sudo apt-get install -y --no-install-recommends build-essential
          fi
          cc --version | head -n 1
  ```

  Then replace the existing "Coverage (report only, gate added in
  later phases)" step with:

  ```yaml
      - name: Coverage gate (internal/lokc ≥ 90%)
        run: make cover-gate
  ```

  Keep the gate step at the same position (after `go test -race`).

- [ ] **Step 5: Verify the workflow parses**

  Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml'))" && echo YAML_OK`
  Expected: `YAML_OK`.

- [ ] **Step 6: Commit**

  ```bash
  git add Makefile .github/workflows/ci.yml
  git commit -m "ci: add cover-gate target (internal/lokc >= 90%)

Replaces the Phase-0 best-effort coverage report with a hard gate on
internal/lokc. The threshold (COVER_GATE_MIN) and package set
(COVER_GATE_PKGS) are Make variables so Phase 2 can extend to
./lok/... by adding to the list. The gate is enforced in CI, which
now also verifies cc is present because the unit tests build a fake
shared library to exercise error paths in OpenLibrary.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 7: Final verification

**Files:** none

- [ ] **Step 1: Full clean run**

  Run:
  ```bash
  make clean
  make all
  make cover
  make cover-gate
  make test-integration
  ```
  Expected: every command exits 0. `make cover-gate` prints the
  coverage line.

- [ ] **Step 2: Branch topology**

  Run: `git log --oneline --graph main..HEAD`
  Expected: 6 commits on `chore/dlopen-loader` above `main`
  (headers, dlopen wrappers, OpenLibrary, errstr, integration test,
  cover-gate/CI).

- [ ] **Step 3: Open the PR (with user confirmation)**

  Ask the user before pushing. If approved, push and open:

  ```bash
  git push -u origin chore/dlopen-loader
  gh pr create --base main --title "Phase 1: dlopen loader" --body "$(cat <<'EOF'
  ## Summary
  - Vendors LibreOfficeKit 24.8.7.2 headers under third_party/lok/.
  - Adds internal/lokc package: cgo dlopen/dlsym wrappers + OpenLibrary
    facade with libreofficekit_hook_2 fallback to libreofficekit_hook.
  - Adds copyAndFree helper for C-owned error strings.
  - Integration test behind lok_integration tag.
  - CI gate: internal/lokc coverage >= 90%.

  Implements Phase 1 of docs/superpowers/specs/2026-04-19-lok-binding-design.md.

  ## Test plan
  - [x] make all
  - [x] make cover-gate (>=90% on internal/lokc)
  - [x] make test-integration with LOK_PATH set (opens real LO)
  - [x] make test-integration without LOK_PATH (skips cleanly)
  - [ ] CI green

  🤖 Generated with [Claude Code](https://claude.com/claude-code)
  EOF
  )"
  ```

---

## Acceptance criteria (matches spec §Phase 1)

- [ ] `internal/lokc` package exists with dlopen-based loader.
- [ ] `OpenLibrary` resolves `libreofficekit_hook_2` with
      `libreofficekit_hook` fallback.
- [ ] Unit tests cover: empty path, missing file, missing symbol,
      hook_2 preferred, hook_1 fallback.
- [ ] `make cover-gate` enforces ≥ 90% on `internal/lokc`.
- [ ] Integration test opens real LO and resolves the hook symbol.
- [ ] No public `lok` API yet — that is Phase 2.
- [ ] `dlclose` is not called (see spec §5.2).

When every box is ticked, `chore/dlopen-loader` is ready to merge;
Phase 2's plan (`feat/office-lifecycle`) can begin.
