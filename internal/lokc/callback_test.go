//go:build linux || darwin

package lokc

import (
	"sync"
	"testing"
)

type fakeDispatcher struct {
	mu       sync.Mutex
	received []Event
}

type Event struct {
	Type    int
	Payload []byte
}

func (f *fakeDispatcher) Dispatch(typ int, payload []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.received = append(f.received, Event{Type: typ, Payload: append([]byte(nil), payload...)})
}

func TestRegisterDispatcher_AssignsUniqueHandles(t *testing.T) {
	d1 := &fakeDispatcher{}
	d2 := &fakeDispatcher{}
	h1 := RegisterDispatcher(d1)
	h2 := RegisterDispatcher(d2)
	t.Cleanup(func() { UnregisterDispatcher(h1); UnregisterDispatcher(h2) })

	if h1 == 0 || h2 == 0 {
		t.Errorf("handles must be non-zero (0 reserved): h1=%d h2=%d", h1, h2)
	}
	if h1 == h2 {
		t.Errorf("handles must be unique: %d == %d", h1, h2)
	}
}

func TestLookupDispatcher_ReturnsRegistered(t *testing.T) {
	d := &fakeDispatcher{}
	h := RegisterDispatcher(d)
	t.Cleanup(func() { UnregisterDispatcher(h) })

	got := lookupDispatcher(h)
	if got != Dispatcher(d) {
		t.Errorf("lookupDispatcher(%d) = %v, want %v", h, got, d)
	}
}

func TestLookupDispatcher_ZeroHandleReturnsNil(t *testing.T) {
	if got := lookupDispatcher(0); got != nil {
		t.Errorf("lookupDispatcher(0) = %v, want nil", got)
	}
}

func TestUnregisterDispatcher_RemovesEntry(t *testing.T) {
	d := &fakeDispatcher{}
	h := RegisterDispatcher(d)
	UnregisterDispatcher(h)
	if got := lookupDispatcher(h); got != nil {
		t.Errorf("after Unregister: lookupDispatcher(%d) = %v, want nil", h, got)
	}
}

func TestRegisterDispatcher_ConcurrentSafe(t *testing.T) {
	const n = 100
	var wg sync.WaitGroup
	handles := make([]dispatchHandle, n)
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			handles[i] = RegisterDispatcher(&fakeDispatcher{})
		}(i)
	}
	wg.Wait()
	t.Cleanup(func() {
		for _, h := range handles {
			UnregisterDispatcher(h)
		}
	})

	// All n handles must be distinct.
	seen := map[dispatchHandle]bool{}
	for _, h := range handles {
		if seen[h] {
			t.Errorf("duplicate handle %d", h)
		}
		seen[h] = true
	}
	if len(seen) != n {
		t.Errorf("got %d unique handles, want %d", len(seen), n)
	}
}
