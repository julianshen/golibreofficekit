//go:build linux || darwin

package lok

import (
	"errors"
	"log"
	"sync"
	"sync/atomic"
)

// listenerBufferSize is the per-listenerSet buffered channel
// capacity. Drop-newest applies once it overflows. 256 covers normal
// interactive event rates from a single document.
const listenerBufferSize = 256

// errNilListener is returned by addChecked when the caller passes a
// nil callback. The public AddListener wraps this in a *LOKError.
var errNilListener = errors.New("lok: listener callback is nil")

// listenerEntry pairs a callback with a unique id so cancel() can
// remove the right slot without comparing function values (Go funcs
// aren't comparable).
type listenerEntry struct {
	id uint64
	cb func(Event)
}

// listenerSet is the shared dispatcher type used by Office and
// Document. It implements lokc.Dispatcher so the cgo trampoline can
// route events into it.
type listenerSet struct {
	mu        sync.Mutex
	listeners []listenerEntry
	nextID    uint64
	ch        chan Event
	dropped   atomic.Uint64
	panicked  atomic.Uint64
	closed    atomic.Bool
	closeOnce sync.Once
	stopCh    chan struct{}
	done      chan struct{}
}

// newListenerSet starts a dispatcher goroutine and returns the set.
// Always paired with close() at end-of-life.
func newListenerSet() *listenerSet {
	ls := &listenerSet{
		ch:     make(chan Event, listenerBufferSize),
		stopCh: make(chan struct{}),
		done:   make(chan struct{}),
	}
	go ls.run()
	return ls
}

// add appends cb to the listener slice and returns a cancel closure.
// The closure is idempotent. cb must not be nil — use addChecked for
// the public path.
func (ls *listenerSet) add(cb func(Event)) func() {
	ls.mu.Lock()
	id := ls.nextID + 1
	ls.nextID = id
	ls.listeners = append(ls.listeners, listenerEntry{id: id, cb: cb})
	ls.mu.Unlock()
	cancelled := false
	var cancelMu sync.Mutex
	return func() {
		cancelMu.Lock()
		defer cancelMu.Unlock()
		if cancelled {
			return
		}
		cancelled = true
		ls.mu.Lock()
		defer ls.mu.Unlock()
		for i, e := range ls.listeners {
			if e.id == id {
				ls.listeners = append(ls.listeners[:i], ls.listeners[i+1:]...)
				return
			}
		}
	}
}

// addChecked is the public-API entrypoint that rejects nil cb.
func (ls *listenerSet) addChecked(cb func(Event)) (func(), error) {
	if cb == nil {
		return nil, errNilListener
	}
	return ls.add(cb), nil
}

// Dispatch implements lokc.Dispatcher. Called from the //export
// trampoline on LOK's thread; it must not block.
//
// LOK can fire callbacks on its own thread while another goroutine is
// closing the set, so the channel is never closed by the sender side.
// Closing flips an atomic flag and signals stopCh; Dispatch drops
// post-close events silently to keep the trampoline panic-free.
func (ls *listenerSet) Dispatch(typ int, payload []byte) {
	if ls.closed.Load() {
		return
	}
	select {
	case ls.ch <- Event{Type: EventType(typ), Payload: payload}:
	default:
		ls.dropped.Add(1)
	}
}

// Dropped returns the cumulative dropped-event count.
func (ls *listenerSet) Dropped() uint64 { return ls.dropped.Load() }

// Panicked returns the cumulative count of listener-callback panics
// the dispatcher recovered from.
func (ls *listenerSet) Panicked() uint64 { return ls.panicked.Load() }

// run is the dispatcher goroutine. It exits when stopCh is closed.
func (ls *listenerSet) run() {
	defer close(ls.done)
	for {
		select {
		case <-ls.stopCh:
			return
		case ev := <-ls.ch:
			ls.mu.Lock()
			// Snapshot the listener slice so a cancel during dispatch
			// doesn't race with iteration.
			snap := make([]listenerEntry, len(ls.listeners))
			copy(snap, ls.listeners)
			ls.mu.Unlock()
			for _, e := range snap {
				ls.runOne(e.cb, ev)
			}
		}
	}
}

// runOne invokes cb with panic recovery so a bad listener can't take
// down the dispatcher goroutine. Other listeners in the same
// dispatch slice still run.
func (ls *listenerSet) runOne(cb func(Event), ev Event) {
	defer func() {
		if r := recover(); r != nil {
			ls.panicked.Add(1)
			log.Printf("lok: listener panic: %v", r)
		}
	}()
	cb(ev)
}

// close signals the dispatcher to exit and waits for it. Idempotent.
// The event channel is intentionally not closed: the LOK trampoline
// can race with this call and any post-close Dispatch must not panic.
func (ls *listenerSet) close() {
	ls.closeOnce.Do(func() {
		ls.closed.Store(true)
		close(ls.stopCh)
	})
	<-ls.done
}
