//go:build linux || darwin

package lok

import (
	"bytes"
	"errors"
	"os"
	"testing"
	"testing/iotest"
)

func TestDocumentType_String(t *testing.T) {
	cases := []struct {
		t    DocumentType
		want string
	}{
		{TypeText, "Text"},
		{TypeSpreadsheet, "Spreadsheet"},
		{TypePresentation, "Presentation"},
		{TypeDrawing, "Drawing"},
		{TypeOther, "Other"},
		{DocumentType(99), "DocumentType(99)"},
	}
	for _, tc := range cases {
		if got := tc.t.String(); got != tc.want {
			t.Errorf("%d: got %q, want %q", tc.t, got, tc.want)
		}
	}
}

func TestBuildLoadOptions_Defaults(t *testing.T) {
	lo := buildLoadOptions(nil)
	if lo.password != "" || lo.readOnly || lo.lang != "" ||
		lo.macroSecuritySet || lo.batch || lo.repair || lo.filterOpts != "" {
		t.Errorf("non-zero defaults: %+v", lo)
	}
}

func TestLoadOptions_WithPassword(t *testing.T) {
	lo := buildLoadOptions([]LoadOption{WithPassword("secret")})
	if lo.password != "secret" {
		t.Errorf("password=%q, want secret", lo.password)
	}
}

func TestLoadOptions_WithReadOnly(t *testing.T) {
	lo := buildLoadOptions([]LoadOption{WithReadOnly()})
	if !lo.readOnly {
		t.Error("readOnly=false, want true")
	}
}

func TestLoadOptions_WithLanguage(t *testing.T) {
	lo := buildLoadOptions([]LoadOption{WithLanguage("en-US")})
	if lo.lang != "en-US" {
		t.Errorf("lang=%q, want en-US", lo.lang)
	}
}

func TestLoadOptions_WithMacroSecurity(t *testing.T) {
	lo := buildLoadOptions([]LoadOption{WithMacroSecurity(MacroSecurityHigh)})
	if !lo.macroSecuritySet || lo.macroSecurity != MacroSecurityHigh {
		t.Errorf("macroSecurity=%v set=%v, want High+true", lo.macroSecurity, lo.macroSecuritySet)
	}
}

func TestLoadOptions_WithBatchMode(t *testing.T) {
	lo := buildLoadOptions([]LoadOption{WithBatchMode()})
	if !lo.batch {
		t.Error("batch=false, want true")
	}
}

func TestLoadOptions_WithRepair(t *testing.T) {
	lo := buildLoadOptions([]LoadOption{WithRepair()})
	if !lo.repair {
		t.Error("repair=false, want true")
	}
}

func TestLoadOptions_WithFilterOptions(t *testing.T) {
	lo := buildLoadOptions([]LoadOption{WithFilterOptions("foo=bar")})
	if lo.filterOpts != "foo=bar" {
		t.Errorf("filterOpts=%q, want foo=bar", lo.filterOpts)
	}
}

func TestDocument_Type_Closed(t *testing.T) {
	fb := &fakeBackend{docType: int(TypeSpreadsheet)}
	withFakeBackend(t, fb)
	o, err := New("/install")
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	d := &Document{office: o, h: &fakeDoc{}, closed: true}
	if got := d.Type(); got != TypeOther {
		t.Errorf("closed doc Type=%v, want TypeOther", got)
	}
}

func TestDocument_Type_Open(t *testing.T) {
	fb := &fakeBackend{docType: int(TypeSpreadsheet)}
	withFakeBackend(t, fb)
	o, err := New("/install")
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	// Document.Type returns the value cached at Load time; go through
	// the Load path rather than constructing a bare Document so the
	// cache is populated.
	doc, err := o.Load("/tmp/x.ods")
	if err != nil {
		t.Fatal(err)
	}
	defer doc.Close()
	if got := doc.Type(); got != TypeSpreadsheet {
		t.Errorf("open doc Type=%v, want TypeSpreadsheet", got)
	}
}

func TestLoad_EmptyPathErrors(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, err := New("/install")
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	if _, err := o.Load(""); !errors.Is(err, ErrPathRequired) {
		t.Errorf("want ErrPathRequired, got %v", err)
	}
}

