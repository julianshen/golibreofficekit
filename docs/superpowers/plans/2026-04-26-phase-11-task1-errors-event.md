# Phase 11 — Task 1: Error sentinels + EventTypeSignatureStatus

**Parent plan:** `docs/superpowers/plans/2026-04-26-phase-11-advanced.md`

---

## Files modified

- `lok/errors.go`
- `lok/event.go`

---

## Step 1: Add error sentinels

In `lok/errors.go`, add to the `var` block:

```go
ErrMacroFailed = errors.New("lok: macro execution failed")
ErrSignFailed  = errors.New("lok: document signing failed")
```

- [ ] Add sentinels and verify `go build ./lok/...`

## Step 2: Add EventTypeSignatureStatus

In `lok/event.go`, add to the `const` block:

```go
EventTypeSignatureStatus EventType = 40 // LOK_CALLBACK_SIGNATURE_STATUS
```

Add the corresponding `String()` case:

```go
case EventTypeSignatureStatus:
    return "EventTypeSignatureStatus"
```

- [ ] Add constant + String case and verify `go build ./lok/...`

## Step 3: Run existing tests

```bash
go test ./lok -race -count=1
```

- [ ] All existing tests pass
