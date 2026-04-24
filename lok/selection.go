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

// validateMime rejects empty / NUL-containing / > 256-byte MIME
// strings. LOK does its own structural validation; this catches the
// cases where we would crash or corrupt the C side before reaching
// it (embedded NUL truncates at C.CString).
func validateMime(s string) error {
	if s == "" || len(s) > 256 {
		return &LOKError{Op: "mime", Detail: "mime type must be non-empty and <= 256 bytes", err: ErrInvalidOption}
	}
	for i := 0; i < len(s); i++ {
		if s[i] == 0 {
			return &LOKError{Op: "mime", Detail: "mime type contains NUL byte", err: ErrInvalidOption}
		}
	}
	return nil
}

// GetTextSelection copies the current text selection as mimeType.
// LOK may substitute a different, compatible mime, which is returned
// in usedMime.
func (d *Document) GetTextSelection(mimeType string) (text, usedMime string, err error) {
	if err := validateMime(mimeType); err != nil {
		return "", "", err
	}
	unlock, gerr := d.guard()
	if gerr != nil {
		return "", "", gerr
	}
	defer unlock()
	t, m := d.office.be.DocumentGetTextSelection(d.h, mimeType)
	return t, m, nil
}

// GetSelectionKind reports what kind of selection is currently
// active without copying any text. Works on all supported LO
// versions.
func (d *Document) GetSelectionKind() (SelectionKind, error) {
	unlock, err := d.guard()
	if err != nil {
		return SelectionKindNone, err
	}
	defer unlock()
	return selectionKindFromLOK(d.office.be.DocumentGetSelectionType(d.h)), nil
}

// GetSelectionTypeAndText returns the selection kind and the
// selected text in a single LOK call. Requires LibreOffice >= 7.4;
// returns ErrUnsupported on older builds.
func (d *Document) GetSelectionTypeAndText(mimeType string) (kind SelectionKind, text, usedMime string, err error) {
	if verr := validateMime(mimeType); verr != nil {
		return SelectionKindNone, "", "", verr
	}
	unlock, gerr := d.guard()
	if gerr != nil {
		return SelectionKindNone, "", "", gerr
	}
	defer unlock()
	k, t, m, ierr := d.office.be.DocumentGetSelectionTypeAndText(d.h, mimeType)
	if ierr != nil {
		return SelectionKindNone, "", "", ierr
	}
	return selectionKindFromLOK(k), t, m, nil
}

// SetTextSelection drags the selection handle of kind typ to the
// document position (x, y) in twips.
func (d *Document) SetTextSelection(typ SetTextSelectionType, x, y int64) error {
	switch typ {
	case SetTextSelectionStart, SetTextSelectionEnd, SetTextSelectionReset:
		// valid
	default:
		return &LOKError{Op: "SetTextSelection", Detail: fmt.Sprintf("type out of range: %d", int(typ)), err: ErrInvalidOption}
	}
	if err := requireInt32XY("SetTextSelection", x, y); err != nil {
		return err
	}
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	d.office.be.DocumentSetTextSelection(d.h, int(typ), int(x), int(y))
	return nil
}

// ResetSelection clears the current selection.
func (d *Document) ResetSelection() error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	d.office.be.DocumentResetSelection(d.h)
	return nil
}

// SetGraphicSelection drags a graphic-selection handle at (x, y)
// twips. typ = Start begins the drag; typ = End completes it.
func (d *Document) SetGraphicSelection(typ SetGraphicSelectionType, x, y int64) error {
	switch typ {
	case SetGraphicSelectionStart, SetGraphicSelectionEnd:
		// valid
	default:
		return &LOKError{Op: "SetGraphicSelection", Detail: fmt.Sprintf("type out of range: %d", int(typ)), err: ErrInvalidOption}
	}
	if err := requireInt32XY("SetGraphicSelection", x, y); err != nil {
		return err
	}
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	d.office.be.DocumentSetGraphicSelection(d.h, int(typ), int(x), int(y))
	return nil
}

// SetBlockedCommandList blocks the comma-separated set of UNO
// commands (csv) for the given view.
func (d *Document) SetBlockedCommandList(viewID int, csv string) error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	d.office.be.DocumentSetBlockedCommandList(d.h, viewID, csv)
	return nil
}
