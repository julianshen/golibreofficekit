package lok

import (
	"errors"
	"slices"
	"sync"
	"testing"
)

type fakePaint struct {
	pxW, pxH, x, y, w, h int
	bufLen               int
}
type fakePartPaint struct {
	part, mode, pxW, pxH, x, y, w, h int
	bufLen                           int
}

// fakeBackend is the in-memory test double.
type fakeBackend struct {
	mu       sync.Mutex
	openErr  error
	hookErr  error
	version  string
	destroys int

	// officeError is what OfficeGetError returns. Tests that exercise
	// the load/save error-detail wiring populate this with a known
	// LO-side message.
	officeError string

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

	// Part state. partsCount convention: -1 = simulate LOK backend
	// failure (matches internal/lokc's return-on-NULL-pClass);
	// 0+ = real part count. Fresh `&fakeBackend{}` is 0-part.
	partsCount     int
	partActive     int
	partNames      map[int]string
	partHashes     map[int]string
	partInfos      map[int]string
	partRects      string
	docWidthTwips  int64
	docHeightTwips int64
	lastPartMode   int

	lastOutlineCol    bool
	lastOutlineLevel  int
	lastOutlineIndex  int
	lastOutlineHidden bool

	// Render state.
	lastInitArgs    string
	tileMode        int // fakes can programme this; default 0
	lastZoom        [4]int
	lastVisibleArea [4]int
	paintCalls      []fakePaint
	partPaintCalls  []fakePartPaint
	searchResultBuf []byte
	searchResultPxW int
	searchResultPxH int
	searchResultOK  bool
	lastSearchQuery string
	shapeSelection  []byte

	// Input state.
	lastKeyType     int
	lastCharCode    int
	lastKeyCode     int
	lastMouseType   int
	lastMouseX      int
	lastMouseY      int
	lastMouseCount  int
	lastMouseButton int
	lastMouseMods   int
	lastUnoCmd      string
	lastUnoArgs     string
	lastUnoNotify   bool

	// Vtable-detect injection points (PR B): fakes assert that the
	// public lok methods propagate the lokc error untouched. Default
	// nil so existing tests stay unaffected.
	postKeyEventErr           error
	postMouseEventErr         error
	postUnoCommandErr         error
	destroyViewErr            error
	setViewErr                error
	setViewLanguageErr        error
	setViewReadOnlyErr        error
	setAccessibilityStateErr  error
	setViewTimezoneErr        error
	setPartErr                error
	setPartModeErr            error
	setOutlineStateErr        error
	initializeForRenderingErr error
	setClientZoomErr          error
	setClientVisibleAreaErr   error

	lastGetTextSelectionMime     string
	lastSelectionTypeAndTextMime string
	selectionText                string
	selectionUsedMime            string
	selectionKind                int
	getTextSelectionErr          error
	getSelectionTypeErr          error
	selectionTypeTextErr         error
	selectionSetterErr           error

	lastSetTextSelectionTyp int
	lastSetTextSelectionX   int
	lastSetTextSelectionY   int
	resetSelectionCalls     int
	lastSetGraphicTyp       int
	lastSetGraphicX         int
	lastSetGraphicY         int
	lastBlockedViewID       int
	lastBlockedCSV          string

	lastGetClipboardMimes []string
	getClipboardResult    []clipboardItemInternal
	getClipboardErr       error
	lastSetClipboardItems []clipboardItemInternal
	setClipboardErr       error

	// Callback registration (Phase 9).
	lastOfficeCallbackHandle   uintptr
	lastDocumentCallbackHandle uintptr
	registerOfficeCallbackErr  error
	registerDocCallbackErr     error

	// Phase 10: command/window tracking.
	lastCommand                string
	lastCommandResult          string
	getCommandValuesErr        error
	completeFunctionErr        error
	lastWindowID               uint32
	lastDialogWindowID         uint64
	lastDialogArgs             string
	lastContentControlArgs     string
	lastFormFieldArgs          string
	lastGestureType            string
	lastExtTextInputType       int
	lastExtTextInputText       string
	sendDialogEventErr         error
	sendContentControlEventErr error
	sendFormFieldEventErr      error

	// Phase 11: advanced + misc.
	lastMacroURL         string
	macroErr             error
	lastSignURL          string
	lastSignCert         []byte
	lastSignKey          []byte
	signErr              error
	filterTypesResult    string
	filterTypesErr       error
	lastInsertCert       []byte
	lastInsertKey        []byte
	insertCertErr        error
	lastAddCert          []byte
	addCertErr           error
	signatureStateResult int
	signatureStateErr    error
	lastPasteMime        string
	lastPasteData        []byte
	pasteErr             error
	lastSelectPart       int
	lastSelectSelected   bool
	selectPartErr        error
	lastMovePos          int
	lastMoveDup          bool
	moveSelectedPartsErr error
	lastRenderFontName   string
	lastRenderFontChar   string
	renderFontBuf        []byte
	renderFontW          int
	renderFontH          int
	renderFontErr        error
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
func (f *fakeBackend) OfficeGetError(officeHandle) string             { return f.officeError }
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
	// Record the call before honouring loadErr so error-path tests can
	// still assert which entry point was taken.
	f.lastLoadURL = url
	if f.loadErr != nil {
		return nil, f.loadErr
	}
	return &fakeDoc{}, nil
}