func TestLoad_PassesFileURL(t *testing.T) {
	fb := &fakeBackend{docType: int(TypeSpreadsheet)}
	withFakeBackend(t, fb)
	o, err := New("/install")
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	doc, err := o.Load("/tmp/hello.ods")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	t.Cleanup(func() { doc.Close() })

	if fb.lastLoadURL != "file:///tmp/hello.ods" {
		t.Errorf("Load URL: got %q, want file:///tmp/hello.ods", fb.lastLoadURL)
	}
	if doc.Type() != TypeSpreadsheet {
		t.Errorf("Type()=%v, want Spreadsheet", doc.Type())
	}
}

func TestLoad_PercentEncodesSpaces(t *testing.T) {
	fb := &fakeBackend{}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	doc, err := o.Load("/tmp/has space.odt")
	if err != nil {
		t.Fatal(err)
	}
	defer doc.Close()
	if fb.lastLoadURL != "file:///tmp/has%20space.odt" {
		t.Errorf("URL: got %q, want file:///tmp/has%%20space.odt", fb.lastLoadURL)
	}
}

func TestLoad_WithOptions_UsesLoadWithOptions(t *testing.T) {
	fb := &fakeBackend{}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()

	doc, err := o.Load("/tmp/x.odt", WithPassword("hunter2"), WithReadOnly())
	if err != nil {
		t.Fatal(err)
	}
	defer doc.Close()
	if fb.lastPwdPassword != "hunter2" {
		t.Error("password not forwarded to SetDocumentPassword")
	}
	if fb.lastLoadOpts == "" {
		t.Error("LoadWithOptions not invoked for options case")
	}
}

func TestLoad_BackendError(t *testing.T) {
	errSynth := errors.New("synthetic")
	fb := &fakeBackend{loadErr: errSynth}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	_, err := o.Load("/tmp/x.odt")
	if !errors.Is(err, errSynth) {
		t.Errorf("want synthetic, got %v", err)
	}
}

// TestLoad_SurfacesLOErrorDetail asserts that when DocumentLoad fails
// AND LibreOffice has a pending error string, Office.Load returns a
// *LOKError whose Detail is the LO-supplied diagnostic — not the
// internal-sentinel message. This is the user-visible bit that tells
// callers WHY a load failed (password required, filter rejected, etc.).
func TestLoad_SurfacesLOErrorDetail(t *testing.T) {
	fb := &fakeBackend{
		loadErr:     errors.New("synthetic load failure"),
		officeError: "password required",
	}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()

	_, err := o.Load("/tmp/x.odt")
	var lokErr *LOKError
	if !errors.As(err, &lokErr) {
		t.Fatalf("Load: want *LOKError, got %T %v", err, err)
	}
	if lokErr.Op != "Load" {
		t.Errorf("Op=%q, want Load", lokErr.Op)
	}
	if lokErr.Detail != "password required" {
		t.Errorf("Detail=%q, want %q", lokErr.Detail, "password required")
	}
}

// TestLoadWithOptions_SurfacesLOErrorDetail mirrors
// TestLoad_SurfacesLOErrorDetail for the LoadWithOptions code path —
// triggered by passing any LoadOption that touches the options string.
// LoadWithOptions and Load take separate backend entry points, so
// both routes must surface the LO diagnostic.
func TestLoadWithOptions_SurfacesLOErrorDetail(t *testing.T) {
	fb := &fakeBackend{
		loadErr:     errors.New("synthetic load failure"),
		officeError: "filter rejected",
	}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()

	_, err := o.Load("/tmp/x.docx", WithReadOnly())
	var lokErr *LOKError
	if !errors.As(err, &lokErr) {
		t.Fatalf("Load: want *LOKError, got %T %v", err, err)
	}
	if lokErr.Op != "Load" {
		t.Errorf("Op=%q, want Load", lokErr.Op)
	}
	if lokErr.Detail != "filter rejected" {
		t.Errorf("Detail=%q, want %q", lokErr.Detail, "filter rejected")
	}
	// Also assert the LoadWithOptions path was exercised, not the
	// no-options path. fb.lastLoadOpts is only populated by
	// DocumentLoadWithOptions in the fake.
	if fb.lastLoadOpts == "" {
		t.Error("LoadWithOptions path was not exercised; fb.lastLoadOpts empty")
	}
}

