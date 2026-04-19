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