func (f *fakeBackend) DocumentLoadWithOptions(_ officeHandle, url, opts string) (documentHandle, error) {
	f.lastLoadURL = url
	f.lastLoadOpts = opts
	if f.loadErr != nil {
		return nil, f.loadErr
	}
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

func (f *fakeBackend) DocumentDestroyView(_ documentHandle, id int) error {
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
	return f.destroyViewErr
}

// DocumentSetView silently ignores an unknown ID — matching real LOK
// which returns void and gives no failure signal, but constraining
// the fake to live IDs catches "destroyed-view regressions" in
// tests before the ID escapes into getView().
func (f *fakeBackend) DocumentSetView(_ documentHandle, id int) error {
	if slices.Contains(f.viewsLive, id) {
		f.viewActive = id
	}
	return f.setViewErr
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

func (f *fakeBackend) DocumentSetViewLanguage(_ documentHandle, id int, lang string) error {
	f.lastViewLangID = id
	f.lastViewLang = lang
	return f.setViewLanguageErr
}

func (f *fakeBackend) DocumentSetViewReadOnly(_ documentHandle, _ int, ro bool) error {
	f.lastViewReadOnly = ro
	return f.setViewReadOnlyErr
}

func (f *fakeBackend) DocumentSetAccessibilityState(_ documentHandle, _ int, en bool) error {
	f.lastViewA11y = en
	return f.setAccessibilityStateErr
}

func (f *fakeBackend) DocumentSetViewTimezone(_ documentHandle, _ int, tz string) error {
	f.lastViewTimezone = tz
	return f.setViewTimezoneErr
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

func TestNew_RegisterOfficeCallbackError(t *testing.T) {
	synth := errors.New("synthetic register-callback failure")
	fb := &fakeBackend{registerOfficeCallbackErr: synth}
	withFakeBackend(t, fb)

	o, err := New("/install")
	if err == nil {
		t.Fatalf("New: want error, got nil; o=%v", o)
	}
	if o != nil {
		t.Errorf("New: want nil Office on failure, got %v", o)
	}
	if !errors.Is(err, synth) {
		t.Errorf("New: want wraps synthetic, got %v", err)
	}

	// Singleton must not be set — a second New must work.
	// Clear the error so the retry can succeed.
	fb.registerOfficeCallbackErr = nil
	o2, err2 := New("/install")
	if err2 != nil {
		t.Fatalf("second New: %v", err2)
	}
	defer o2.Close()
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

// --- Part / size fake methods ---

func (f *fakeBackend) DocumentGetParts(documentHandle) int { return f.partsCount }
func (f *fakeBackend) DocumentGetPart(documentHandle) int  { return f.partActive }

func (f *fakeBackend) DocumentSetPart(_ documentHandle, n int) error {
	if n >= 0 && n < f.partsCount {
		f.partActive = n
	}
	return f.setPartErr
}

func (f *fakeBackend) DocumentSetPartMode(_ documentHandle, mode int) error {
	f.lastPartMode = mode
	return f.setPartModeErr
}

func (f *fakeBackend) DocumentGetPartName(_ documentHandle, n int) string {
	return f.partNames[n]
}

func (f *fakeBackend) DocumentGetPartHash(_ documentHandle, n int) string {
	return f.partHashes[n]
}

func (f *fakeBackend) DocumentGetPartInfo(_ documentHandle, n int) string {
	return f.partInfos[n]
}

func (f *fakeBackend) DocumentGetPartPageRectangles(documentHandle) string {
	return f.partRects
}

func (f *fakeBackend) DocumentGetDocumentSize(documentHandle) (int64, int64) {
	return f.docWidthTwips, f.docHeightTwips
}

func (f *fakeBackend) DocumentSetOutlineState(_ documentHandle, column bool, level, index int, hidden bool) error {
	f.lastOutlineCol = column
	f.lastOutlineLevel = level
	f.lastOutlineIndex = index
	f.lastOutlineHidden = hidden
	return f.setOutlineStateErr
}

func (f *fakeBackend) DocumentInitializeForRendering(_ documentHandle, args string) error {
	f.lastInitArgs = args
	return f.initializeForRenderingErr
}
func (f *fakeBackend) DocumentGetTileMode(documentHandle) int { return f.tileMode }
func (f *fakeBackend) DocumentSetClientZoom(_ documentHandle, tpw, tph, ttw, tth int) error {
	f.lastZoom = [4]int{tpw, tph, ttw, tth}
	return f.setClientZoomErr
}
func (f *fakeBackend) DocumentSetClientVisibleArea(_ documentHandle, x, y, w, h int) error {
	f.lastVisibleArea = [4]int{x, y, w, h}
	return f.setClientVisibleAreaErr
}
func (f *fakeBackend) DocumentPaintTile(_ documentHandle, buf []byte, pxW, pxH, x, y, w, h int) {
	f.paintCalls = append(f.paintCalls, fakePaint{pxW: pxW, pxH: pxH, x: x, y: y, w: w, h: h, bufLen: len(buf)})
}
func (f *fakeBackend) DocumentPaintPartTile(_ documentHandle, buf []byte, part, mode, pxW, pxH, x, y, w, h int) {
	f.partPaintCalls = append(f.partPaintCalls, fakePartPaint{
		part: part, mode: mode, pxW: pxW, pxH: pxH, x: x, y: y, w: w, h: h, bufLen: len(buf),
	})
}
func (f *fakeBackend) DocumentRenderSearchResult(_ documentHandle, q string) ([]byte, int, int, bool) {
	f.lastSearchQuery = q
	return f.searchResultBuf, f.searchResultPxW, f.searchResultPxH, f.searchResultOK
}
func (f *fakeBackend) DocumentRenderShapeSelection(documentHandle) []byte {
	return f.shapeSelection
}

func (f *fakeBackend) DocumentPostKeyEvent(_ documentHandle, typ, charCode, keyCode int) error {
	f.lastKeyType = typ
	f.lastCharCode = charCode
	f.lastKeyCode = keyCode
	return f.postKeyEventErr
}
func (f *fakeBackend) DocumentPostMouseEvent(_ documentHandle, typ, x, y, count, buttons, mods int) error {
	f.lastMouseType = typ
	f.lastMouseX = x
	f.lastMouseY = y
	f.lastMouseCount = count
	f.lastMouseButton = buttons
	f.lastMouseMods = mods
	return f.postMouseEventErr
}
func (f *fakeBackend) DocumentPostUnoCommand(_ documentHandle, cmd, args string, notify bool) error {
	f.lastUnoCmd = cmd
	f.lastUnoArgs = args
	f.lastUnoNotify = notify
	return f.postUnoCommandErr
}

func (f *fakeBackend) DocumentSetTextSelection(_ documentHandle, typ, x, y int) error {
	f.lastSetTextSelectionTyp = typ
	f.lastSetTextSelectionX = x
	f.lastSetTextSelectionY = y
	return f.selectionSetterErr
}

func (f *fakeBackend) DocumentResetSelection(documentHandle) error {
	f.resetSelectionCalls++
	return f.selectionSetterErr
}

func (f *fakeBackend) DocumentSetGraphicSelection(_ documentHandle, typ, x, y int) error {
	f.lastSetGraphicTyp = typ
	f.lastSetGraphicX = x
	f.lastSetGraphicY = y
	return f.selectionSetterErr
}

func (f *fakeBackend) DocumentSetBlockedCommandList(_ documentHandle, viewID int, csv string) error {
	f.lastBlockedViewID = viewID
	f.lastBlockedCSV = csv
	return f.selectionSetterErr
}
func (f *fakeBackend) DocumentGetTextSelection(_ documentHandle, mime string) (string, string, error) {
	f.lastGetTextSelectionMime = mime
	if f.getTextSelectionErr != nil {
		return "", "", f.getTextSelectionErr
	}
	return f.selectionText, f.selectionUsedMime, nil
}

func (f *fakeBackend) DocumentGetSelectionType(documentHandle) (int, error) {
	if f.getSelectionTypeErr != nil {
		return 0, f.getSelectionTypeErr
	}
	return f.selectionKind, nil
}

func (f *fakeBackend) DocumentGetSelectionTypeAndText(_ documentHandle, mime string) (int, string, string, error) {
	f.lastSelectionTypeAndTextMime = mime
	if f.selectionTypeTextErr != nil {
		return -1, "", "", f.selectionTypeTextErr
	}
	return f.selectionKind, f.selectionText, f.selectionUsedMime, nil
}
func (f *fakeBackend) DocumentGetClipboard(_ documentHandle, mimes []string) ([]clipboardItemInternal, error) {
	// Record a copy so test mutations don't race with the fake.
	if mimes != nil {
		f.lastGetClipboardMimes = append([]string(nil), mimes...)
	} else {
		f.lastGetClipboardMimes = nil
	}
	if f.getClipboardErr != nil {
		return nil, f.getClipboardErr
	}
	out := make([]clipboardItemInternal, len(f.getClipboardResult))
	copy(out, f.getClipboardResult)
	return out, nil
}

func (f *fakeBackend) DocumentSetClipboard(_ documentHandle, items []clipboardItemInternal) error {
	f.lastSetClipboardItems = append([]clipboardItemInternal(nil), items...)
	return f.setClipboardErr
}

func (f *fakeBackend) RegisterOfficeCallback(_ officeHandle, h uintptr) error {
	f.lastOfficeCallbackHandle = h
	return f.registerOfficeCallbackErr
}

func (f *fakeBackend) RegisterDocumentCallback(_ documentHandle, h uintptr) error {
	f.lastDocumentCallbackHandle = h
	return f.registerDocCallbackErr
}

func (f *fakeBackend) GetCommandValues(_ documentHandle, cmd string) (string, error) {
	f.lastCommand = cmd
	if f.getCommandValuesErr != nil {
		return "", f.getCommandValuesErr
	}
	return f.lastCommandResult, nil
}

func (f *fakeBackend) CompleteFunction(_ documentHandle, name string) error {
	f.lastCommand = "CompleteFunction:" + name
	return f.completeFunctionErr
}

func (f *fakeBackend) SendDialogEvent(_ documentHandle, windowID uint64, argsJSON string) error {
	f.lastDialogWindowID = windowID
	f.lastDialogArgs = argsJSON
	return f.sendDialogEventErr
}

func (f *fakeBackend) SendContentControlEvent(_ documentHandle, argsJSON string) error {
	f.lastContentControlArgs = argsJSON
	return f.sendContentControlEventErr
}

func (f *fakeBackend) SendFormFieldEvent(_ documentHandle, argsJSON string) error {
	f.lastFormFieldArgs = argsJSON
	return f.sendFormFieldEventErr
}

func (f *fakeBackend) PostWindowKeyEvent(_ documentHandle, windowID uint32, typ, charCode, keyCode int) error {
	f.lastWindowID = windowID
	return nil
}

func (f *fakeBackend) PostWindowMouseEvent(_ documentHandle, windowID uint32, typ int, x, y int64, count int, buttons, mods int) error {
	f.lastWindowID = windowID
	return nil
}

func (f *fakeBackend) PostWindowGestureEvent(_ documentHandle, windowID uint32, typ string, _, _, _ int64) error {
	f.lastWindowID = windowID
	f.lastGestureType = typ
	return nil
}

func (f *fakeBackend) PostWindowExtTextInputEvent(_ documentHandle, windowID uint32, typ int, text string) error {
	f.lastWindowID = windowID
	f.lastExtTextInputType = typ
	f.lastExtTextInputText = text
	return nil
}

func (f *fakeBackend) ResizeWindow(_ documentHandle, windowID uint32, w, h int) error {
	f.lastWindowID = windowID
	return nil
}

func (f *fakeBackend) PaintWindow(_ documentHandle, windowID uint32, _ []byte, _, _, _, _ int) error {
	f.lastWindowID = windowID
	return nil
}

func (f *fakeBackend) PaintWindowDPI(_ documentHandle, windowID uint32, _ []byte, _, _, _, _ int, _ float64) error {
	f.lastWindowID = windowID
	return nil
}

func (f *fakeBackend) PaintWindowForView(_ documentHandle, windowID uint32, _ int, _ []byte, _, _, _, _ int, _ float64) error {
	f.lastWindowID = windowID
	return nil
}

// --- Phase 11: advanced + misc ---

func (f *fakeBackend) OfficeRunMacro(_ officeHandle, url string) error {
	f.lastMacroURL = url
	return f.macroErr
}

func (f *fakeBackend) OfficeSignDocument(_ officeHandle, url string, cert, key []byte) error {
	f.lastSignURL = url
	f.lastSignCert = cert
	f.lastSignKey = key
	return f.signErr
}

func (f *fakeBackend) OfficeGetFilterTypes(officeHandle) (string, error) {
	return f.filterTypesResult, f.filterTypesErr
}

func (f *fakeBackend) DocumentInsertCertificate(_ documentHandle, cert, key []byte) error {
	f.lastInsertCert = cert
	f.lastInsertKey = key
	return f.insertCertErr
}

func (f *fakeBackend) DocumentAddCertificate(_ documentHandle, cert []byte) error {
	f.lastAddCert = cert
	return f.addCertErr
}

func (f *fakeBackend) DocumentGetSignatureState(documentHandle) (int, error) {
	return f.signatureStateResult, f.signatureStateErr
}

func (f *fakeBackend) DocumentPaste(_ documentHandle, mime string, data []byte) error {
	f.lastPasteMime = mime
	f.lastPasteData = data
	return f.pasteErr
}

func (f *fakeBackend) DocumentSelectPart(_ documentHandle, part int, selected bool) error {
	f.lastSelectPart = part
	f.lastSelectSelected = selected
	return f.selectPartErr
}

func (f *fakeBackend) DocumentMoveSelectedParts(_ documentHandle, pos int, dup bool) error {
	f.lastMovePos = pos
	f.lastMoveDup = dup
	return f.moveSelectedPartsErr
}

func (f *fakeBackend) DocumentRenderFont(_ documentHandle, fontName, char string) ([]byte, int, int, error) {
	f.lastRenderFontName = fontName
	f.lastRenderFontChar = char
	return f.renderFontBuf, f.renderFontW, f.renderFontH, f.renderFontErr
}
