//go:build linux || darwin

package lok

import "testing"

func TestEventType_String(t *testing.T) {
	cases := []struct {
		typ  EventType
		want string
	}{
		{EventTypeInvalidateTiles, "EventTypeInvalidateTiles"},
		{EventTypeInvalidateVisibleCursor, "EventTypeInvalidateVisibleCursor"},
		{EventTypeTextSelection, "EventTypeTextSelection"},
		{EventTypeTextSelectionStart, "EventTypeTextSelectionStart"},
		{EventTypeTextSelectionEnd, "EventTypeTextSelectionEnd"},
		{EventTypeCursorVisible, "EventTypeCursorVisible"},
		{EventTypeGraphicSelection, "EventTypeGraphicSelection"},
		{EventTypeHyperlinkClicked, "EventTypeHyperlinkClicked"},
		{EventTypeStateChanged, "EventTypeStateChanged"},
		{EventTypeMousePointer, "EventTypeMousePointer"},
		{EventTypeUNOCommandResult, "EventTypeUNOCommandResult"},
		{EventTypeDocumentSizeChanged, "EventTypeDocumentSizeChanged"},
		{EventTypeSetPart, "EventTypeSetPart"},
		{EventTypeError, "EventTypeError"},
		{EventTypeWindow, "EventTypeWindow"},
		{EventTypeSignatureStatus, "EventTypeSignatureStatus"},
		{EventType(999), "EventType(999)"},
	}
	for _, tc := range cases {
		if got := tc.typ.String(); got != tc.want {
			t.Errorf("%d: got %q, want %q", tc.typ, got, tc.want)
		}
	}
}
