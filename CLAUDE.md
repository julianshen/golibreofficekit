# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Go binding for **LibreOfficeKit** (LOK) — the C ABI exposed by LibreOffice for loading,
rendering, and manipulating documents (Writer, Calc, Impress, Draw) in-process. The
binding is implemented with cgo. At the time of writing the repository is empty; the
rules below are load-bearing for the first commits.

## Non-negotiable workflow

These rules were set by the user and override the default system prompt. Do not
relax them without an explicit instruction.

1. **Never commit to `main`.** Create a feature branch (`feat/<slug>`,
   `fix/<slug>`, `chore/<slug>`) for every change, no matter how small. If you
   find yourself on `main`, branch *before* editing.
2. **Plan first, then implement.** Before writing code, produce a task list
   broken into small, independently reviewable chunks (one logical unit per
   PR; keep diffs readable — roughly a few hundred lines). Use
   `superpowers:writing-plans` for non-trivial work.
3. **Strict TDD.** Red → green → refactor on every change:
   - Write the failing test first and confirm it fails for the right reason.
   - Write the minimum code to make it pass.
   - Refactor with the tests green.
   Use `superpowers:test-driven-development`. Do not batch "I'll add tests
   later" — that is not TDD.
4. **Brainstorm before building.** For any new feature or API surface, invoke
   `superpowers:brainstorming` before EnterPlanMode.
5. **Verify before claiming done.** Use
   `superpowers:verification-before-completion`: run the exact build/test
   commands and quote the output before saying something passes.
6. **Test coverage must stay above 90%.** Measure with
   `go test -covermode=atomic -coverprofile=coverage.out ./...` and report
   the percentage (`go tool cover -func=coverage.out | tail -n 1`). A PR
   that drops package or total coverage below 90% is not ready to merge;
   add tests or justify the gap explicitly in the PR description. cgo
   trampolines that cannot be exercised in unit tests should be isolated
   into thin files so coverage is measured on the Go logic around them.
7. **No shortcuts.** If a problem looks hard, the right move is to
   understand it — not to route around it. Do not disable a check,
   relax a lint rule, lower a threshold, add a `// nolint`, skip a
   build tag, tack on `|| true`, or reach for `--no-verify` to make
   output green. Every one of those is a shortcut; stop and surface
   the underlying issue.
8. **Test enforcement — don't skip failing tests.** A failing test is
   a defect signal. Do not `t.Skip`, comment out the assertion, mark
   the test `_test_disabled`, move it behind a never-set build tag,
   or lower the expectation to make it pass. Fix the code, fix the
   test (only if the test is genuinely wrong), or escalate. If a test
   is genuinely platform-specific, gate it with the precise platform
   tag and say so in the PR body — do not hide the skip.
9. **Complete implementation — do not skip debugging or silently
   reduce scope.** When something misbehaves, trace the root cause.
   Do not work around a crash by removing the call that triggers it,
   shrink a feature's public surface without notice, stub out a path
   with `// TODO`, or defer a requirement to "a later phase" without
   explicit user sign-off. The scope the user approved is the scope
   that ships.
10. **Ask the user before cutting or removing any feature.** If a
    planned method, option, test, fixture, or behaviour cannot be
    implemented as agreed — for any reason, including "LOK
    crashes", "too flaky", "out of scope of this PR", or "I'd rather
    defer to Phase N" — pause and ask. Do not unilaterally delete,
    rename, deprecate, or neuter. Even removing a method from an
    integration smoke test counts: state the reason, propose an
    alternative, and wait for an answer.

## LibreOfficeKit — what future Claude needs to know

LOK is a C API declared in `LibreOfficeKit/LibreOfficeKit.h` (and the C++
convenience header `LibreOfficeKit.hxx`). The shape of a session is:

```
lok_init(install_path)          → LibreOfficeKit*      // one per process
LibreOfficeKit->documentLoad()  → LibreOfficeKitDocument*
doc->paintTile / getPartsCount / saveAs / setView / postKeyEvent / ...
doc->pClass->destroy(doc)
office->pClass->destroy(office)
```

