# Phase 11 — Task 6: Public API — filter_types.go + tests

**Parent plan:** `docs/superpowers/plans/2026-04-26-phase-11-advanced.md`

**Prerequisites:** Task 4 completed (backend interface extended).

---

## Files created

- `lok/filter_types.go`
- `lok/filter_types_test.go`

---

## Step 1: Create `lok/filter_types.go`

```go
//go:build linux || darwin

package lok

// FilterTypes returns the list of document filters LibreOffice supports as a
// JSON string. The format is documented in LibreOfficeKit.h.
// Returns ErrClosed if the Office has been closed.
func (o *Office) FilterTypes() (string, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return "", ErrClosed
	}
	return o.be.OfficeGetFilterTypes(o.h)
}
```

- [ ] Create file and verify `go build ./lok`

## Step 2: Create `lok/filter_types_test.go`

```go
//go:build linux || darwin

package lok

import (
	"errors"
	"testing"
)

func TestFilterTypes(t *testing.T) {
	fb := &fakeBackend{filterTypesResult: `{"writer":"writer8"}`}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()

	got, err := o.FilterTypes()
	if err != nil {
		t.Fatal(err)
	}
	if got != `{"writer":"writer8"}` {
		t.Errorf("got %q", got)
	}
}

func TestFilterTypes_Closed(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	o.Close()
	_, err := o.FilterTypes()
	if !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestFilterTypes_BackendError(t *testing.T) {
	fb := &fakeBackend{filterTypesErr: ErrUnsupported}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	_, err := o.FilterTypes()
	if !errors.Is(err, ErrUnsupported) {
		t.Errorf("want ErrUnsupported, got %v", err)
	}
}
```

- [ ] Create file and verify `go test ./lok -run 'TestFilterTypes' -v`

## Step 3: Full build + test

```bash
go build ./...
go test ./lok -race -count=1
```

- [ ] Builds and tests pass
