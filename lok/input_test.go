//go:build linux || darwin

package lok

import "testing"

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
	}
	for _, tc := range cases {
		if got := tc.in.String(); got != tc.want {
			t.Errorf("%d: got %q, want %q", tc.in, got, tc.want)
		}
	}
}
