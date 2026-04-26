//go:build linux || darwin

package lok

// PasteData inserts data of the given MIME type into the document at
// the current cursor location. This is LOK's direct paste API and is
// distinct from Document.Paste, the .uno:Paste convenience wrapper
// that operates on the system clipboard.
//
// mimeType describes the data format (e.g. "text/plain", "image/png").
// Returns *LOKError when mimeType is empty, ErrClosed on a closed
// document, ErrUnsupported when the LO build does not expose paste.
func (d *Document) PasteData(mimeType string, data []byte) error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	if mimeType == "" {
		return &LOKError{Op: "PasteData", Detail: "mimeType is required"}
	}
	return d.office.be.DocumentPaste(d.h, mimeType, data)
}

// SelectPart adjusts the selection state of a part (Calc sheet,
// Impress slide, Draw page). The selected flag maps to LOK's nSelect
// argument; see LibreOfficeKit.h for the per-doc-type interpretation
// (Impress treats nSelect as a tri-state where 2 means toggle).
// Returns ErrClosed on a closed document.
func (d *Document) SelectPart(part int, selected bool) error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	return d.office.be.DocumentSelectPart(d.h, part, selected)
}

// MoveSelectedParts moves the currently selected parts to position.
// If duplicate is true the parts are copied instead of moved.
// Returns ErrClosed on a closed document.
func (d *Document) MoveSelectedParts(position int, duplicate bool) error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	return d.office.be.DocumentMoveSelectedParts(d.h, position, duplicate)
}

// RenderFont renders char in fontName as a 32-bit-per-pixel bitmap
// matching LO's tile-rendering format on this platform (premultiplied
// BGRA on little-endian targets, the same shape PaintTileRaw produces;
// callers targeting big-endian builds should consult Document.TileMode
// before assuming a byte order). The returned slice has length 4*w*h.
// If fontName is empty the LO default font is used.
// Returns ErrClosed on a closed document and ErrUnsupported when the
// LO build does not expose renderFont.
func (d *Document) RenderFont(fontName, char string) ([]byte, int, int, error) {
	unlock, err := d.guard()
	if err != nil {
		return nil, 0, 0, err
	}
	defer unlock()
	return d.office.be.DocumentRenderFont(d.h, fontName, char)
}