// TestLoad_FallsBackToBackendErrorWhenNoLODetail confirms the
// fallback: when LO has nothing pending in getError, the *LOKError
// still surfaces a useful message rather than dropping the cause.
func TestLoad_FallsBackToBackendErrorWhenNoLODetail(t *testing.T) {
	synth := errors.New("synthetic load failure")
	fb := &fakeBackend{loadErr: synth, officeError: ""}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()

	_, err := o.Load("/tmp/x.odt")
	var lokErr *LOKError
	if !errors.As(err, &lokErr) {
		t.Fatalf("Load: want *LOKError, got %T %v", err, err)
	}
	if lokErr.Op != "Load" {
		t.Errorf("Op=%q, want Load", lokErr.Op)
	}
	if lokErr.Detail == "" {
		t.Error("Detail empty; want fallback message including the backend error")
	}
	// Original error must still be reachable for errors.Is.
	if !errors.Is(err, synth) {
		t.Errorf("errors.Is(synth) failed; chain lost the cause: %v", err)
	}
}

func TestLoad_InvalidLanguageOption(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	_, err := o.Load("/tmp/x.odt", WithLanguage("en,US"))
	if !errors.Is(err, ErrInvalidOption) {
		t.Errorf("want ErrInvalidOption, got %v", err)
	}
}

func TestLoad_RegisterDocumentCallbackError(t *testing.T) {
	synth := errors.New("synthetic register-doc-callback failure")
	fb := &fakeBackend{registerDocCallbackErr: synth}
	withFakeBackend(t, fb)

	o, err := New("/install")
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	doc, err := o.Load("/tmp/x.odt")
	if err == nil {
		t.Fatalf("Load: want error, got nil; doc=%v", doc)
	}
	if doc != nil {
		t.Errorf("Load: want nil Document on failure, got %v", doc)
	}
	if !errors.Is(err, synth) {
		t.Errorf("Load: want wraps synthetic, got %v", err)
	}
}

func TestDocument_Close_Idempotent(t *testing.T) {
	fb := &fakeBackend{}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	if err := doc.Close(); err != nil {
		t.Fatal(err)
	}
	if err := doc.Close(); err != nil {
		t.Error(err)
	}
	if fb.docDestroys != 1 {
		t.Errorf("docDestroys: want 1, got %d", fb.docDestroys)
	}
}

func TestSaveAs_PassesFileURL(t *testing.T) {
	fb := &fakeBackend{}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()

	if err := doc.SaveAs("/tmp/x.pdf", "pdf", "SkipImages=1"); err != nil {
		t.Fatal(err)
	}
	if fb.lastSaveURL != "file:///tmp/x.pdf" {
		t.Errorf("SaveAs URL: %q", fb.lastSaveURL)
	}
	if fb.lastSaveFmt != "pdf" {
		t.Errorf("SaveAs format: %q", fb.lastSaveFmt)
	}
	if fb.lastSaveOpts != "SkipImages=1" {
		t.Errorf("SaveAs opts: %q", fb.lastSaveOpts)
	}
}

func TestSaveAs_EmptyPathErrors(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()
	if err := doc.SaveAs("", "", ""); !errors.Is(err, ErrPathRequired) {
		t.Errorf("want ErrPathRequired, got %v", err)
	}
}

func TestSaveAs_BackendError(t *testing.T) {
	synth := errors.New("synthetic save")
	fb := &fakeBackend{saveErr: synth}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()
	err := doc.SaveAs("/tmp/x.pdf", "pdf", "")
	if !errors.Is(err, synth) {
		t.Errorf("want synthetic via Unwrap, got %v", err)
	}
}

