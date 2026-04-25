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

// Event is a test-local type holding the arguments the trampoline
// delivered to Dispatch. It will be replaced by lok.Event in Task 4.
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

func TestGoLOKDispatchOffice_RoutesToHandle(t *testing.T) {
	d := &fakeDispatcher{}
	h := RegisterDispatcher(d)
	t.Cleanup(func() { UnregisterDispatcher(h) })

	s := "hello"
	testDispatchOffice(2, &s, h)

	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.received) != 1 {
		t.Fatalf("got %d events, want 1", len(d.received))
	}
	if d.received[0].Type != 2 || string(d.received[0].Payload) != "hello" {
		t.Errorf("got %+v, want type=2 payload=hello", d.received[0])
	}
}

func TestGoLOKDispatchDocument_RoutesToHandle(t *testing.T) {
	d := &fakeDispatcher{}
	h := RegisterDispatcher(d)
	t.Cleanup(func() { UnregisterDispatcher(h) })

	s := "world"
	testDispatchDocument(8, &s, h)

	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.received) != 1 || d.received[0].Type != 8 || string(d.received[0].Payload) != "world" {
		t.Errorf("got %+v", d.received)
	}
}

func TestGoLOKDispatch_NULLPayloadGivesNilSlice(t *testing.T) {
	d := &fakeDispatcher{}
	h := RegisterDispatcher(d)
	t.Cleanup(func() { UnregisterDispatcher(h) })

	testDispatchOffice(0, nil, h)

	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.received) != 1 || d.received[0].Payload != nil {
		t.Errorf("got %+v, want one event with nil payload", d.received)
	}
}

func TestGoLOKDispatchDocument_NULLPayloadGivesNilSlice(t *testing.T) {
	d := &fakeDispatcher{}
	h := RegisterDispatcher(d)
	t.Cleanup(func() { UnregisterDispatcher(h) })

	testDispatchDocument(0, nil, h)

	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.received) != 1 || d.received[0].Payload != nil {
		t.Errorf("got %+v, want one event with nil payload", d.received)
	}
}

func TestGoLOKDispatch_UnknownHandleNoOp(t *testing.T) {
	// Use handle=0 (reserved) and an arbitrary unregistered value.
	testDispatchOffice(2, nil, dispatchHandle(0))
	testDispatchOffice(2, nil, dispatchHandle(99999))
	// Survives without panic. No assertion needed.
}

func TestGoLOKDispatch_LongPayloadRoundTrip(t *testing.T) {
	d := &fakeDispatcher{}
	h := RegisterDispatcher(d)
	t.Cleanup(func() { UnregisterDispatcher(h) })

	want := make([]byte, 65*1024)
	for i := range want {
		want[i] = byte('a' + i%26)
	}
	s := string(want)
	testDispatchOffice(0, &s, h)

	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.received) != 1 || string(d.received[0].Payload) != string(want) {
		t.Errorf("payload round-trip failed; len got=%d want=%d", len(d.received[0].Payload), len(want))
	}
}

func TestRegisterOfficeCallback_NilSafe(t *testing.T) {
	// Without an OfficeHandle helper we can only test the invalid
	// path here. The success path is covered by realBackend tests
	// against a fake document handle in lok/real_backend_test.go.
	if err := RegisterOfficeCallback(OfficeHandle{}, dispatchHandle(1)); err != ErrUnsupported {
		t.Errorf("zero handle: err=%v, want ErrUnsupported", err)
	}
}

func TestRegisterDocumentCallback_NilSafe(t *testing.T) {
	if err := RegisterDocumentCallback(DocumentHandle{}, dispatchHandle(1)); err != ErrUnsupported {
		t.Errorf("zero handle: err=%v, want ErrUnsupported", err)
	}
	if err := RegisterDocumentCallback(newFakeDoc(t), dispatchHandle(1)); err != ErrUnsupported {
		t.Errorf("nil pClass: err=%v, want ErrUnsupported", err)
	}
}
