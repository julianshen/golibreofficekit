//go:build linux || darwin

package lok

import (
	"bytes"
	"image"
	"image/png"
)

// twipsPerInchPng is the LOK twip→inch ratio. Render* methods convert
// (twipDim * 96 * dpiScale) / twipsPerInchPng to pixels — so a 1500-twip
// width at dpiScale=1.0 is 100 px.
const (
	twipsPerInchPng = 1440
	defaultDPI      = 96
)

// RenderImage renders the active part of the document as a single
// premultiplied-corrected NRGBA image, sized to the document's
// twip extent at the given DPI scale (1.0 = 96 DPI).
//
// For multi-part documents (Calc, Impress) the active part is what
// gets rendered; call SetPart first to choose another. Writer-style
// docs render their entire layout (all pages stacked vertically).
//
// Requires a prior InitializeForRendering. Returns *LOKError when
// dpiScale is non-positive or the document reports a zero size.
func (d *Document) RenderImage(dpiScale float64) (*image.NRGBA, error) {
	pxW, pxH, twipW, twipH, err := d.renderTargetSize("RenderImage", dpiScale)
	if err != nil {
		return nil, err
	}
	return d.PaintTile(pxW, pxH, TwipRect{X: 0, Y: 0, W: twipW, H: twipH})
}

// RenderPNG is RenderImage followed by png.Encode. For callers that
// only want the encoded bytes (e.g. an HTTP handler returning a
// page preview), this skips the intermediate *image.NRGBA exposure.
func (d *Document) RenderPNG(dpiScale float64) ([]byte, error) {
	img, err := d.RenderImage(dpiScale)
	if err != nil {
		return nil, err
	}
	return encodePNG("RenderPNG", img)
}

// encodePNG is the common png.Encode wrapper used by RenderPNG and
// RenderPagePNG. op labels the *LOKError so error messages identify
// the high-level caller.
func encodePNG(op string, img *image.NRGBA) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, &LOKError{Op: op, Detail: err.Error(), err: err}
	}
	return buf.Bytes(), nil
}

// renderTargetSize resolves the rendering rect and pixel dimensions
// for the document's current state. op labels the *LOKError so error
// messages identify the high-level caller (RenderImage / RenderPNG).
func (d *Document) renderTargetSize(op string, dpiScale float64) (pxW, pxH int, twipW, twipH int64, err error) {
	if dpiScale <= 0 {
		return 0, 0, 0, 0, &LOKError{Op: op, Detail: "dpiScale must be > 0"}
	}
	twipW, twipH, err = d.DocumentSize()
	if err != nil {
		return 0, 0, 0, 0, err
	}
	if twipW <= 0 || twipH <= 0 {
		return 0, 0, 0, 0, &LOKError{Op: op, Detail: "DocumentSize is zero; nothing to render"}
	}
	pxW = twipsToPixels(twipW, dpiScale)
	pxH = twipsToPixels(twipH, dpiScale)
	return pxW, pxH, twipW, twipH, nil
}

// twipsToPixels converts a twip dimension to pixels at LO's native
// 96 DPI scaled by dpiScale, rounding to the nearest pixel.
func twipsToPixels(twips int64, dpiScale float64) int {
	return int(float64(twips)*defaultDPI*dpiScale/twipsPerInchPng + 0.5)
}
