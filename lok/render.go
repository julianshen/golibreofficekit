//go:build linux || darwin

package lok

import (
	"fmt"
	"image"
	"math"
)

// lokTileModeBGRA is LOK's LOK_TILEMODE_BGRA — Cairo ARGB32 byte order
// (B, G, R, A with premultiplied alpha). The binding refuses any other
// tile mode at InitializeForRendering time.
const lokTileModeBGRA = 1

// InitializeForRendering prepares the document for tile painting and
// verifies LOK is configured for premultiplied BGRA output. args is
// an opaque JSON hint string passed through to LOK (empty is valid).
// Must be called before any Paint* or Render* method; subsequent
// paints use the cached tile-mode check.
func (d *Document) InitializeForRendering(args string) error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	d.office.be.DocumentInitializeForRendering(d.h, args)
	mode := d.office.be.DocumentGetTileMode(d.h)
	if mode != lokTileModeBGRA {
		return &LOKError{Op: "InitializeForRendering", Detail: fmt.Sprintf("unsupported tile mode %d (binding requires LOK_TILEMODE_BGRA)", mode)}
	}
	d.tileModeReady = true
	return nil
}

// SetClientZoom tells LOK the caller's render scale. Fire-and-forget;
// a nil return does not confirm LOK applied the values. Does NOT
// require a prior InitializeForRendering — zoom is a cache hint.
func (d *Document) SetClientZoom(tilePxW, tilePxH, tileTwipW, tileTwipH int) error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	d.office.be.DocumentSetClientZoom(d.h, tilePxW, tilePxH, tileTwipW, tileTwipH)
	return nil
}

// SetClientVisibleArea tells LOK the client's visible region in twips.
// Helps LOK prefetch tiles; does NOT require InitializeForRendering.
// Any field beyond math.MaxInt32 returns *LOKError — LOK's C ABI
// takes int (32-bit) and we refuse to silently truncate.
func (d *Document) SetClientVisibleArea(r TwipRect) error {
	if err := requireInt32Rect("SetClientVisibleArea", r); err != nil {
		return err
	}
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	d.office.be.DocumentSetClientVisibleArea(d.h, int(r.X), int(r.Y), int(r.W), int(r.H))
	return nil
}

// requireInt32Rect returns *LOKError if any rect field exceeds
// math.MaxInt32. LOK's tile-position and visible-area ABI takes C int
// (32-bit on LP64); truncation would silently corrupt LOK's internal
// coordinates. Negative values are legal (LO uses them for offsets).
func requireInt32Rect(op string, r TwipRect) error {
	if r.X > math.MaxInt32 || r.X < math.MinInt32 ||
		r.Y > math.MaxInt32 || r.Y < math.MinInt32 ||
		r.W > math.MaxInt32 || r.W < math.MinInt32 ||
		r.H > math.MaxInt32 || r.H < math.MinInt32 {
		return &LOKError{Op: op, Detail: fmt.Sprintf("rect field exceeds int32 range: %+v", r)}
	}
	return nil
}

// imageBoundsForTile returns a Go image.Rectangle matching a pxW×pxH
// tile. Private; callers compose with image.NewNRGBA.
func imageBoundsForTile(pxW, pxH int) image.Rectangle {
	return image.Rect(0, 0, pxW, pxH)
}
