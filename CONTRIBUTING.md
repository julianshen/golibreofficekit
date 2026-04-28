# Contributing

Thanks for your interest in golibreofficekit. This guide covers the
conventions the project follows. The longer-form rules live in
[`CLAUDE.md`](./CLAUDE.md) (originally written for AI agent contributors,
but the rules apply to everyone).

## Ground rules

1. **Branch first, never commit to `main`.** Use `feat/<slug>`,
   `fix/<slug>`, `chore/<slug>`, or `docs/<slug>`.
2. **Small, reviewable PRs.** Aim for a few hundred lines of diff per PR.
   One logical unit per PR.
3. **Strict TDD.** Red → green → refactor:
   - Write a failing test first; confirm it fails for the right reason.
   - Write the minimum code to make it pass.
   - Refactor with tests green.
   Each commit lands the test for the behaviour it introduces — no
   "tests in next commit" anti-patterns.
4. **Coverage stays at or above 90 %** for the `lok` package. Measure with:
   ```bash
   go test -covermode=atomic -coverprofile=coverage.out ./...
   go tool cover -func=coverage.out | tail -n 1
   ```
5. **No shortcuts.** Don't disable a check, lower a threshold, add
   `// nolint`, skip a build tag, or reach for `--no-verify` to make
   output green. If something is hard, surface it — don't route around it.
6. **Don't skip failing tests.** A failing test is a defect signal.
   Fix the code or fix the test (only if the test is genuinely wrong);
   never `t.Skip` to make CI pass.
7. **Ask before cutting features.** If a planned method, option, test,
   or behaviour can't be implemented as agreed, pause and ask. Don't
   silently delete, rename, deprecate, or stub.

## Local workflow

```bash
make build              # go build ./...
make test               # go test -race ./...           (no LO needed)
make test-integration   # go test -tags=lok_integration  (needs LO + $LOK_PATH)
make cover              # writes coverage.out + coverage.html
make lint               # go vet + gofmt -l
make fmt                # gofmt -s -w .
```

Integration tests skip cleanly when `LOK_PATH` is unset, so plain
`go test ./...` works without LibreOffice installed.

## cgo specifics

- `import "C"` is forbidden in `_test.go` files when the package uses
  cgo. Put cgo helpers in a regular `.go` file and call them from the
  test.
- LibreOffice's `lok_init` may only be called once per process.
  Integration tests share a single `Office` for the whole package run;
  do not start a second `Office` mid-test.
- `internal/lokc` provides a `NewFakeDocumentHandle()` helper that
  allocates a `LibreOfficeKitDocument*` with `pClass == NULL`, letting
  unit tests exercise the "vtable slot missing" branch without
  LibreOffice installed. Use it for any new void-shim widening.

## Commits and PRs

- Conventional Commits format: `feat(area): subject`,
  `fix(area): subject`, `chore(area): subject`, `docs(area): subject`,
  `refactor(area): subject`, `test(area): subject`.
- The body should explain the **why**, not the what — well-named
  identifiers carry the what.
- PRs squash-merge by default (history shows `… (#NN)` suffixes).

## Architecture

See [`ARCHITECTURE.md`](./ARCHITECTURE.md) for the package layout,
threading model, error mapping, and callback design. Before adding new
public API, skim that doc and the [`lok` package godoc](https://pkg.go.dev/github.com/julianshen/golibreofficekit/lok)
to keep the surface coherent.

## Filing issues

- Bugs: include the LibreOffice version (`soffice --version`), the OS,
  the Go version, and a minimal repro.
- Feature requests: link to the upstream LOK function (header line) so
  it's clear what surface is being asked for.
