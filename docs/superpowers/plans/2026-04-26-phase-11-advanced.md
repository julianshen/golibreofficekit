# Phase 11 — Advanced: macros, signing & certificates (implementation plan)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bind LibreOfficeKit's macro execution, document signing, certificate management, signature-state query, plus gap-fill APIs (`getFilterTypes`, `paste`, `selectPart`, `moveSelectedParts`, `renderFont`).

**Architecture:** Same four-layer pattern as Phases 3–10. `internal/lokc` owns the C shims; `lok` owns the public API and error mapping.

**Tech Stack:** Go + cgo on `linux || darwin`, LibreOfficeKit C ABI, `go test` + integration tests gated by `LOK_PATH` / `LOK_TEST_CERTS`.

**Branch:** `feat/advanced` (to be created from `main`)

**Spec:** `docs/superpowers/specs/2026-04-26-phase-11-advanced-design.md`

---

## File Structure

Files created:

- `lok/advanced.go` — `RunMacro`, `SignDocument` (Office); `InsertCertificate`, `AddCertificate`, `SignatureState` (Document)
- `lok/advanced_test.go` — unit tests
- `lok/filter_types.go` — `Office.FilterTypes`
- `lok/filter_types_test.go` — unit tests
- `lok/misc.go` — `Paste`, `SelectPart`, `MoveSelectedParts`, `RenderFont`
- `lok/misc_test.go` — unit tests
- `internal/lokc/advanced.go` — C shims + Go wrappers for advanced functions
- `internal/lokc/advanced_test.go` — guard-rail tests
- `internal/lokc/misc.go` — C shims + Go wrappers for gap-fill functions
- `internal/lokc/misc_test.go` — guard-rail tests

Files modified:

- `lok/backend.go` — add new interface methods
- `lok/real_backend.go` — add forwarders
- `lok/office_test.go` — add fakeBackend stubs
- `lok/event.go` — add `EventTypeSignatureStatus`
- `lok/errors.go` — add `ErrMacroFailed`, `ErrSignFailed`
- `lok/integration_test.go` — add integration smoke tests

---

## Task breakdown

The plan is split into separate task documents to avoid LLM timeouts.
Implement in order; each task document is self-contained.

| Task | File | Description |
|------|------|-------------|
| 1 | `docs/superpowers/plans/2026-04-26-phase-11-task1-errors-event.md` | Error sentinels + EventTypeSignatureStatus |
| 2 | `docs/superpowers/plans/2026-04-26-phase-11-task2-lokc-advanced.md` | C shims + Go wrappers for advanced functions (lokc) |
| 3 | `docs/superpowers/plans/2026-04-26-phase-11-task3-lokc-misc.md` | C shims + Go wrappers for gap-fill functions (lokc) |
| 4 | `docs/superpowers/plans/2026-04-26-phase-11-task4-backend.md` | Extend backend interface + fakeBackend + realBackend |
| 5 | `docs/superpowers/plans/2026-04-26-phase-11-task5-advanced-api.md` | Public API: advanced.go + tests |
| 6 | `docs/superpowers/plans/2026-04-26-phase-11-task6-filtertypes.md` | Public API: filter_types.go + tests |
| 7 | `docs/superpowers/plans/2026-04-26-phase-11-task7-misc-api.md` | Public API: misc.go + tests |
| 8 | `docs/superpowers/plans/2026-04-26-phase-11-task8-integration.md` | Integration smoke tests |
| 9 | `docs/superpowers/plans/2026-04-26-phase-11-task9-coverage.md` | Coverage verification + final review |

---

## Success Criteria

- [ ] All unit tests pass (`go test ./lok ./internal/lokc -race`).
- [ ] Integration tests pass when `LOK_PATH` is set.
- [ ] `lok` package coverage ≥ 90%.
- [ ] No new lint errors (`go vet`, `gofmt`).
- [ ] Public API matches design spec.
- [ ] Documentation (godoc) added for all new exported symbols.
