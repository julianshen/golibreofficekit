//go:build linux || darwin

package lok

import "fmt"

// SetTextSelectionType mirrors LOK_SETTEXTSELECTION_*.
type SetTextSelectionType int

const (
	SetTextSelectionStart SetTextSelectionType = 0 // LOK_SETTEXTSELECTION_START
	SetTextSelectionEnd   SetTextSelectionType = 1 // LOK_SETTEXTSELECTION_END
	SetTextSelectionReset SetTextSelectionType = 2 // LOK_SETTEXTSELECTION_RESET
)

func (t SetTextSelectionType) String() string {
	switch t {
	case SetTextSelectionStart:
		return "SetTextSelectionStart"
	case SetTextSelectionEnd:
		return "SetTextSelectionEnd"
	case SetTextSelectionReset:
		return "SetTextSelectionReset"
	default:
		return fmt.Sprintf("SetTextSelectionType(%d)", int(t))
	}
}

// SetGraphicSelectionType mirrors LOK_SETGRAPHICSELECTION_*.
type SetGraphicSelectionType int

const (
	SetGraphicSelectionStart SetGraphicSelectionType = 0 // LOK_SETGRAPHICSELECTION_START
	SetGraphicSelectionEnd   SetGraphicSelectionType = 1 // LOK_SETGRAPHICSELECTION_END
)

func (t SetGraphicSelectionType) String() string {
	switch t {
	case SetGraphicSelectionStart:
		return "SetGraphicSelectionStart"
	case SetGraphicSelectionEnd:
		return "SetGraphicSelectionEnd"
	default:
		return fmt.Sprintf("SetGraphicSelectionType(%d)", int(t))
	}
}

// SelectionKind mirrors LOK_SELTYPE_*. LARGE_TEXT is not surfaced as
// a distinct kind — the LOK header documents it as "unused (same as
// LOK_SELTYPE_COMPLEX)" and code that receives it folds to
// SelectionKindComplex.
type SelectionKind int

const (
	SelectionKindNone    SelectionKind = 0 // LOK_SELTYPE_NONE
	SelectionKindText    SelectionKind = 1 // LOK_SELTYPE_TEXT
	SelectionKindComplex SelectionKind = 3 // LOK_SELTYPE_COMPLEX (LARGE_TEXT = 2 folds here)
)

// selectionKindFromLOK normalises a raw LOK int into SelectionKind.
// LOK_SELTYPE_LARGE_TEXT (2) is folded into SelectionKindComplex.
// Any other unknown value is returned verbatim so callers can log
// the surprise.
func selectionKindFromLOK(v int) SelectionKind {
	switch v {
	case 0:
		return SelectionKindNone
	case 1:
		return SelectionKindText
	case 2, 3:
		return SelectionKindComplex
	default:
		return SelectionKind(v)
	}
}

func (k SelectionKind) String() string {
	switch k {
	case SelectionKindNone:
		return "SelectionKindNone"
	case SelectionKindText:
		return "SelectionKindText"
	case SelectionKindComplex:
		return "SelectionKindComplex"
	default:
		return fmt.Sprintf("SelectionKind(%d)", int(k))
	}
}
