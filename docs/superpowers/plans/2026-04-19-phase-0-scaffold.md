# Phase 0 — Module Scaffold Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bootstrap the `github.com/julianshen/golibreofficekit` Go module with a
buildable empty tree, a `Makefile` wrapping the workflow, a GitHub Actions CI
pipeline, and a README stub. No LOK code yet — this plan only sets up the
ground the later phases stand on.

**Architecture:** An empty Go module. `make build`, `make test`, `make lint`,
`make fmt`, `make cover` wrap the canonical Go commands so every later phase
uses the same entrypoints. CI runs `go vet`, `gofmt -l` (must be silent), and
`go test -race` on linux/amd64. No production packages exist yet, so `go test`
is a vacuous green that proves the toolchain, module path, and CI wiring —
the very first real test arrives in Phase 1 (dlopen loader).

**Tech Stack:** Go 1.23+, GNU Make, GitHub Actions, `gofmt`, `go vet`. No
external Go dependencies in this phase.

**Branching:** All work happens on `chore/scaffold`, branched from `main`.
Before starting, the `chore/design-spec` branch must be merged to `main`
(the spec is a dependency of this plan).

---

## Files

All paths relative to the repo root `/home/julianshen/prj/golibreofficekit`.

| Path | Role |
|------|------|
| `go.mod` (create) | Declares module path and Go version |
| `go.sum` (auto-generated) | Created by `go mod tidy` when the first dependency is added; not present after Phase 0 |
| `.gitignore` (create) | Excludes build artefacts, coverage files, editor cruft |
| `Makefile` (create) | Wraps `build`, `test`, `test-integration`, `cover`, `lint`, `fmt`, `fmt-check` |
| `README.md` (create) | One-page stub pointing at `CLAUDE.md` and the spec |
| `.github/workflows/ci.yml` (create) | Lint + unit-test job on linux/amd64, Go 1.23 & tip |
| `docs/superpowers/plans/2026-04-19-phase-0-scaffold.md` (this file, already created) | This plan |

No code files are created in Phase 0 on purpose. Phase 1 introduces
`internal/lokc/` with the first real test.

Coverage gate note (per spec §6): the `lok` package does not yet exist, so
the `lok ≥ 90%` rule is vacuously satisfied. The `internal/lokc` gate
activates in Phase 1. CI for Phase 0 therefore runs only `go vet`, `gofmt
-l`, and `go test -race ./...` (the last returns "no test files" — a
green run).

---

## Task 0: Prepare branch

**Files:** none

- [ ] **Step 1: Verify working tree is clean on `chore/design-spec`**

  Run: `git status --short`
  Expected: empty output.

- [ ] **Step 2: Merge the spec branch to `main`**

  Run:
  ```bash
  git checkout main
  git merge --ff-only chore/design-spec
  git merge-base --is-ancestor chore/design-spec HEAD && echo MERGED_OK
  ```
  Expected: fast-forward succeeds and `MERGED_OK` is printed. If a
  non-fast-forward error fires, stop and ask the user; do not `git merge
  --no-ff` without authorisation.

  Rollback (if needed, before pushing): `git reset --hard ORIG_HEAD` on
  `main`.

- [ ] **Step 3: Create and switch to `chore/scaffold`**

  Run:
  ```bash
  git checkout -b chore/scaffold
  git branch --show-current
  ```
  Expected: `chore/scaffold`.

---

## Task 1: `go.mod`

**Files:**
- Create: `go.mod`

- [ ] **Step 1: Write the verification command**

  Run: `test -f go.mod && echo present || echo missing`
  Expected: `missing`.

- [ ] **Step 2: Create `go.mod`**

  Contents:
  ```
  module github.com/julianshen/golibreofficekit

  go 1.23
  ```

- [ ] **Step 3: Verify the module builds**

  Run: `go build ./...`
  Expected: exit 0, no output, no packages built (there are none yet).

- [ ] **Step 4: Verify `go vet`**

  Run: `go vet ./...`
  Expected: exit 0, no output.

