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
