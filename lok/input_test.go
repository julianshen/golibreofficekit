//go:build linux || darwin

package lok

import (
	"errors"
	"math"
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

func TestKeyCodeConstants_MatchAwtKey(t *testing.T) {
	// Golden check against com::sun::star::awt::Key (IDL:
	// offapi/com/sun/star/awt/Key.idl). Catches a typo in the
	// constants that would otherwise pass the forwarding tests
	// (which compare symbolically against themselves).
	cases := []struct {
		name string
		got  int
		want int
	}{
		{"Enter", KeyCodeEnter, 1280},
		{"Esc", KeyCodeEsc, 1281},
		{"Tab", KeyCodeTab, 1282},
		{"Backspace", KeyCodeBackspace, 1283},
		{"Delete", KeyCodeDelete, 1286},
		{"Up", KeyCodeUp, 1024},
		{"Down", KeyCodeDown, 1025},
		{"Left", KeyCodeLeft, 1026},
		{"Right", KeyCodeRight, 1027},
		{"Home", KeyCodeHome, 1028},
		{"End", KeyCodeEnd, 1029},
		{"PageUp", KeyCodePageUp, 1030},
		{"PageDown", KeyCodePageDown, 1031},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("KeyCode%s = %d, want %d", tc.name, tc.got, tc.want)
		}
	}
}

func TestPostKeyEvent_Rejects(t *testing.T) {
	cases := []struct {
		name              string
		charCode, keyCode int
	}{
		{"charCode over int32", math.MaxInt32 + 1, 0},
		{"charCode negative overflow", math.MinInt32 - 1, 0},
		{"keyCode over int32", 0, math.MaxInt32 + 1},
		{"keyCode negative overflow", 0, math.MinInt32 - 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fb := &fakeBackend{}
			_, doc := loadFakeDoc(t, fb)
			err := doc.PostKeyEvent(KeyEventInput, tc.charCode, tc.keyCode)
			var lokErr *LOKError
			if !errors.As(err, &lokErr) || lokErr.Op != "PostKeyEvent" {
				t.Errorf("want *LOKError{Op: PostKeyEvent}, got %T %v", err, err)
			}
			if fb.lastKeyType != 0 || fb.lastCharCode != 0 || fb.lastKeyCode != 0 {
				t.Errorf("backend invoked despite out-of-range input: %+v", fb)
			}
		})
	}
}

func TestPostMouseEvent_Forwards(t *testing.T) {
	// Cover every named MouseButton / Modifier bit plus one multi-bit
	// combo each so a `int(buttons & 0x01)` partial-mask regression
	// would fail at least one row. Also exercises all three
	// MouseEventType variants.
	cases := []struct {
		name    string
		typ     MouseEventType
		x, y    int64
		count   int
		buttons MouseButton
		mods    Modifier
	}{
		{"left+shift down", MouseButtonDown, 720, 960, 1, MouseLeft, ModShift},
		{"right alone up", MouseButtonUp, 100, 200, 1, MouseRight, 0},
		{"middle alone move", MouseMove, 0, 0, 0, MouseMiddle, 0},
		{"left+middle", MouseButtonDown, 5, 6, 2, MouseLeft | MouseMiddle, 0},
		{"left+right+middle", MouseButtonDown, 7, 8, 1,
			MouseLeft | MouseRight | MouseMiddle, 0},
		{"mod1 only", MouseButtonDown, 1, 2, 1, MouseLeft, ModMod1},
		{"mod2 only", MouseButtonDown, 1, 2, 1, MouseLeft, ModMod2},
		{"mod3 only", MouseButtonDown, 1, 2, 1, MouseLeft, ModMod3},
		{"all mods", MouseButtonDown, 1, 2, 1, MouseLeft,
			ModShift | ModMod1 | ModMod2 | ModMod3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fb := &fakeBackend{}
			_, doc := loadFakeDoc(t, fb)
			if err := doc.PostMouseEvent(tc.typ, tc.x, tc.y, tc.count,
				tc.buttons, tc.mods); err != nil {
				t.Fatal(err)
			}
			if fb.lastMouseType != int(tc.typ) ||
				fb.lastMouseX != int(tc.x) || fb.lastMouseY != int(tc.y) ||
				fb.lastMouseCount != tc.count ||
				fb.lastMouseButton != int(tc.buttons) ||
				fb.lastMouseMods != int(tc.mods) {
				t.Errorf("fakeBackend state=%+v; tc=%+v", fb, tc)
			}
		})
	}
}

func TestPostMouseEvent_RejectsOverflow(t *testing.T) {
	// Cover both positive and negative int32 overflow on both axes so
	// requireInt32XY's `< math.MinInt32` branches aren't dead.
	cases := []struct {
		name string
		x, y int64
	}{
		{"x above MaxInt32", math.MaxInt32 + 1, 0},
		{"x below MinInt32", math.MinInt32 - 1, 0},
		{"y above MaxInt32", 0, math.MaxInt32 + 1},
		{"y below MinInt32", 0, math.MinInt32 - 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, doc := loadFakeDoc(t, &fakeBackend{})
			err := doc.PostMouseEvent(MouseMove, tc.x, tc.y, 0, 0, 0)
			var lokErr *LOKError
			if !errors.As(err, &lokErr) || lokErr.Op != "PostMouseEvent" {
				t.Errorf("want *LOKError{Op: PostMouseEvent}, got %T %v", err, err)
			}
		})
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

// TestInputSetters_PropagateUnsupported asserts every widened input
// method forwards the backend ErrUnsupported untouched, so callers
// on stripped LO builds (vtable slot NULL) see the unsupported
// signal instead of silent success.
func TestInputSetters_PropagateUnsupported(t *testing.T) {
	cases := []struct {
		name   string
		inject func(*fakeBackend)
		call   func(*Document) error
	}{
		{"PostKeyEvent", func(f *fakeBackend) { f.postKeyEventErr = ErrUnsupported },
			func(d *Document) error { return d.PostKeyEvent(KeyEventInput, 'a', 0) }},
		{"PostMouseEvent", func(f *fakeBackend) { f.postMouseEventErr = ErrUnsupported },
			func(d *Document) error { return d.PostMouseEvent(MouseButtonDown, 0, 0, 1, MouseLeft, 0) }},
		{"PostUnoCommand", func(f *fakeBackend) { f.postUnoCommandErr = ErrUnsupported },
			func(d *Document) error { return d.PostUnoCommand(".uno:Bold", "", false) }},
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
