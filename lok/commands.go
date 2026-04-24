//go:build linux || darwin

package lok

import (
	"fmt"
	"math"
)

// Bold toggles bold on the current selection. Equivalent to
// PostUnoCommand(".uno:Bold", "", false).
func (d *Document) Bold() error { return d.PostUnoCommand(".uno:Bold", "", false) }

// Italic toggles italic on the current selection.
func (d *Document) Italic() error { return d.PostUnoCommand(".uno:Italic", "", false) }

// Underline toggles underline on the current selection.
func (d *Document) Underline() error { return d.PostUnoCommand(".uno:Underline", "", false) }

// Undo reverses the most recent editing action.
func (d *Document) Undo() error { return d.PostUnoCommand(".uno:Undo", "", false) }

// Redo re-applies the most recently undone action.
func (d *Document) Redo() error { return d.PostUnoCommand(".uno:Redo", "", false) }

// Copy copies the current selection to the system clipboard.
func (d *Document) Copy() error { return d.PostUnoCommand(".uno:Copy", "", false) }

// Cut removes the current selection and places it on the clipboard.
func (d *Document) Cut() error { return d.PostUnoCommand(".uno:Cut", "", false) }

// Paste inserts the clipboard content at the caret.
func (d *Document) Paste() error { return d.PostUnoCommand(".uno:Paste", "", false) }

// SelectAll selects the entire document content.
func (d *Document) SelectAll() error { return d.PostUnoCommand(".uno:SelectAll", "", false) }

// InsertPageBreak inserts a page break at the caret.
func (d *Document) InsertPageBreak() error {
	return d.PostUnoCommand(".uno:InsertPageBreak", "", false)
}

// InsertTable inserts a table with the given row and column counts
// at the caret. Builds LOK's awt::Any JSON args internally. rows
// and cols must be in [1, math.MaxInt32] — UNO's "long" type is
// 32-bit, so values outside that range cannot be represented and
// zero/negative dimensions produce no table.
func (d *Document) InsertTable(rows, cols int) error {
	if rows < 1 || cols < 1 || rows > math.MaxInt32 || cols > math.MaxInt32 {
		return &LOKError{Op: "InsertTable",
			Detail: fmt.Sprintf("rows and cols must be in [1, %d]: rows=%d cols=%d",
				math.MaxInt32, rows, cols)}
	}
	args := fmt.Sprintf(`{"Columns":{"type":"long","value":%d},"Rows":{"type":"long","value":%d}}`, cols, rows)
	return d.PostUnoCommand(".uno:InsertTable", args, false)
}
