package lok

import (
	"errors"
	"sync"
	"testing"
)

// fakeBackend is the in-memory test double.
type fakeBackend struct {
	mu       sync.Mutex
	openErr  error
	hookErr  error
	version  string
	destroys int
}

type fakeLib struct{}

func (fakeLib) libraryBrand() {}

type fakeOffice struct {
	be *fakeBackend
}

func (*fakeOffice) officeBrand() {}

func (f *fakeBackend) OpenLibrary(path string) (libraryHandle, error) {
	if f.openErr != nil {
		return nil, f.openErr
	}
	return fakeLib{}, nil
}

func (f *fakeBackend) InvokeHook(lib libraryHandle, _ string) (officeHandle, error) {
	if f.hookErr != nil {
		return nil, f.hookErr
	}
	return &fakeOffice{be: f}, nil
}
func (f *fakeBackend) OfficeGetError(officeHandle) string                      { return "" }
func (f *fakeBackend) OfficeGetVersionInfo(officeHandle) string                { return f.version }
func (f *fakeBackend) OfficeSetOptionalFeatures(officeHandle, uint64)          {}
func (f *fakeBackend) OfficeSetDocumentPassword(officeHandle, string, string)  {}
func (f *fakeBackend) OfficeSetAuthor(officeHandle, string)                    {}
func (f *fakeBackend) OfficeDumpState(officeHandle) string                     { return "state" }
func (f *fakeBackend) OfficeTrimMemory(officeHandle, int)                      {}
func (f *fakeBackend) OfficeDestroy(officeHandle) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.destroys++
}

// withFakeBackend swaps the package-level backend + singleton. It
// mutates globals (currentBackend, live), so tests using it must NOT
// call t.Parallel() — that would race on those globals.
func withFakeBackend(t *testing.T, f *fakeBackend) {
	t.Helper()
	orig := currentBackend
	t.Cleanup(func() { setBackend(orig); resetSingleton() })
	setBackend(f)
	resetSingleton()
}

func TestNew_EmptyPathErrors(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	_, err := New("")
	if !errors.Is(err, ErrInstallPathRequired) {
		t.Fatalf("want ErrInstallPathRequired, got %v", err)
	}
}

func TestNew_Singleton(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, err := New("/install")
	if err != nil {
		t.Fatalf("first New: %v", err)
	}
	defer o.Close()

	_, err = New("/install")
	if !errors.Is(err, ErrAlreadyInitialised) {
		t.Errorf("second New: want ErrAlreadyInitialised, got %v", err)
	}
}

func TestNew_AfterCloseSucceeds(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, err := New("/install")
	if err != nil {
		t.Fatalf("first New: %v", err)
	}
	if err := o.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	o2, err := New("/install")
	if err != nil {
		t.Fatalf("second New after Close: %v", err)
	}
	o2.Close()
}

func TestNew_OpenLibraryError(t *testing.T) {
	customErr := errors.New("synthetic open failure")
	withFakeBackend(t, &fakeBackend{openErr: customErr})
	_, err := New("/install")
	if !errors.Is(err, customErr) {
		t.Errorf("want synthetic err, got %v", err)
	}
}

func TestNew_HookError(t *testing.T) {
	customErr := errors.New("synthetic hook failure")
	withFakeBackend(t, &fakeBackend{hookErr: customErr})
	_, err := New("/install")
	if !errors.Is(err, customErr) {
		t.Errorf("want synthetic err, got %v", err)
	}
}

func TestClose_Idempotent(t *testing.T) {
	fb := &fakeBackend{}
	withFakeBackend(t, fb)
	o, err := New("/install")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := o.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := o.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
	if fb.destroys != 1 {
		t.Errorf("destroys: want 1, got %d", fb.destroys)
	}
}
