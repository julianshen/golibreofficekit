//go:build linux || darwin

package lok

import "testing"

func TestSetTextSelectionType_String(t *testing.T) {
	cases := []struct {
		typ  SetTextSelectionType
		want string
	}{
		{SetTextSelectionStart, "SetTextSelectionStart"},
		{SetTextSelectionEnd, "SetTextSelectionEnd"},
		{SetTextSelectionReset, "SetTextSelectionReset"},
		{SetTextSelectionType(99), "SetTextSelectionType(99)"},
	}
	for _, tc := range cases {
		if got := tc.typ.String(); got != tc.want {
			t.Errorf("%d: got %q, want %q", tc.typ, got, tc.want)
		}
	}
}

func TestSetGraphicSelectionType_String(t *testing.T) {
	cases := []struct {
		typ  SetGraphicSelectionType
		want string
	}{
		{SetGraphicSelectionStart, "SetGraphicSelectionStart"},
		{SetGraphicSelectionEnd, "SetGraphicSelectionEnd"},
		{SetGraphicSelectionType(99), "SetGraphicSelectionType(99)"},
	}
	for _, tc := range cases {
		if got := tc.typ.String(); got != tc.want {
			t.Errorf("%d: got %q, want %q", tc.typ, got, tc.want)
		}
	}
}

func TestSelectionKind_String(t *testing.T) {
	cases := []struct {
		k    SelectionKind
		want string
	}{
		{SelectionKindNone, "SelectionKindNone"},
		{SelectionKindText, "SelectionKindText"},
		{SelectionKindComplex, "SelectionKindComplex"},
		{SelectionKind(99), "SelectionKind(99)"},
	}
	for _, tc := range cases {
		if got := tc.k.String(); got != tc.want {
			t.Errorf("%d: got %q, want %q", tc.k, got, tc.want)
		}
	}
}
