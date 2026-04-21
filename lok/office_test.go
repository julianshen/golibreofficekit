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

	// Capture fields for Task 7 methods. Not mutex-guarded because
	// withFakeBackend forbids t.Parallel().
	lastAuthor      string
	lastTrimTarget  int
	dumpStateOut    string
	lastPwdURL      string
	lastPwdPassword string

	loadErr      error
	saveErr      error
	lastLoadURL  string
	lastLoadOpts string
	lastSaveURL  string
	lastSaveFmt  string
	lastSaveOpts string
	docDestroys  int
	docType      int // returned by DocumentGetType
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
func (f *fakeBackend) OfficeGetError(officeHandle) string             { return "" }
func (f *fakeBackend) OfficeGetVersionInfo(officeHandle) string       { return f.version }
func (f *fakeBackend) OfficeSetOptionalFeatures(officeHandle, uint64) {}
func (f *fakeBackend) OfficeSetAuthor(_ officeHandle, s string)       { f.lastAuthor = s }
func (f *fakeBackend) OfficeTrimMemory(_ officeHandle, n int)         { f.lastTrimTarget = n }
func (f *fakeBackend) OfficeDumpState(_ officeHandle) string          { return f.dumpStateOut }
func (f *fakeBackend) OfficeSetDocumentPassword(_ officeHandle, url, pwd string) {
	f.lastPwdURL = url
	f.lastPwdPassword = pwd
}
func (f *fakeBackend) OfficeDestroy(officeHandle) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.destroys++
}

type fakeDoc struct {
	be  *fakeBackend
	url string
}

func (*fakeDoc) documentBrand() {}

func (f *fakeBackend) DocumentLoad(_ officeHandle, url string) (documentHandle, error) {
	if f.loadErr != nil {
		return nil, f.loadErr
	}
	f.lastLoadURL = url
	return &fakeDoc{be: f, url: url}, nil
}

func (f *fakeBackend) DocumentLoadWithOptions(_ officeHandle, url, opts string) (documentHandle, error) {
	if f.loadErr != nil {
		return nil, f.loadErr
	}
	f.lastLoadURL = url
	f.lastLoadOpts = opts
	return &fakeDoc{be: f, url: url}, nil
}

func (f *fakeBackend) DocumentGetType(documentHandle) int { return f.docType }

func (f *fakeBackend) DocumentSaveAs(d documentHandle, url, format, opts string) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.lastSaveURL = url
	f.lastSaveFmt = format
	f.lastSaveOpts = opts
	return nil
}

func (f *fakeBackend) DocumentDestroy(documentHandle) {
	// Not mutex-guarded: withFakeBackend forbids t.Parallel() so
	// concurrent access is a programmer bug, consistent with the
	// other capture fields added in Phase 2.
	f.docDestroys++
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

func TestSetAuthor_Records(t *testing.T) {
	fb := &fakeBackend{}
	withFakeBackend(t, fb)
	o, err := New("/install")
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	if err := o.SetAuthor("Jane Doe"); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	if fb.lastAuthor != "Jane Doe" {
		t.Errorf("recorded %q, want Jane Doe", fb.lastAuthor)
	}
}

func TestTrimMemory_PassesTarget(t *testing.T) {
	fb := &fakeBackend{}
	withFakeBackend(t, fb)
	o, err := New("/install")
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	if err := o.TrimMemory(42); err != nil {
		t.Fatalf("TrimMemory: %v", err)
	}
	if fb.lastTrimTarget != 42 {
		t.Errorf("recorded %d, want 42", fb.lastTrimTarget)
	}
}

func TestDumpState_ReturnsBackendString(t *testing.T) {
	fb := &fakeBackend{dumpStateOut: "snapshot-xyz"}
	withFakeBackend(t, fb)
	o, err := New("/install")
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	got, err := o.DumpState()
	if err != nil {
		t.Fatalf("DumpState: %v", err)
	}
	if got != "snapshot-xyz" {
		t.Errorf("DumpState=%q, want snapshot-xyz", got)
	}
}

func TestSetDocumentPassword_PassesCredentials(t *testing.T) {
	fb := &fakeBackend{}
	withFakeBackend(t, fb)
	o, err := New("/install")
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	if err := o.SetDocumentPassword("file:///tmp/x.odt", "hunter2"); err != nil {
		t.Fatalf("SetDocumentPassword: %v", err)
	}
	if fb.lastPwdURL != "file:///tmp/x.odt" || fb.lastPwdPassword != "hunter2" {
		t.Errorf("recorded (url=%q pwd=%q)", fb.lastPwdURL, fb.lastPwdPassword)
	}
}

func TestSetDocumentPassword_EmptyURLErrors(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, err := New("/install")
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	err = o.SetDocumentPassword("", "x")
	var lokErr *LOKError
	if !errors.As(err, &lokErr) || lokErr.Op != "SetDocumentPassword" {
		t.Errorf("want *LOKError Op=SetDocumentPassword, got %T %v", err, err)
	}
}

func TestRemainingMethods_AfterCloseErrors(t *testing.T) {
	cases := []struct {
		name string
		call func(*Office) error
	}{
		{"SetAuthor", func(o *Office) error { return o.SetAuthor("x") }},
		{"TrimMemory", func(o *Office) error { return o.TrimMemory(0) }},
		{"DumpState", func(o *Office) error { _, err := o.DumpState(); return err }},
		{"SetDocumentPassword", func(o *Office) error { return o.SetDocumentPassword("file:///x", "p") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withFakeBackend(t, &fakeBackend{})
			o, err := New("/install")
			if err != nil {
				t.Fatal(err)
			}
			o.Close()
			if err := tc.call(o); !errors.Is(err, ErrClosed) {
				t.Errorf("want ErrClosed, got %v", err)
			}
		})
	}
}
