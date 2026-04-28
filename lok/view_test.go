//go:build linux || darwin

package lok

import (
	"errors"
	"testing"
)

func loadFakeDoc(t *testing.T, fb *fakeBackend) (*Office, *Document) {
	t.Helper()
	withFakeBackend(t, fb)
	o, err := New("/install")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := o.Load("/tmp/x.odt")
	if err != nil {
		o.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { doc.Close(); o.Close() })
	return o, doc
}

func TestCreateView_AllocatesID(t *testing.T) {
	_, doc := loadFakeDoc(t, &fakeBackend{})
	id, err := doc.CreateView()
	if err != nil {
		t.Fatal(err)
	}
	if id < 0 {
		t.Errorf("CreateView: got %d, want non-negative", id)
	}
}

func TestCreateView_BackendFailureErrors(t *testing.T) {
	_, doc := loadFakeDoc(t, &fakeBackend{viewCreateErr: true})
	_, err := doc.CreateView()
	if !errors.Is(err, ErrViewCreateFailed) {
		t.Errorf("want ErrViewCreateFailed, got %v", err)
	}
	var lokErr *LOKError
	if !errors.As(err, &lokErr) {
		t.Errorf("want *LOKError wrapper, got %T %v", err, err)
	}
}

func TestCreateViewWithOptions_PassesThrough(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	id, err := doc.CreateViewWithOptions("Language=de-DE")
	if err != nil {
		t.Fatal(err)
	}
	if id < 0 {
		t.Errorf("CreateViewWithOptions: got %d", id)
	}
	if fb.lastViewOptions != "Language=de-DE" {
		t.Errorf("CreateViewWithOptions options=%q, want %q", fb.lastViewOptions, "Language=de-DE")
	}
}

func TestCreateViewWithOptions_BackendFailureErrors(t *testing.T) {
	_, doc := loadFakeDoc(t, &fakeBackend{viewCreateErr: true})
	_, err := doc.CreateViewWithOptions("x=1")
	if !errors.Is(err, ErrViewCreateFailed) {
		t.Errorf("want ErrViewCreateFailed, got %v", err)
	}
}

func TestSetView_UpdatesActiveView(t *testing.T) {
	_, doc := loadFakeDoc(t, &fakeBackend{})
	id1, _ := doc.CreateView()
	_, _ = doc.CreateView()
	if err := doc.SetView(id1); err != nil {
		t.Fatal(err)
	}
	got, err := doc.View()
	if err != nil {
		t.Fatal(err)
	}
	if got != id1 {
		t.Errorf("View()=%d after SetView(%d)", got, id1)
	}
}

func TestViews_ListsLiveIDs(t *testing.T) {
	_, doc := loadFakeDoc(t, &fakeBackend{})
	a, _ := doc.CreateView()
	b, _ := doc.CreateView()
	ids, err := doc.Views()
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != a || ids[1] != b {
		t.Errorf("Views=%v, want [%d %d]", ids, a, b)
	}
}

func TestViews_EmptyReturnsNilNil(t *testing.T) {
	_, doc := loadFakeDoc(t, &fakeBackend{})
	ids, err := doc.Views()
	if err != nil {
		t.Fatal(err)
	}
	if ids != nil {
		t.Errorf("Views on empty: got %v, want nil", ids)
	}
}

func TestDestroyView_RemovesFromList(t *testing.T) {
	_, doc := loadFakeDoc(t, &fakeBackend{})
	a, _ := doc.CreateView()
	b, _ := doc.CreateView()
	if err := doc.DestroyView(a); err != nil {
		t.Fatal(err)
	}
	ids, err := doc.Views()
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != b {
		t.Errorf("after Destroy(%d), Views=%v, want [%d]", a, ids, b)
	}
}

func TestView_AfterCloseErrors(t *testing.T) {
	_, doc := loadFakeDoc(t, &fakeBackend{})
	_, _ = doc.CreateView()
	doc.Close()
	if _, err := doc.View(); !errors.Is(err, ErrClosed) {
		t.Errorf("View after Close: want ErrClosed, got %v", err)
	}
	if _, err := doc.Views(); !errors.Is(err, ErrClosed) {
		t.Errorf("Views after Close: want ErrClosed, got %v", err)
	}
	if err := doc.SetView(0); !errors.Is(err, ErrClosed) {
		t.Errorf("SetView after Close: want ErrClosed, got %v", err)
	}
	if err := doc.DestroyView(0); !errors.Is(err, ErrClosed) {
		t.Errorf("DestroyView after Close: want ErrClosed, got %v", err)
	}
	if _, err := doc.CreateView(); !errors.Is(err, ErrClosed) {
		t.Errorf("CreateView after Close: want ErrClosed, got %v", err)
	}
	if _, err := doc.CreateViewWithOptions(""); !errors.Is(err, ErrClosed) {
		t.Errorf("CreateViewWithOptions after Close: want ErrClosed, got %v", err)
	}
}

