# Phase 11 — Task 9: Coverage verification + final review

**Parent plan:** `docs/superpowers/plans/2026-04-26-phase-11-advanced.md`

**Prerequisites:** Tasks 1–8 completed.

---

## Step 1: Run coverage

```bash
go test -covermode=atomic -coverprofile=coverage.out ./lok/...
go tool cover -func=coverage.out | tail -5
```

- [ ] `lok` package coverage ≥ 90%

```bash
go test -covermode=atomic -coverprofile=coverage_lokc.out ./internal/lokc/...
go tool cover -func=coverage_lokc.out | tail -5
```

- [ ] `internal/lokc` coverage acceptable

## Step 2: Run full test suite with race detector

```bash
go test -race ./lok/...
go test -race ./internal/lokc/...
```

- [ ] All tests pass, no race conditions

## Step 3: Lint and format

```bash
go vet ./...
gofmt -s -l .
```

- [ ] No vet errors
- [ ] No unformatted files

## Step 4: Build everything

```bash
go build ./...
```

- [ ] Clean build

## Step 5: Update coverage matrix in design spec

Add the following to `docs/superpowers/specs/2026-04-19-lok-binding-design.md` §11:

Under **LibreOfficeKit (office-level)**:

| LOK function     | Phase | Go symbol            |
|------------------|-------|----------------------|
| `getFilterTypes` | 11    | `Office.FilterTypes` |
| `runMacro`       | 11    | `Office.RunMacro`    |
| `signDocument`   | 11    | `Office.SignDocument`|

Under **LibreOfficeKitDocument (per-document)**:

| LOK function          | Phase | Go symbol                       |
|-----------------------|-------|---------------------------------|
| `insertCertificate`   | 11    | `Document.InsertCertificate`    |
| `addCertificate`      | 11    | `Document.AddCertificate`       |
| `getSignatureState`   | 11    | `Document.SignatureState`       |
| `paste`               | 11    | `Document.Paste`                |
| `selectPart`          | 11    | `Document.SelectPart`           |
| `moveSelectedParts`   | 11    | `Document.MoveSelectedParts`    |
| `renderFont`          | 11    | `Document.RenderFont`           |

- [ ] Coverage matrix updated

## Step 6: Verify all checks pass

```bash
go build ./... && go vet ./... && go test -race ./lok/... ./internal/lokc/...
```

- [ ] All green
