//go:build linux || darwin

package lok

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// TwipRect is a rectangle in LOK's twip coordinates (1/1440 inch).
type TwipRect struct {
	X, Y, W, H int64
}

// Parts returns the number of parts (sheets/pages/slides).
func (d *Document) Parts() (int, error) {
	unlock, err := d.guard()
	if err != nil {
		return 0, err
	}
	defer unlock()
	n := d.office.be.DocumentGetParts(d.h)
	if n < 0 {
		return 0, &LOKError{Op: "Parts", Detail: "LOK returned -1"}
	}
	return n, nil
}

// Part returns the currently-active part index.
func (d *Document) Part() (int, error) {
	unlock, err := d.guard()
	if err != nil {
		return 0, err
	}
	defer unlock()
	n := d.office.be.DocumentGetPart(d.h)
	if n < 0 {
		return 0, &LOKError{Op: "Part", Detail: "LOK returned -1"}
	}
	return n, nil
}

// SetPart activates the part at index n.
func (d *Document) SetPart(n int) error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	d.office.be.DocumentSetPart(d.h, n)
	return nil
}

// SetPartMode switches the part-mode (Calc's "view" mode, etc.).
// Values are the LOK_PARTMODE_* enums from LibreOfficeKitEnums.h.
func (d *Document) SetPartMode(mode int) error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	d.office.be.DocumentSetPartMode(d.h, mode)
	return nil
}

// PartName returns the display name of the given part.
func (d *Document) PartName(n int) (string, error) {
	unlock, err := d.guard()
	if err != nil {
		return "", err
	}
	defer unlock()
	return d.office.be.DocumentGetPartName(d.h, n), nil
}

// PartHash returns the stable content hash of the given part.
func (d *Document) PartHash(n int) (string, error) {
	unlock, err := d.guard()
	if err != nil {
		return "", err
	}
	defer unlock()
	return d.office.be.DocumentGetPartHash(d.h, n), nil
}

// PartInfo returns the part's LOK JSON metadata as json.RawMessage,
// or (nil, nil) when LOK returns an empty string. Writer and Calc
// documents legitimately return empty — only Impress populates
// per-part info in LOK 24.8. Callers that require populated info
// should check `raw == nil` and act accordingly; this is not an
// error condition at the binding level.
func (d *Document) PartInfo(n int) (json.RawMessage, error) {
	unlock, err := d.guard()
	if err != nil {
		return nil, err
	}
	defer unlock()
	raw := d.office.be.DocumentGetPartInfo(d.h, n)
	if raw == "" {
		return nil, nil
	}
	return json.RawMessage(raw), nil
}

// DocumentSize returns the document's (width, height) in twips.
func (d *Document) DocumentSize() (widthTwips, heightTwips int64, err error) {
	unlock, gerr := d.guard()
	if gerr != nil {
		return 0, 0, gerr
	}
	defer unlock()
	w, h := d.office.be.DocumentGetDocumentSize(d.h)
	return w, h, nil
}

// PartPageRectangles returns the page rectangles for the current
// part in twip coordinates. An empty LOK response yields (nil, nil).
// Malformed LOK output surfaces as *LOKError.
func (d *Document) PartPageRectangles() ([]TwipRect, error) {
	unlock, err := d.guard()
	if err != nil {
		return nil, err
	}
	defer unlock()
	raw := d.office.be.DocumentGetPartPageRectangles(d.h)
	if raw == "" {
		return nil, nil
	}
	return parsePartPageRectangles(raw)
}

// SetOutlineState toggles outline-group visibility. column=true for
// Calc column grouping, false for row grouping. level is the outline
// depth; index is the group index at that level. hidden collapses
// the group when true.
func (d *Document) SetOutlineState(column bool, level, index int, hidden bool) error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	d.office.be.DocumentSetOutlineState(d.h, column, level, index, hidden)
	return nil
}

// parsePartPageRectangles parses LOK's "x, y, w, h; x, y, w, h; …"
// format into a []TwipRect. Empty input yields (nil, nil). Trailing
// semicolons and whitespace are tolerated. Malformed input surfaces
// as *LOKError.
func parsePartPageRectangles(s string) ([]TwipRect, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	groups := strings.Split(s, ";")
	out := make([]TwipRect, 0, len(groups))
	for _, g := range groups {
		g = strings.TrimSpace(g)
		if g == "" {
			continue
		}
		fields := strings.Split(g, ",")
		if len(fields) != 4 {
			return nil, &LOKError{Op: "PartPageRectangles", Detail: fmt.Sprintf("expected 4 fields, got %d: %q", len(fields), g)}
		}
		vals := [4]int64{}
		for i, f := range fields {
			v, err := strconv.ParseInt(strings.TrimSpace(f), 10, 64)
			if err != nil {
				return nil, &LOKError{Op: "PartPageRectangles", Detail: err.Error(), err: err}
			}
			vals[i] = v
		}
		out = append(out, TwipRect{X: vals[0], Y: vals[1], W: vals[2], H: vals[3]})
	}
	return out, nil
}
