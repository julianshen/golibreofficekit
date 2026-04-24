//go:build linux || darwin

package lok

import (
	"fmt"
	"math"
)

// The helpers below are thin named wrappers around PostUnoCommand for
// common .uno:* commands. They share PostUnoCommand's contract on
// observability: effects are visible only through a registered
// document callback.

func (d *Document) Bold() error      { return d.PostUnoCommand(".uno:Bold", "", false) }
func (d *Document) Italic() error    { return d.PostUnoCommand(".uno:Italic", "", false) }
func (d *Document) Underline() error { return d.PostUnoCommand(".uno:Underline", "", false) }
func (d *Document) Undo() error      { return d.PostUnoCommand(".uno:Undo", "", false) }
func (d *Document) Redo() error      { return d.PostUnoCommand(".uno:Redo", "", false) }
func (d *Document) Copy() error      { return d.PostUnoCommand(".uno:Copy", "", false) }
func (d *Document) Cut() error       { return d.PostUnoCommand(".uno:Cut", "", false) }

// Paste inserts clipboard content at the caret, replacing any selection.
func (d *Document) Paste() error { return d.PostUnoCommand(".uno:Paste", "", false) }

func (d *Document) SelectAll() error { return d.PostUnoCommand(".uno:SelectAll", "", false) }

func (d *Document) InsertPageBreak() error {
	return d.PostUnoCommand(".uno:InsertPageBreak", "", false)
}

// InsertTable inserts a table with the given row and column counts
// at the caret. The args payload is LOK's awt::Any JSON (UNO's "long"
// type is 32-bit), so rows and cols must be in [1, math.MaxInt32];
// values outside that range return *LOKError{Op:"InsertTable"}
// without invoking LOK.
func (d *Document) InsertTable(rows, cols int) error {
	if rows < 1 || cols < 1 || rows > math.MaxInt32 || cols > math.MaxInt32 {
		return &LOKError{Op: "InsertTable",
			Detail: fmt.Sprintf("rows and cols must be in [1, %d]: rows=%d cols=%d",
				math.MaxInt32, rows, cols)}
	}
	args := fmt.Sprintf(`{"Columns":{"type":"long","value":%d},"Rows":{"type":"long","value":%d}}`, cols, rows)
	return d.PostUnoCommand(".uno:InsertTable", args, false)
}