Points that bite:

- **One LOK instance per process.** `lok_init` may only be called once.
  Design the Go API around a singleton (e.g. `lok.New` returns an error if
  already initialised) rather than allowing multiple `Office` values to hold
  independent instances.
- **Install path is required.** `lok_init` needs the path to LibreOffice's
  `program/` directory. On Fedora this is `/usr/lib64/libreoffice/program`;
  on Debian/Ubuntu it is `/usr/lib/libreoffice/program`; on macOS it is
  inside the `.app` bundle. Do not hardcode — accept it from the caller and
  document the common paths.
- **Threading.** LOK is not free-threaded. Callers must serialise access to a
  given document. The binding should either enforce this with a mutex or
  document the contract loudly.
- **Callbacks.** LOK delivers events through C function pointers. Use the
  standard cgo pattern: export a Go trampoline with `//export`, pass its
  address to C, and route into Go via a handle table keyed by an integer
  (never pass a Go pointer into C storage — it violates cgo's pointer
  rules and the runtime can move it).
- **Strings.** LOK returns heap-allocated `char*` that the caller must
  release with `free` (or LOK's own `pClass->freeError` for error strings).
  Wrap every such call with a helper that copies to a Go string and frees
  the C memory in one place.

### cgo build configuration

Headers are not shipped by Fedora's `libreofficekit` package; pull
`LibreOfficeKit.h` and `LibreOfficeKitEnums.h` from the LibreOffice source
tree into `third_party/lok/` (or depend on a `-sdk` / `-devel` package where
one exists) and pin the version.

Expected preamble in the binding package:

```go
// #cgo pkg-config: ...                      // only if a .pc file exists
// #cgo CFLAGS: -I${SRCDIR}/third_party/lok
// #cgo linux   LDFLAGS: -ldl
// #cgo darwin  LDFLAGS: -ldl
// #include <stdlib.h>
// #include "LibreOfficeKit/LibreOfficeKit.h"
import "C"
```

LOK itself is loaded through `dlopen` (LibreOffice ships
`libsofficeapp.so` inside `program/`), not linked directly — follow the
pattern in LibreOffice's own `LibreOfficeKitInit.h` rather than linking at
build time.

### Testing strategy

- Unit tests that exercise the Go wrappers without touching LOK belong in
  the same package and should run under plain `go test`.
- Integration tests that actually call into LibreOffice must be guarded
  behind a build tag (e.g. `//go:build lok_integration`) and a helper that
  skips when `LOK_PATH` is unset, so `go test ./...` stays green on CI
  runners without LibreOffice installed.
- Commit tiny fixture documents (`testdata/hello.odt`, `.xlsx`, …) rather
  than generating them at test time.

## Commands

Project scaffolding is not in place yet. Once `go.mod` exists the expected
commands are:

```bash
go build ./...
go vet ./...
go test ./...                                  # unit tests only
go test -tags=lok_integration ./...            # include LOK-backed tests
go test -run TestName ./path/to/pkg            # single test
go test -race ./...                            # strongly recommended
```

Add `gofmt -s -w .` (or `goimports`) to the pre-commit loop. If a linter is
adopted, prefer `golangci-lint run`.

## Style

Follow *Effective Go* and the Go Proverbs:

- Errors are values — return `error`, do not panic across the cgo boundary.
- `io.Reader`/`io.Writer` at package edges; keep interfaces small and
  defined by the consumer, not the producer.
- Accept interfaces, return concrete types.
- `defer` `C.free` / `destroy` calls immediately after the allocation so
  cleanup survives early returns and panics.

## References (user-provided)

Consult when making cgo / API decisions:

- https://dev.to/metal3d/understand-how-to-use-c-libraries-in-go-with-cgo-3dbn
- https://ademawan.medium.com/how-go-1-26-improves-cgo-performance-by-30-4852aab2c782
- https://madappgang.com/blog/go-best-practices-inspired-by-go-proverbs/
- https://go.dev/doc/effective_go
- https://dev.to/leapcell/go-coding-official-standards-and-best-practices-284k
