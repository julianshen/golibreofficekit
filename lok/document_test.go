//go:build linux || darwin

package lok

import "testing"

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

	d := &Document{office: o, h: &fakeDoc{be: fb}, closed: true}
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

	d := &Document{office: o, h: &fakeDoc{be: fb}, closed: false}
	if got := d.Type(); got != TypeSpreadsheet {
		t.Errorf("open doc Type=%v, want TypeSpreadsheet", got)
	}
}