- [ ] **Step 5: Verify `go test`**

  Run: `go test ./...`
  Expected: exit 0, no output. `./...` on a module with zero packages
  pattern-matches nothing, so `go test` silently succeeds — the "no Go
  files" message only appears when `go test` is invoked on a specific
  directory containing `.go` files without tests.

- [ ] **Step 6: Commit**

  ```bash
  git add go.mod
  git commit -m "chore(scaffold): initialise go.mod for golibreofficekit

Module path: github.com/julianshen/golibreofficekit; Go 1.23 minimum
per spec §1.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 2: `.gitignore`

**Files:**
- Create: `.gitignore`

- [ ] **Step 1: Create `.gitignore`**

  Contents:
  ```
  # Build output
  /bin/
  /dist/

  # Coverage
  coverage.out
  coverage.html

  # Editor state
  .idea/
  .vscode/
  *.swp
  *.swo

  # OS
  .DS_Store

  # Go
  *.test
  *.out
  ```

- [ ] **Step 2: Verify nothing accidentally ignored is tracked**

  Run: `git status --ignored --short`
  Expected: no tracked file appears under `!!`. Untracked/ignored entries
  are fine.

- [ ] **Step 3: Commit**

  ```bash
  git add .gitignore
  git commit -m "chore(scaffold): add .gitignore

Covers build output, coverage artefacts, and common editor/OS cruft.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 3: `Makefile`

**Files:**
- Create: `Makefile`

The Makefile is the workflow contract for every later phase; every
phase of the spec references these targets verbatim.

- [ ] **Step 1: Confirm Makefile is absent (red state)**

  Run: `test ! -f Makefile && echo ABSENT`
  Expected: `ABSENT`.

- [ ] **Step 2: Create `Makefile`**

  Contents (TAB-indented — this matters). `GOFLAGS ?=` is intentionally
  empty; it exists so callers can override (`make test GOFLAGS='-v'`):
  ```makefile
  # golibreofficekit — project Makefile.
  # Every target is idempotent and safe to re-run.

  SHELL        := /usr/bin/env bash
  .SHELLFLAGS  := -eu -o pipefail -c

  GO           ?= go
  GOFLAGS      ?=
  PKG          := ./...
  COVER_OUT    := coverage.out
  COVER_HTML   := coverage.html
  INTEGRATION_TAG := lok_integration

  .PHONY: all build test test-integration cover lint fmt fmt-check vet tidy clean

  all: fmt-check vet test

  build:
  	$(GO) build $(GOFLAGS) $(PKG)

  test:
  	$(GO) test -race $(GOFLAGS) $(PKG)

  test-integration:
  	$(GO) test -race -tags=$(INTEGRATION_TAG) $(GOFLAGS) $(PKG)

  cover:
  	$(GO) test -covermode=atomic -coverprofile=$(COVER_OUT) $(PKG)
  	$(GO) tool cover -func=$(COVER_OUT) | tail -n 1
  	$(GO) tool cover -html=$(COVER_OUT) -o $(COVER_HTML)

  vet:
  	$(GO) vet $(PKG)

  # Rewrite in place.
  fmt:
  	gofmt -w .

  # Read-only guard used by CI and `make lint`. Fails if any file is
  # unformatted — never rewrites.
  fmt-check:
  	@unformatted="$$(gofmt -l .)"; \
  	if [ -n "$$unformatted" ]; then \
  	  echo "gofmt needed on:"; echo "$$unformatted"; exit 1; \
  	fi

  lint: vet fmt-check

  tidy:
  	$(GO) mod tidy

  clean:
  	rm -f $(COVER_OUT) $(COVER_HTML)
  ```

  Why two formatting targets: `fmt` rewrites, `fmt-check` is read-only.
  CI and `lint` use `fmt-check` so "silent success" genuinely means "no
  file needed formatting", rather than "we auto-fixed everything".

- [ ] **Step 3: Verify each target on the empty module**

  Run:
  ```bash
  make build     && echo BUILD_OK
  make vet       && echo VET_OK
  make fmt       && echo FMT_OK
  make fmt-check && echo FMT_CHECK_OK
  make test      && echo TEST_OK
  make cover     && echo COVER_OK
  make lint      && echo LINT_OK
  make clean     && echo CLEAN_OK
  ```
  Expected: every target prints its `_OK` line. `make cover` prints
  `total:	(statements)	0.0%` on the penultimate line — expected for
  Phase 0 and tightens to ≥ 90% on `lok` from Phase 2.

