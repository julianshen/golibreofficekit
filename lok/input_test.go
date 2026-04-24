//go:build linux || darwin

package lok

import (
	"errors"
	"testing"
)

func TestMouseButton_Has(t *testing.T) {
	set := MouseLeft | MouseRight
	if !set.Has(MouseLeft) {
		t.Error("expected MouseLeft in set")
	}
	if !set.Has(MouseRight) {
		t.Error("expected MouseRight in set")
	}
	if set.Has(MouseMiddle) {
		t.Error("did not expect MouseMiddle in set")
	}
	// Has(multi-bit) returns true only if ALL bits are set.
	if !set.Has(MouseLeft | MouseRight) {
		t.Error("expected set to contain MouseLeft|MouseRight")
	}
	if set.Has(MouseLeft | MouseMiddle) {
		t.Error("did not expect set to contain MouseLeft|MouseMiddle")
	}
}

func TestMouseButton_String(t *testing.T) {
	cases := []struct {
		in   MouseButton
		want string
	}{
		{0, "(none)"},
		{MouseLeft, "MouseLeft"},
		{MouseLeft | MouseRight, "MouseLeft|MouseRight"},
		{MouseLeft | MouseMiddle | MouseRight, "MouseLeft|MouseRight|MouseMiddle"},
		// Unknown bits must render losslessly; "" is not an acceptable
		// String() output for a non-zero value.
		{MouseButton(8), "0x8"},
		{MouseLeft | MouseButton(8), "MouseLeft|0x8"},
		{MouseButton(0xFFF8), "0xfff8"},
	}
	for _, tc := range cases {
		if got := tc.in.String(); got != tc.want {
			t.Errorf("%d: got %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestModifier_Has(t *testing.T) {
	set := ModShift | ModMod1
	if !set.Has(ModShift) {
		t.Error("expected ModShift in set")
	}
	if !set.Has(ModMod1) {
		t.Error("expected ModMod1 in set")
	}
	if set.Has(ModMod2) {
		t.Error("did not expect ModMod2 in set")
	}
	if !set.Has(ModShift | ModMod1) {
		t.Error("expected set to contain ModShift|ModMod1")
	}
}

func TestModifier_String(t *testing.T) {
	cases := []struct {
		in   Modifier
		want string
	}{
		{0, "(none)"},
		{ModShift, "ModShift"},
		{ModShift | ModMod1, "ModShift|ModMod1"},
		{ModShift | ModMod1 | ModMod2 | ModMod3, "ModShift|ModMod1|ModMod2|ModMod3"},
		// Unknown bits must render losslessly; "" is not an acceptable
		// String() output for a non-zero value.
		{Modifier(0x10), "0x10"},
		{ModShift | Modifier(0x10), "ModShift|0x10"},
		{Modifier(0xFFF0), "0xfff0"},
	}
	for _, tc := range cases {
		if got := tc.in.String(); got != tc.want {
			t.Errorf("%d: got %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestPostKeyEvent_Forwards(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	if err := doc.PostKeyEvent(KeyEventInput, 'G', 0); err != nil {
		t.Fatal(err)
	}
	if fb.lastKeyType != int(KeyEventInput) || fb.lastCharCode != 'G' || fb.lastKeyCode != 0 {
		t.Errorf("got (type=%d, char=%d, key=%d); want (0, 71, 0)",
			fb.lastKeyType, fb.lastCharCode, fb.lastKeyCode)
	}
}

func TestPostKeyEvent_UsesKeyCodeConstant(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	if err := doc.PostKeyEvent(KeyEventInput, 0, KeyCodeEnter); err != nil {
		t.Fatal(err)
	}
	if fb.lastCharCode != 0 || fb.lastKeyCode != KeyCodeEnter {
		t.Errorf("got (char=%d, key=%d); want (0, %d)",
			fb.lastCharCode, fb.lastKeyCode, KeyCodeEnter)
	}
}

func TestPostMouseEvent_Forwards(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	if err := doc.PostMouseEvent(MouseButtonDown, 720, 960, 1, MouseLeft, ModShift); err != nil {
		t.Fatal(err)
	}
	if fb.lastMouseType != int(MouseButtonDown) ||
		fb.lastMouseX != 720 || fb.lastMouseY != 960 ||
		fb.lastMouseCount != 1 ||
		fb.lastMouseButton != int(MouseLeft) ||
		fb.lastMouseMods != int(ModShift) {
		t.Errorf("fakeBackend state=%+v", fb)
	}
}

func TestPostMouseEvent_RejectsOverflowX(t *testing.T) {
	_, doc := loadFakeDoc(t, &fakeBackend{})
	err := doc.PostMouseEvent(MouseMove, 1<<32+1, 0, 0, 0, 0)
	var lokErr *LOKError
	if !errors.As(err, &lokErr) || lokErr.Op != "PostMouseEvent" {
		t.Errorf("want *LOKError{Op: PostMouseEvent}, got %T %v", err, err)
	}
}

func TestPostMouseEvent_RejectsOverflowY(t *testing.T) {
	_, doc := loadFakeDoc(t, &fakeBackend{})
	err := doc.PostMouseEvent(MouseMove, 0, 1<<32+1, 0, 0, 0)
	var lokErr *LOKError
	if !errors.As(err, &lokErr) || lokErr.Op != "PostMouseEvent" {
		t.Errorf("want *LOKError{Op: PostMouseEvent}, got %T %v", err, err)
	}
}

func TestPostUnoCommand_Forwards(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	if err := doc.PostUnoCommand(".uno:Bold", "", false); err != nil {
		t.Fatal(err)
	}
	if fb.lastUnoCmd != ".uno:Bold" || fb.lastUnoArgs != "" || fb.lastUnoNotify {
		t.Errorf("fakeBackend state=%+v", fb)
	}
}

func TestPostUnoCommand_ForwardsNotifyTrue(t *testing.T) {
	fb := &fakeBackend{}
	_, doc := loadFakeDoc(t, fb)
	if err := doc.PostUnoCommand(".uno:Save", `{"x":1}`, true); err != nil {
		t.Fatal(err)
	}
	if fb.lastUnoCmd != ".uno:Save" || fb.lastUnoArgs != `{"x":1}` || !fb.lastUnoNotify {
		t.Errorf("fakeBackend state=%+v", fb)
	}
}

func TestInputMethods_AfterCloseErrors(t *testing.T) {
	cases := []struct {
		name string
		call func(*Document) error
	}{
		{"PostKeyEvent", func(d *Document) error { return d.PostKeyEvent(KeyEventInput, 'a', 0) }},
		{"PostMouseEvent", func(d *Document) error {
			return d.PostMouseEvent(MouseButtonDown, 0, 0, 1, MouseLeft, 0)
		}},
		{"PostUnoCommand", func(d *Document) error { return d.PostUnoCommand(".uno:Bold", "", false) }},
		{"Bold", func(d *Document) error { return d.Bold() }},
		{"Italic", func(d *Document) error { return d.Italic() }},
		{"Underline", func(d *Document) error { return d.Underline() }},
		{"Undo", func(d *Document) error { return d.Undo() }},
		{"Redo", func(d *Document) error { return d.Redo() }},
		{"Copy", func(d *Document) error { return d.Copy() }},
		{"Cut", func(d *Document) error { return d.Cut() }},
		{"Paste", func(d *Document) error { return d.Paste() }},
		{"SelectAll", func(d *Document) error { return d.SelectAll() }},
		{"InsertPageBreak", func(d *Document) error { return d.InsertPageBreak() }},
		{"InsertTable", func(d *Document) error { return d.InsertTable(1, 1) }},
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
