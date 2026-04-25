//go:build linux || darwin

package lok

import (
	"runtime"
	"sync"

	"github.com/julianshen/golibreofficekit/internal/lokc"
)

// currentBackend is swapped in tests; production sets it via init().
var (
	backendMu      sync.Mutex
	currentBackend backend
)

func setBackend(b backend) {
	backendMu.Lock()
	defer backendMu.Unlock()
	currentBackend = b
}

// Office is the LibreOffice process. It is safe to use from multiple
// goroutines; calls serialise on an internal mutex.
type Office struct {
	mu        sync.Mutex
	be        backend
	h         officeHandle
	closed    bool
	listeners *listenerSet
	listenerH uintptr // dispatch handle in the lokc handle table
}

// Singleton state.
var (
	singletonMu sync.Mutex
	live        *Office
)

// resetSingleton exists for tests; production paths use Close.
func resetSingleton() {
	singletonMu.Lock()
	live = nil
	singletonMu.Unlock()
}

// New loads LibreOffice from installPath and returns the single
// *Office for this process. A second New while a previous *Office is
// live returns ErrAlreadyInitialised.
func New(installPath string, opts ...Option) (*Office, error) {
	if installPath == "" {
		return nil, ErrInstallPathRequired
	}

	singletonMu.Lock()
	defer singletonMu.Unlock()
	if live != nil {
		return nil, ErrAlreadyInitialised
	}

	options := buildOptions(opts)

	backendMu.Lock()
	be := currentBackend
	backendMu.Unlock()
	if be == nil {
		be = realBackend{}
	}

	// Pin to the OS thread for the hook call (LO's internal init
	// installs thread-local state). A single defer handles every
	// exit path including panic.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	lib, err := be.OpenLibrary(installPath)
	if err != nil {
		return nil, &LOKError{Op: "OpenLibrary", Detail: err.Error(), err: err}
	}
	h, err := be.InvokeHook(lib, options.userProfileURL)
	if err != nil {
		return nil, &LOKError{Op: "InvokeHook", Detail: err.Error(), err: err}
	}

	o := &Office{be: be, h: h}
	o.listeners = newListenerSet()
	o.listenerH = lokc.RegisterDispatcherUintptr(o.listeners)
	if regErr := be.RegisterOfficeCallback(h, o.listenerH); regErr != nil {
		// Reap the dispatcher goroutine before returning the error.
		lokc.UnregisterDispatcherUintptr(o.listenerH)
		o.listeners.close()
		be.OfficeDestroy(h)
		return nil, &LOKError{Op: "RegisterOfficeCallback", Detail: regErr.Error(), err: regErr}
	}
	live = o
	return o, nil
}

// Close destroys the LOK office and clears the singleton. Safe to
// call multiple times; only the first invocation hits LOK.
func (o *Office) Close() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return nil
	}
	o.closed = true
	lokc.UnregisterDispatcherUintptr(o.listenerH)
	o.listeners.close()
	o.be.OfficeDestroy(o.h)

	singletonMu.Lock()
	if live == o {
		live = nil
	}
	singletonMu.Unlock()
	return nil
}

// Option configures New.
type Option func(*options)

type options struct {
	userProfileURL string
}

// WithUserProfile sets the user-profile URL passed to
// libreofficekit_hook_2. Empty string uses LO's default location.
func WithUserProfile(url string) Option {
	return func(o *options) { o.userProfileURL = url }
}

func buildOptions(opts []Option) options {
	var o options
	for _, fn := range opts {
		fn(&o)
	}
	return o
}

// SetAuthor sets LibreOffice's author metadata via setOption("Author", ...).
// Returns ErrClosed if the Office has been closed.
func (o *Office) SetAuthor(author string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return ErrClosed
	}
	o.be.OfficeSetAuthor(o.h, author)
	return nil
}

// TrimMemory forwards to LOK's trimMemory with the caller's target
// level. Returns ErrClosed if the Office has been closed.
func (o *Office) TrimMemory(target int) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return ErrClosed
	}
	o.be.OfficeTrimMemory(o.h, target)
	return nil
}

// DumpState returns LO's internal state dump. Returns ErrClosed if
// the Office has been closed.
func (o *Office) DumpState() (string, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return "", ErrClosed
	}
	return o.be.OfficeDumpState(o.h), nil
}

// AddListener registers cb to receive office-wide events. See
// listener.go for the dispatch contract. Returns ErrClosed if the
// Office has been closed and ErrInvalidOption if cb is nil.
func (o *Office) AddListener(cb func(Event)) (cancel func(), err error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return nil, ErrClosed
	}
	c, addErr := o.listeners.addChecked(cb)
	if addErr != nil {
		return nil, &LOKError{Op: "AddListener", Detail: addErr.Error(), err: ErrInvalidOption}
	}
	return c, nil
}

// DroppedEvents returns the cumulative count of office-level events
// the dispatcher dropped because the buffer was full.
func (o *Office) DroppedEvents() uint64 {
	return o.listeners.Dropped()
}

// SetDocumentPassword preloads the password for a document URL that
// will be opened next. Returns ErrClosed if the Office has been
// closed and *LOKError with Op="SetDocumentPassword" if url is empty.
func (o *Office) SetDocumentPassword(url, password string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return ErrClosed
	}
	if url == "" {
		return &LOKError{Op: "SetDocumentPassword", Detail: "url is required"}
	}
	o.be.OfficeSetDocumentPassword(o.h, url, password)
	return nil
}
