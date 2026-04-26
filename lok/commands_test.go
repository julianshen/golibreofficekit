//go:build linux || darwin

package lok

import (
	"errors"
	"math"
	"testing"
)

func TestTypedHelpers_DispatchExpectedCommand(t *testing.T) {
	cases := []struct {
		name string
		call func(*Document) error
		want string
	}{
		{"Bold", func(d *Document) error { return d.Bold() }, ".uno:Bold"},
		{"Italic", func(d *Document) error { return d.Italic() }, ".uno:Italic"},
		{"Underline", func(d *Document) error { return d.Underline() }, ".uno:Underline"},
		{"Undo", func(d *Document) error { return d.Undo() }, ".uno:Undo"},
		{"Redo", func(d *Document) error { return d.Redo() }, ".uno:Redo"},
		{"Copy", func(d *Document) error { return d.Copy() }, ".uno:Copy"},
		{"Cut", func(d *Document) error { return d.Cut() }, ".uno:Cut"},
		{"Paste", func(d *Document) error { return d.Paste() }, ".uno:Paste"},
		{"SelectAll", func(d *Document) error { return d.SelectAll() }, ".uno:SelectAll"},
		{"InsertPageBreak", func(d *Document) error { return d.InsertPageBreak() }, ".uno:InsertPageBreak"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fb := &fakeBackend{}
			_, doc := loadFakeDoc(t, fb)
			if err := tc.call(doc); err != nil {
				t.Fatal(err)
			}
			if fb.lastUnoCmd != tc.want {
				t.Errorf("cmd=%q, want %q", fb.lastUnoCmd, tc.want)
			}
			if fb.lastUnoArgs != "" {
				t.Errorf("args=%q, want empty", fb.lastUnoArgs)
			}
			if fb.lastUnoNotify {
				t.Error("notify=true, want false")
			}
		})
	}
}

func TestInsertTable_Rejects(t *testing.T) {
	cases := []struct {
		name       string
		rows, cols int
	}{
		{"zero rows", 0, 1},
		{"zero cols", 1, 0},
		{"negative rows", -1, 1},
		{"negative cols", 1, -1},
		{"rows over int32", math.MaxInt32 + 1, 1},
		{"cols over int32", 1, math.MaxInt32 + 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fb := &fakeBackend{}
			_, doc := loadFakeDoc(t, fb)
			err := doc.InsertTable(tc.rows, tc.cols)
			var lokErr *LOKError
			if !errors.As(err, &lokErr) || lokErr.Op != "InsertTable" {
				t.Errorf("want *LOKError{Op: InsertTable}, got %T %v", err, err)
			}
			if fb.lastUnoCmd != "" {
				t.Errorf("backend was invoked (cmd=%q); expected pre-flight rejection",
					fb.lastUnoCmd)
			}
		})
	}
}

func TestInsertTable_BuildsExpectedJSON(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	if err := doc.InsertTable(3, 4); err != nil {
		t.Fatal(err)
	}
	if fb.lastUnoCmd != ".uno:InsertTable" {
		t.Errorf("cmd=%q", fb.lastUnoCmd)
	}
	want := `{"Columns":{"type":"long","value":4},"Rows":{"type":"long","value":3}}`
	if fb.lastUnoArgs != want {
		t.Errorf("args=%q, want %q", fb.lastUnoArgs, want)
	}
	if fb.lastUnoNotify {
		t.Error("notify=true, want false")
	}
}

func TestGetCommandValues(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)

	fb.lastCommandResult = `{"enabled":true}`
	raw, err := doc.GetCommandValues(".uno:Save")
	if err != nil {
		t.Fatalf("GetCommandValues: %v", err)
	}
	if string(raw) != `{"enabled":true}` {
		t.Errorf("got %s", raw)
	}
	if fb.lastCommand != ".uno:Save" {
		t.Errorf("lastCommand=%s", fb.lastCommand)
	}
}

func TestGetCommandValues_BackendError(t *testing.T) {
	fb := &fakeBackend{getCommandValuesErr: ErrUnsupported}
	_, doc := loadFakeDoc(t, fb)

	_, err := doc.GetCommandValues(".uno:Save")
	if !errors.Is(err, ErrUnsupported) {
		t.Errorf("want ErrUnsupported, got %v", err)
	}
}

func TestGetCommandValues_Closed(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	doc.Close()

	_, err := doc.GetCommandValues(".uno:Save")
	if !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestCompleteFunction(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)

	if err := doc.CompleteFunction("SUM"); err != nil {
		t.Fatalf("CompleteFunction: %v", err)
	}
	if fb.lastCommand != "CompleteFunction:SUM" {
		t.Errorf("lastCommand=%s", fb.lastCommand)
	}
}

func TestCompleteFunction_Closed(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	doc.Close()

	if err := doc.CompleteFunction("SUM"); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestIsCommandEnabled_True(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)

	fb.lastCommandResult = `{"enabled":true}`
	ok, err := doc.IsCommandEnabled(".uno:Bold")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected true")
	}
}

func TestIsCommandEnabled_False(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)

	fb.lastCommandResult = `{"enabled":false}`
	ok, err := doc.IsCommandEnabled(".uno:Bold")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected false")
	}
}

func TestIsCommandEnabled_StateField(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)

	fb.lastCommandResult = `{"state":true}`
	ok, err := doc.IsCommandEnabled(".uno:Bold")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected true from state field")
	}
}

func TestIsCommandEnabled_NoEnabledField(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)

	fb.lastCommandResult = `{"command":".uno:Bold"}`
	ok, err := doc.IsCommandEnabled(".uno:Bold")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected false when enabled/state absent")
	}
}

func TestIsCommandEnabled_InvalidJSON(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)

	fb.lastCommandResult = `{invalid`
	_, err := doc.IsCommandEnabled(".uno:Bold")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestGetFontNames(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)

	fb.lastCommandResult = `{"type":"list","command":".uno:FontName","value":["Arial","Times"]}`
	names, err := doc.GetFontNames()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 || names[0] != "Arial" || names[1] != "Times" {
		t.Errorf("got %v", names)
	}
}

func TestGetFontNames_EmptyList(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)

	fb.lastCommandResult = `{"type":"list","command":".uno:FontName","value":[]}`
	names, err := doc.GetFontNames()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 0 {
		t.Errorf("expected empty, got %v", names)
	}
}

func TestGetFontNames_NoValueField(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)

	fb.lastCommandResult = `{"type":"list"}`
	names, err := doc.GetFontNames()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 0 {
		t.Errorf("expected empty slice, got %v", names)
	}
}

func TestGetFontNames_InvalidJSON(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)

	fb.lastCommandResult = `not json`
	_, err := doc.GetFontNames()
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
