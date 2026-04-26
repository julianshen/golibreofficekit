//go:build linux || darwin

package lok

import (
	"encoding/json"
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

// GetCommandValues returns a JSON document describing the current
// state/possible values for command. The returned JSON is specific to
// the command; see LibreOfficeKitEnums.h for command names and
// expected payload formats.
//
// Common commands:
//   ".uno:Save"                     — always enabled when document is modifiable
//   ".uno:Undo" / ".uno:Redo"       — enabled/disabled state
//   ".uno:Bold" / ".uno:Italic"     — checked state
//   ".uno:FontName"                 — list of available fonts
//   ".uno:StyleApply"               — list of styles
//   ".uno:CharFontName"             — current font
//
// Returns ErrUnsupported if LOK does not implement getCommandValues for this
// build. Returns a non-nil error for invalid commands or closed documents.
func (d *Document) GetCommandValues(command string) (json.RawMessage, error) {
	unlock, err := d.guard()
	if err != nil {
		return nil, err
	}
	defer unlock()
	s, err := d.office.be.GetCommandValues(d.h, command)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(s), nil
}

// CompleteFunction attempts to complete a function (formula) in a spreadsheet.
// name is the function name. Returns an error if the function cannot be completed.
// This is a no-op for non-Calc documents.
func (d *Document) CompleteFunction(name string) error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	return d.office.be.CompleteFunction(d.h, name)
}

// IsCommandEnabled returns whether command is currently enabled.
// Returns an error if the command JSON cannot be parsed or when the
// "enabled" or "state" fields are present but of an invalid type/value.
// When those fields are missing (absent), they are treated as disabled
// and the function returns (false, nil).
func (d *Document) IsCommandEnabled(cmd string) (bool, error) {
	raw, err := d.GetCommandValues(cmd)
	if err != nil {
		return false, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return false, err
	}
	if v, ok := m["enabled"].(bool); ok {
		return v, nil
	}
	if v, ok := m["state"].(bool); ok {
		return v, nil
	}
	return false, nil
}

// GetFontNames returns the list of available font names.
// Returns an error if the command JSON cannot be parsed. When the "value"
// field is absent or not a list, returns an empty slice (not nil).
func (d *Document) GetFontNames() ([]string, error) {
	raw, err := d.GetCommandValues(".uno:FontName")
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	if v, ok := m["value"].([]interface{}); ok {
		names := make([]string, len(v))
		for i, x := range v {
			names[i] = fmt.Sprint(x)
		}
		return names, nil
	}
	return []string{}, nil
}
