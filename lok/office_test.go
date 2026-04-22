package lok

import (
	"errors"
	"slices"
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

	// Captured call arguments. Not mutex-guarded because
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

	// View state: monotonic IDs starting at 1000 to stay visually
	// distinct from real LO view IDs (which start at 0) in test
	// output.
	viewsNextID      int
	viewsLive        []int
	viewActive       int
	viewCreateErr    bool // if true, CreateView returns -1
	lastViewOptions  string
	lastViewLang     string
	lastViewLangID   int
	lastViewReadOnly bool
	lastViewA11y     bool
	lastViewTimezone string
}

const fakeViewIDBase = 1000

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

type fakeDoc struct{}

func (*fakeDoc) documentBrand() {}

func (f *fakeBackend) DocumentLoad(_ officeHandle, url string) (documentHandle, error) {
	if f.loadErr != nil {
		return nil, f.loadErr
	}
	f.lastLoadURL = url
	return &fakeDoc{}, nil
}

func (f *fakeBackend) DocumentLoadWithOptions(_ officeHandle, url, opts string) (documentHandle, error) {
	if f.loadErr != nil {
		return nil, f.loadErr
	}
	f.lastLoadURL = url
	f.lastLoadOpts = opts
	return &fakeDoc{}, nil
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
	// concurrent access is a programmer bug.
	f.docDestroys++
}

func (f *fakeBackend) DocumentCreateView(documentHandle) int {
	if f.viewCreateErr {
		return -1
	}
	if f.viewsNextID == 0 {
		f.viewsNextID = fakeViewIDBase
	}
	id := f.viewsNextID
	f.viewsNextID++
	f.viewsLive = append(f.viewsLive, id)
	f.viewActive = id
	return id
}

func (f *fakeBackend) DocumentCreateViewWithOptions(d documentHandle, opts string) int {
	f.lastViewOptions = opts
	return f.DocumentCreateView(d)
}

func (f *fakeBackend) DocumentDestroyView(_ documentHandle, id int) {
	for i, v := range f.viewsLive {
		if v == id {
			f.viewsLive = append(f.viewsLive[:i], f.viewsLive[i+1:]...)
			break
		}
	}
	// Active-view fallback: fake-only convention. Real LOK's
	// getView() behaviour after destroying the current view is
	// undocumented (likely returns a stale ID or falls back to the
	// remaining default view). For the fake, pick a deterministic
	// successor so tests can assert View()/SetView() interactions.
	if f.viewActive == id && len(f.viewsLive) > 0 {
		f.viewActive = f.viewsLive[0]
	} else if f.viewActive == id {
		f.viewActive = -1
	}
}

// DocumentSetView silently ignores an unknown ID — matching real LOK
// which returns void and gives no failure signal, but constraining
// the fake to live IDs catches "destroyed-view regressions" in
// tests before the ID escapes into getView().
func (f *fakeBackend) DocumentSetView(_ documentHandle, id int) {
	if slices.Contains(f.viewsLive, id) {
		f.viewActive = id
	}
}

// DocumentGetView returns -1 when no views are live rather than the
// zero value of viewActive, matching the "no active view" signal
// lok.View() checks for.
func (f *fakeBackend) DocumentGetView(documentHandle) int {
	if len(f.viewsLive) == 0 {
		return -1
	}
	return f.viewActive
}

func (f *fakeBackend) DocumentGetViewsCount(documentHandle) int { return len(f.viewsLive) }

func (f *fakeBackend) DocumentGetViewIds(documentHandle) ([]int, bool) {
	if len(f.viewsLive) == 0 {
		return nil, true
	}
	out := make([]int, len(f.viewsLive))
	copy(out, f.viewsLive)
	return out, true
}

func (f *fakeBackend) DocumentSetViewLanguage(_ documentHandle, id int, lang string) {
	f.lastViewLangID = id
	f.lastViewLang = lang
}

func (f *fakeBackend) DocumentSetViewReadOnly(_ documentHandle, _ int, ro bool) {
	f.lastViewReadOnly = ro
}

func (f *fakeBackend) DocumentSetAccessibilityState(_ documentHandle, _ int, en bool) {
	f.lastViewA11y = en
}

func (f *fakeBackend) DocumentSetViewTimezone(_ documentHandle, _ int, tz string) {
	f.lastViewTimezone = tz
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
