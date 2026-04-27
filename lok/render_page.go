//go:build linux || darwin

package lok

import (
	"fmt"
	"image"
)

// RenderPage renders a single page of the document at the given DPI
// scale (1.0 = 96 DPI). Page semantics depend on document type:
//
//   - Calc:    page i is sheet i (0-based)
//   - Impress: page i is slide i (0-based)
//   - Draw:    page i is page i (0-based)
//   - Writer:  page i is the i'th page within the current part,
//     using PartPageRectangles to bound the source rect
//
// Multi-part documents (Calc/Impress/Draw) restore the active part
// after rendering, so RenderPage(2, …) followed by RenderPage(0, …)
// leaves the document on whatever part was active before either call.
//
// Requires a prior InitializeForRendering. Out-of-range page indices
// return *LOKError.
func (d *Document) RenderPage(page int, dpiScale float64) (*image.NRGBA, error) {
	if dpiScale <= 0 {
		return nil, &LOKError{Op: "RenderPage", Detail: "dpiScale must be > 0"}
	}
	unlock, err := d.guard()
	if err != nil {
		return nil, err
	}
	defer unlock()
	if !d.tileModeReady {
		return nil, &LOKError{Op: "RenderPage", Detail: "InitializeForRendering not called"}
	}

	parts := d.office.be.DocumentGetParts(d.h)
	if parts > 0 {
		if page < 0 || page >= parts {
			return nil, &LOKError{Op: "RenderPage",
				Detail: fmt.Sprintf("page %d out of range [0, %d)", page, parts)}
		}
		return d.renderMultiPartPage(page, dpiScale)
	}
	return d.renderWriterPage(page, dpiScale)
}

// RenderPagePNG is RenderPage followed by png.Encode.
func (d *Document) RenderPagePNG(page int, dpiScale float64) ([]byte, error) {
	img, err := d.RenderPage(page, dpiScale)
	if err != nil {
		return nil, err
	}
	return encodePNG("RenderPagePNG", img)
}

// renderWriterPage handles the Writer/single-part case via
// PartPageRectangles + PaintTile. The lock is held by the caller.
func (d *Document) renderWriterPage(page int, dpiScale float64) (*image.NRGBA, error) {
	raw := d.office.be.DocumentGetPartPageRectangles(d.h)
	if raw == "" {
		return nil, &LOKError{Op: "RenderPage", Detail: "no pages reported by LO (PartPageRectangles empty)"}
	}
	rects, err := parsePartPageRectangles(raw)
	if err != nil {
		return nil, err
	}
	if page < 0 || page >= len(rects) {
		return nil, &LOKError{Op: "RenderPage",
			Detail: fmt.Sprintf("page %d out of range [0, %d)", page, len(rects))}
	}
	r := rects[page]
	return d.paintToNRGBA(r, dpiScale, func(buf []byte, pxW, pxH int) {
		d.office.be.DocumentPaintTile(d.h, buf, pxW, pxH,
			int(r.X), int(r.Y), int(r.W), int(r.H))
	})
}

// renderMultiPartPage handles Calc/Impress/Draw via SetPart +
// DocumentSize + PaintPartTile. The active part is saved and
// restored. Caller has already bounds-checked page and holds the
// document lock.
func (d *Document) renderMultiPartPage(page int, dpiScale float64) (*image.NRGBA, error) {
	// LO can return -1 when the active view was destroyed; restoring
	// to -1 leaves the doc in an undocumented state. Skip the restore
	// in that case so we don't make things worse, and only restore
	// when we actually moved the active part.
	active := d.office.be.DocumentGetPart(d.h)
	d.office.be.DocumentSetPart(d.h, page)
	if active >= 0 && active != page {
		defer d.office.be.DocumentSetPart(d.h, active)
	}

	w, h := d.office.be.DocumentGetDocumentSize(d.h)
	if w <= 0 || h <= 0 {
		return nil, &LOKError{Op: "RenderPage",
			Detail: fmt.Sprintf("page %d has zero DocumentSize", page)}
	}
	r := TwipRect{W: w, H: h}
	return d.paintToNRGBA(r, dpiScale, func(buf []byte, pxW, pxH int) {
		d.office.be.DocumentPaintPartTile(d.h, buf, page, 0, pxW, pxH, 0, 0, int(w), int(h))
	})
}

// paintToNRGBA validates the twip rect, converts to pixel dimensions
// at dpiScale, allocates the NRGBA, runs paint to fill img.Pix with
// premultiplied BGRA, then unpremultiplies in place. Caller holds
// the document lock; paint must be a synchronous backend call that
// fills exactly len(img.Pix) bytes.
func (d *Document) paintToNRGBA(r TwipRect, dpiScale float64, paint func(buf []byte, pxW, pxH int)) (*image.NRGBA, error) {
	const op = "RenderPage"
	if r.W <= 0 || r.H <= 0 {
		return nil, &LOKError{Op: op, Detail: "page has zero dimensions"}
	}
	if err := requireInt32Rect(op, r); err != nil {
		return nil, err
	}
	pxW := twipsToPixels(r.W, dpiScale)
	pxH := twipsToPixels(r.H, dpiScale)
	if pxW <= 0 || pxH <= 0 {
		return nil, &LOKError{Op: op, Detail: "rendered size collapses to zero pixels"}
	}
	img := image.NewNRGBA(imageBoundsForTile(pxW, pxH))
	paint(img.Pix, pxW, pxH)
	unpremultiplyBGRAToNRGBA(img.Pix, img.Pix, pxW, pxH)
	return img, nil
}