- [ ] **Step 4: Verify `make test-integration` on a machine without LO**

  Run: `make test-integration`
  Expected: exit 0, no output. There are no packages matching
  `./...` yet, so the tagged build is silently green. Phase 1 adds the
  first tag-gated test.

- [ ] **Step 5: Commit**

  ```bash
  git add Makefile
  git commit -m "chore(scaffold): add Makefile

Wraps build/test/test-integration/cover/lint/fmt/tidy/clean. Every
subsequent phase uses these targets so the workflow contract stays
stable across the project.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 4: `README.md`

**Files:**
- Create: `README.md`

- [ ] **Step 1: Create `README.md`**

  Contents:
  ```markdown
  # golibreofficekit

  Go binding for [LibreOfficeKit](https://wiki.documentfoundation.org/Development/LibreOfficeKit)
  (LOK), the C ABI exposed by LibreOffice for in-process document loading,
  rendering, and editing (Writer, Calc, Impress, Draw).

  **Status:** pre-alpha, under active development. No public API yet.

  ## Development

  - Project guide for agentic workers: [`CLAUDE.md`](./CLAUDE.md)
  - Design spec: [`docs/superpowers/specs/2026-04-19-lok-binding-design.md`](./docs/superpowers/specs/2026-04-19-lok-binding-design.md)
  - Implementation plans: [`docs/superpowers/plans/`](./docs/superpowers/plans/)

  Common commands:

  ```bash
  make build             # go build ./...
  make test              # go test -race ./...
  make test-integration  # adds -tags=lok_integration; needs LibreOffice
  make cover             # produces coverage.out and coverage.html
  make lint              # go vet + gofmt check
  ```

  ## Requirements (target)

  - Go 1.23 or newer.
  - LibreOffice 24.8+ installed for integration tests (`LOK_PATH` points
    at the `program/` directory).
  - Linux or macOS. Windows is not supported in the first release.

  ## Licence

  To be decided before the first tagged release.
  ```

- [ ] **Step 2: Verify the README renders cleanly**

  Run: `cat README.md | head -n 5`
  Expected: the top-level heading and first paragraph.

- [ ] **Step 3: Commit**

  ```bash
  git add README.md
  git commit -m "chore(scaffold): add README stub

Points at CLAUDE.md and the design spec, lists the Make targets, notes
the target requirements.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 5: GitHub Actions CI

**Files:**
- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: Create the workflows directory**

  Run: `mkdir -p .github/workflows && test -d .github/workflows && echo OK`
  Expected: `OK`. `mkdir -p` is idempotent, so re-running is safe.

- [ ] **Step 2: Create `.github/workflows/ci.yml`**

  Contents:
  ```yaml
  name: CI

  on:
    push:
      branches: [main]
    pull_request:
      branches: [main]

  permissions:
    contents: read

  jobs:
    lint-and-test:
      name: lint + unit tests (Go ${{ matrix.go }})
      runs-on: ubuntu-24.04
      strategy:
        fail-fast: false
        matrix:
          include:
            - go: "1.23"
              required: true
            - go: "stable"
              required: false
      continue-on-error: ${{ !matrix.required }}
      steps:
        - uses: actions/checkout@v4

        - name: Set up Go ${{ matrix.go }}
          uses: actions/setup-go@v5
          with:
            go-version: ${{ matrix.go }}
            check-latest: true
            cache: true

        - name: Show Go version
          run: go version

        - name: go vet
          run: go vet ./...

        - name: gofmt check
          run: |
            unformatted="$(gofmt -l .)"
            if [ -n "$unformatted" ]; then
              echo "gofmt needed on:"
              echo "$unformatted"
              exit 1
            fi

        - name: go test -race
          run: go test -race ./...

        - name: Coverage (report only, gate added in later phases)
          run: |
            go test -covermode=atomic -coverprofile=coverage.out ./...
            # coverage.out always contains at least `mode: atomic\n`; count
            # real data lines to decide whether to print a total.
            if [ "$(grep -cv '^mode:' coverage.out)" -gt 0 ]; then
              go tool cover -func=coverage.out | tail -n 1
            else
              echo "no covered packages yet — expected in Phase 0"
            fi
  ```

  Notes for the implementer:
  - Python/Node are not required — `actions/setup-go@v5` is the only
    toolchain setup.
  - Matrix on `1.23` (minimum, required) and `stable` (floating,
    `continue-on-error`) — a Go release breaking only on tip does not
    block the PR.
  - The coverage step is best-effort in Phase 0 and will be promoted to
    a hard gate (`< 90% → fail`) in Phase 2 when `lok` exists.

- [ ] **Step 3: Lint the YAML locally**

  Run:
  ```bash
  python3 -c "import yaml, sys; yaml.safe_load(open('.github/workflows/ci.yml'))" \
    && echo YAML_OK
  ```
  Expected: `YAML_OK`. If `python3` or PyYAML is unavailable, skip this
  step — CI will catch invalid YAML on push.

- [ ] **Step 4: Commit**

  ```bash
  git add .github/workflows/ci.yml
  git commit -m "chore(scaffold): add GitHub Actions CI

Runs go vet, gofmt check, and go test -race on a matrix of Go 1.23 and
stable. Coverage is reported but not gated in Phase 0 — gate promotes to
a hard 90% failure in Phase 2 when the lok package exists.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 6: Final verification

**Files:** none

- [ ] **Step 1: Clean workspace check**

  Run: `git status --short`
  Expected: empty.

- [ ] **Step 2: Run the full workflow**

  Run:
  ```bash
  make clean
  make all              # fmt-check + vet + test
  make cover
  make test-integration
  ```
  Expected: all commands exit 0; `make cover` prints a total line
  (`total:	(statements)	0.0%`). `make test` and
  `make test-integration` emit no output on an empty module.

- [ ] **Step 3: Confirm branch topology**

  Run: `git log --oneline --graph --all -n 10`
  Expected: `chore/scaffold` has 5 new commits on top of `main`
  (`go.mod`, `.gitignore`, `Makefile`, `README.md`, `ci.yml`), and `main`
  itself holds the spec commits from the merge in Task 0.

- [ ] **Step 4: (Optional) Open a PR**

  If the user wants a PR:
  ```bash
  git push -u origin chore/scaffold
  gh pr create --base main --title "chore(scaffold): bootstrap module, Makefile, CI, README" \
    --body "$(cat <<'EOF'
  ## Summary
  - Creates `go.mod` for module `github.com/julianshen/golibreofficekit` (Go 1.23).
  - Adds `Makefile` with build/test/cover/lint/fmt/tidy/clean + `test-integration`.
  - Adds README stub linking to CLAUDE.md and the design spec.
  - Adds GitHub Actions CI (matrix: Go 1.23, stable; go vet, gofmt, go test -race, coverage report).

  Implements Phase 0 of `docs/superpowers/specs/2026-04-19-lok-binding-design.md`.

  ## Test plan
  - [x] `make all` green locally
  - [x] `make cover` produces `coverage.out` (0.0% expected — no code yet)
  - [x] `make test-integration` green (no tagged tests yet)
  - [ ] CI green on PR

  🤖 Generated with [Claude Code](https://claude.com/claude-code)
  EOF
  )"
  ```
  **Ask before pushing.** Pushing and opening PRs is a shared-state
  action per the session rules; get explicit authorisation.

---

## Acceptance criteria (matches spec §Phase 0)

- [ ] `make build` succeeds on a fresh clone.
- [ ] `make test` exits 0 (no packages yet — silent green).
- [ ] `make lint` (= `vet` + `fmt-check`) exits 0 with no output.
- [ ] `make cover` produces `coverage.out` without error.
- [ ] CI workflow file parses and runs on push/PR.
- [ ] No LOK code, no vendored headers yet — those belong to Phase 1.

When this plan's boxes are all ticked, `chore/scaffold` is ready to merge
to `main` and Phase 1's plan (`chore/dlopen-loader`) can begin.
