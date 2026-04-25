//go:build linux || darwin

package lok

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/julianshen/golibreofficekit/internal/lokc"
)

func TestListenerSet_DispatchInvokesAllListeners(t *testing.T) {
	ls := newListenerSet()
	defer ls.close()

	var aGot, bGot atomic.Int32
	ls.add(func(e Event) {
		if e.Type == EventTypeTextSelection {
			aGot.Add(1)
		}
	})
	ls.add(func(e Event) {
		if e.Type == EventTypeTextSelection {
			bGot.Add(1)
		}
	})

	ls.Dispatch(int(EventTypeTextSelection), []byte("hello"))

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if aGot.Load() == 1 && bGot.Load() == 1 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Errorf("got a=%d b=%d, want 1+1", aGot.Load(), bGot.Load())
}

func TestListenerSet_CancelStopsDelivery(t *testing.T) {
	ls := newListenerSet()
	defer ls.close()

	var got atomic.Int32
	cancel := ls.add(func(e Event) { got.Add(1) })
	ls.Dispatch(int(EventTypeTextSelection), nil)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if got.Load() == 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got.Load() != 1 {
		t.Fatalf("first dispatch never delivered: got=%d", got.Load())
	}

	cancel()
	cancel() // idempotent
	ls.Dispatch(int(EventTypeTextSelection), nil)
	time.Sleep(50 * time.Millisecond)
	if got.Load() != 1 {
		t.Errorf("after cancel: got=%d, want 1 (no new deliveries)", got.Load())
	}
}

func TestListenerSet_DropOnFull(t *testing.T) {
	ls := newListenerSet()
	defer ls.close()

	block := make(chan struct{})
	release := make(chan struct{})
	ls.add(func(e Event) {
		block <- struct{}{}
		<-release
	})

	ls.Dispatch(0, nil)
	<-block

	for range listenerBufferSize {
		ls.Dispatch(0, nil)
	}
	ls.Dispatch(0, nil)
	if got := ls.Dropped(); got == 0 {
		t.Errorf("Dropped()=0 after over-saturation, want >= 1")
	}

	// Drain block in the background so the dispatcher can finish
	// processing the buffered events before close() is called by defer.
	// The goroutine exits when block is closed after ls.done signals.
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-block:
			case <-stop:
				return
			}
		}
	}()
	close(release)
	// Wait for the dispatcher to drain, then stop the drainer.
	ls.close()
	close(stop)
}

func TestListenerSet_PanicRecovered(t *testing.T) {
	ls := newListenerSet()
	defer ls.close()

	var afterPanic atomic.Int32
	ls.add(func(e Event) { panic("boom") })
	ls.add(func(e Event) { afterPanic.Add(1) })

	ls.Dispatch(0, nil)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if afterPanic.Load() == 1 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Errorf("listener after panic never ran: %d", afterPanic.Load())
}

func TestListenerSet_AddNilReturnsError(t *testing.T) {
	ls := newListenerSet()
	defer ls.close()
	if _, err := ls.addChecked(nil); err == nil {
		t.Errorf("addChecked(nil) returned no error")
	}
}

func TestListenerSet_CloseJoinsDispatcher(t *testing.T) {
	ls := newListenerSet()
	done := make(chan struct{})
	ls.add(func(e Event) {})
	ls.Dispatch(0, nil)

	go func() {
		ls.close()
		close(done)
	}()

	select {
	case <-done:
		// dispatcher exited cleanly
	case <-time.After(time.Second):
		t.Errorf("close() did not return within 1s — dispatcher leak?")
	}
	ls.close() // idempotent
}

// dispatchOfficeFromFake walks back through the lokc handle table
// to the listenerSet the fake handed off when New() called
// RegisterOfficeCallback. Used by tests to simulate trampoline
// firing without going through the real cgo path.
func dispatchOfficeFromFake(t *testing.T, fb *fakeBackend) *listenerSet {
	t.Helper()
	if fb.lastOfficeCallbackHandle == 0 {
		t.Fatalf("fakeBackend never received RegisterOfficeCallback")
	}
	d := lokc.LookupDispatcherForTest(fb.lastOfficeCallbackHandle)
	if d == nil {
		t.Fatalf("no dispatcher registered under handle %d", fb.lastOfficeCallbackHandle)
	}
	ls, ok := d.(*listenerSet)
	if !ok {
		t.Fatalf("dispatcher is %T, not *listenerSet", d)
	}
	return ls
}

func TestOffice_AddListener_DeliversEvent(t *testing.T) {
	fb := &fakeBackend{}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()

	got := make(chan Event, 1)
	cancel, err := o.AddListener(func(e Event) {
		select {
		case got <- e:
		default:
		}
	})
	if err != nil {
		t.Fatalf("AddListener: %v", err)
	}
	defer cancel()

	dispatchOfficeFromFake(t, fb).Dispatch(int(EventTypeStateChanged), []byte(".uno:Bold=true"))

	select {
	case e := <-got:
		if e.Type != EventTypeStateChanged || string(e.Payload) != ".uno:Bold=true" {
			t.Errorf("got %+v", e)
		}
	case <-time.After(time.Second):
		t.Fatalf("listener never received event")
	}
}

func TestOffice_AddListener_AfterCloseErrors(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	o.Close()
	if _, err := o.AddListener(func(Event) {}); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestOffice_AddListener_NilCallback(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	if _, err := o.AddListener(nil); !errors.Is(err, ErrInvalidOption) {
		t.Errorf("want ErrInvalidOption, got %v", err)
	}
}

func TestOffice_DroppedEvents_StartsAtZero(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	if got := o.DroppedEvents(); got != 0 {
		t.Errorf("DroppedEvents()=%d, want 0", got)
	}
}

var _ = sync.Mutex{}
