//go:build linux || darwin

package lok

import "fmt"

// EventType mirrors LOK_CALLBACK_*. The named constants below are a
// curated subset; values outside the named set still arrive as
// EventType(N). Run `grep LOK_CALLBACK_ third_party/lok/.../LibreOfficeKitEnums.h`
// for the full list LOK ships.
type EventType int

const (
	EventTypeInvalidateTiles         EventType = 0  // LOK_CALLBACK_INVALIDATE_TILES
	EventTypeInvalidateVisibleCursor EventType = 1  // LOK_CALLBACK_INVALIDATE_VISIBLE_CURSOR
	EventTypeTextSelection           EventType = 2  // LOK_CALLBACK_TEXT_SELECTION
	EventTypeTextSelectionStart      EventType = 3  // LOK_CALLBACK_TEXT_SELECTION_START
	EventTypeTextSelectionEnd        EventType = 4  // LOK_CALLBACK_TEXT_SELECTION_END
	EventTypeCursorVisible           EventType = 5  // LOK_CALLBACK_CURSOR_VISIBLE
	EventTypeGraphicSelection        EventType = 6  // LOK_CALLBACK_GRAPHIC_SELECTION
	EventTypeHyperlinkClicked        EventType = 7  // LOK_CALLBACK_HYPERLINK_CLICKED
	EventTypeStateChanged            EventType = 8  // LOK_CALLBACK_STATE_CHANGED
	EventTypeDocumentSizeChanged     EventType = 13 // LOK_CALLBACK_DOCUMENT_SIZE_CHANGED
	EventTypeSetPart                 EventType = 14 // LOK_CALLBACK_SET_PART
	EventTypeUNOCommandResult        EventType = 16 // LOK_CALLBACK_UNO_COMMAND_RESULT
	EventTypeMousePointer            EventType = 18 // LOK_CALLBACK_MOUSE_POINTER
	EventTypeError                   EventType = 22 // LOK_CALLBACK_ERROR
	EventTypeWindow                  EventType = 36 // LOK_CALLBACK_WINDOW
)

func (t EventType) String() string {
	switch t {
	case EventTypeInvalidateTiles:
		return "EventTypeInvalidateTiles"
	case EventTypeInvalidateVisibleCursor:
		return "EventTypeInvalidateVisibleCursor"
	case EventTypeTextSelection:
		return "EventTypeTextSelection"
	case EventTypeTextSelectionStart:
		return "EventTypeTextSelectionStart"
	case EventTypeTextSelectionEnd:
		return "EventTypeTextSelectionEnd"
	case EventTypeCursorVisible:
		return "EventTypeCursorVisible"
	case EventTypeGraphicSelection:
		return "EventTypeGraphicSelection"
	case EventTypeHyperlinkClicked:
		return "EventTypeHyperlinkClicked"
	case EventTypeStateChanged:
		return "EventTypeStateChanged"
	case EventTypeMousePointer:
		return "EventTypeMousePointer"
	case EventTypeUNOCommandResult:
		return "EventTypeUNOCommandResult"
	case EventTypeDocumentSizeChanged:
		return "EventTypeDocumentSizeChanged"
	case EventTypeSetPart:
		return "EventTypeSetPart"
	case EventTypeError:
		return "EventTypeError"
	case EventTypeWindow:
		return "EventTypeWindow"
	default:
		return fmt.Sprintf("EventType(%d)", int(t))
	}
}

// Event is one delivered LOK callback. Payload is a Go-owned copy of
// the C-allocated string LOK provided. Format depends on Type — see
// LibreOfficeKitEnums.h. Empty Payload is common; LOK signals many
// events with no body.
type Event struct {
	Type    EventType
	Payload []byte
}