func TestSetViewLanguage_Records(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	id, _ := doc.CreateView()
	if err := doc.SetViewLanguage(id, "de-DE"); err != nil {
		t.Fatal(err)
	}
	if fb.lastViewLang != "de-DE" || fb.lastViewLangID != int(id) {
		t.Errorf("SetViewLanguage recorded (id=%d lang=%q)", fb.lastViewLangID, fb.lastViewLang)
	}
}

func TestSetViewReadOnly_Records(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	id, _ := doc.CreateView()
	if err := doc.SetViewReadOnly(id, true); err != nil {
		t.Fatal(err)
	}
	if !fb.lastViewReadOnly {
		t.Error("SetViewReadOnly(true) not recorded")
	}
}

func TestSetAccessibilityState_Records(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	id, _ := doc.CreateView()
	if err := doc.SetAccessibilityState(id, true); err != nil {
		t.Fatal(err)
	}
	if !fb.lastViewA11y {
		t.Error("SetAccessibilityState(true) not recorded")
	}
}

func TestSetViewTimezone_Records(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	id, _ := doc.CreateView()
	if err := doc.SetViewTimezone(id, "Europe/Berlin"); err != nil {
		t.Fatal(err)
	}
	if fb.lastViewTimezone != "Europe/Berlin" {
		t.Errorf("SetViewTimezone: got %q", fb.lastViewTimezone)
	}
}

// TestViewSetters_PropagateUnsupported asserts every widened view
// method forwards the backend ErrUnsupported untouched, so callers
// on stripped LO builds (vtable slot NULL) see the unsupported
// signal instead of silent success.
func TestViewSetters_PropagateUnsupported(t *testing.T) {
	cases := []struct {
		name   string
		inject func(*fakeBackend)
		call   func(*Document) error
	}{
		{"DestroyView", func(f *fakeBackend) { f.destroyViewErr = ErrUnsupported },
			func(d *Document) error { return d.DestroyView(0) }},
		{"SetView", func(f *fakeBackend) { f.setViewErr = ErrUnsupported },
			func(d *Document) error { return d.SetView(0) }},
		{"SetViewLanguage", func(f *fakeBackend) { f.setViewLanguageErr = ErrUnsupported },
			func(d *Document) error { return d.SetViewLanguage(0, "x") }},
		{"SetViewReadOnly", func(f *fakeBackend) { f.setViewReadOnlyErr = ErrUnsupported },
			func(d *Document) error { return d.SetViewReadOnly(0, true) }},
		{"SetAccessibilityState", func(f *fakeBackend) { f.setAccessibilityStateErr = ErrUnsupported },
			func(d *Document) error { return d.SetAccessibilityState(0, true) }},
		{"SetViewTimezone", func(f *fakeBackend) { f.setViewTimezoneErr = ErrUnsupported },
			func(d *Document) error { return d.SetViewTimezone(0, "UTC") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fb := &fakeBackend{}
			tc.inject(fb)
			_, doc := loadFakeDoc(t, fb)
			if err := tc.call(doc); !errors.Is(err, ErrUnsupported) {
				t.Errorf("err=%v, want ErrUnsupported", err)
			}
		})
	}
}

func TestViewConfigurators_AfterCloseErrors(t *testing.T) {
	cases := []struct {
		name string
		call func(*Document) error
	}{
		{"SetViewLanguage", func(d *Document) error { return d.SetViewLanguage(0, "x") }},
		{"SetViewReadOnly", func(d *Document) error { return d.SetViewReadOnly(0, true) }},
		{"SetAccessibilityState", func(d *Document) error { return d.SetAccessibilityState(0, true) }},
		{"SetViewTimezone", func(d *Document) error { return d.SetViewTimezone(0, "UTC") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, doc := loadFakeDoc(t, &fakeBackend{})
			doc.Close()
			if err := tc.call(doc); !errors.Is(err, ErrClosed) {
				t.Errorf("want ErrClosed, got %v", err)
			}
		})
	}
}