func TestSaveAs_AfterCloseErrors(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	doc.Close()
	if err := doc.SaveAs("/tmp/x.pdf", "pdf", ""); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestSave_ReusesOrigURL(t *testing.T) {
	fb := &fakeBackend{}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()
	if err := doc.Save(); err != nil {
		t.Fatal(err)
	}
	if fb.lastSaveURL != doc.origURL {
		t.Errorf("Save: URL=%q, want %q", fb.lastSaveURL, doc.origURL)
	}
	if fb.lastSaveFmt != "" {
		t.Errorf("Save: format should be empty, got %q", fb.lastSaveFmt)
	}
}

func TestSave_AfterCloseErrors(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	doc.Close()
	if err := doc.Save(); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestSave_BackendError(t *testing.T) {
	synth := errors.New("synthetic save via Save")
	fb := &fakeBackend{saveErr: synth}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	doc, _ := o.Load("/tmp/x.odt")
	defer doc.Close()
	if err := doc.Save(); !errors.Is(err, synth) {
		t.Errorf("want synthetic via Unwrap, got %v", err)
	}
}

func TestLoadFromReader_WritesTempFileAndCleansUp(t *testing.T) {
	fb := &fakeBackend{}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()

	content := []byte("PK fake ODT bytes")
	doc, err := o.LoadFromReader(bytes.NewReader(content), "odt")
	if err != nil {
		t.Fatal(err)
	}
	tempPath := doc.tempPath
	if tempPath == "" {
		t.Fatal("tempPath not set")
	}
	if _, err := os.Stat(tempPath); err != nil {
		t.Errorf("temp file not present: %v", err)
	}
	if err := doc.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Errorf("temp file not removed: %v", err)
	}
}

func TestLoadFromReader_EmptyFilterOK(t *testing.T) {
	fb := &fakeBackend{}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	doc, err := o.LoadFromReader(bytes.NewReader([]byte("x")), "")
	if err != nil {
		t.Fatal(err)
	}
	defer doc.Close()
	if doc.tempPath == "" {
		t.Error("tempPath not set")
	}
}

func TestLoadFromReader_ReaderError(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, _ := New("/install")
	defer o.Close()
	_, err := o.LoadFromReader(iotest.ErrReader(errors.New("boom")), "odt")
	if err == nil {
		t.Fatal("expected reader error to surface")
	}
	// Ensure no orphaned temp files remain. We can't get the path
	// back (the Document isn't returned), so this just verifies that
	// LoadFromReader doesn't leak on the error path at least via
	// os.TempDir. A directory listing is out of scope; rely on the
	// implementation to Remove on failure.
}

func TestLoadFromReader_LoadError(t *testing.T) {
	synth := errors.New("synthetic load")
	fb := &fakeBackend{loadErr: synth}
	withFakeBackend(t, fb)
	o, _ := New("/install")
	defer o.Close()
	_, err := o.LoadFromReader(bytes.NewReader([]byte("x")), "odt")
	if !errors.Is(err, synth) {
		t.Errorf("want synthetic, got %v", err)
	}
}

func TestComposeLoadOptions(t *testing.T) {
	cases := []struct {
		name string
		in   loadOptions
		want string
		err  error
	}{
		{"empty", loadOptions{}, "", nil},
		{"readonly", loadOptions{readOnly: true}, "ReadOnly=1", nil},
		{"lang", loadOptions{lang: "en-US"}, "Language=en-US", nil},
		{"macro", loadOptions{macroSecurity: MacroSecurityMedium, macroSecuritySet: true}, "MacroSecurityLevel=1", nil},
		{"batch+repair", loadOptions{batch: true, repair: true}, "Batch=1,Repair=1", nil},
		{"filter pass-through", loadOptions{filterOpts: "SkipImages=1"}, "SkipImages=1", nil},
		{"combined", loadOptions{readOnly: true, lang: "de-DE", filterOpts: "X=1"}, "ReadOnly=1,Language=de-DE,X=1", nil},
		{"lang with comma errors", loadOptions{lang: "en,US"}, "", ErrInvalidOption},
		{"lang with equals errors", loadOptions{lang: "en=US"}, "", ErrInvalidOption},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := composeLoadOptions(tc.in)
			if !errors.Is(err, tc.err) {
				t.Errorf("err: got %v, want %v", err, tc.err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
