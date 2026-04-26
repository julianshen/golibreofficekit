//go:build linux || darwin

package lok

// PaintWindow paints a window into the provided buffer. The buffer must
// have length 4*pxW*pxH. Returns premultiplied BGRA (same format as PaintTileRaw).
// x, y specify the top-left corner of the source rectangle in twips.
func (d *Document) PaintWindow(windowID uint32, buf []byte, x, y, pxW, pxH int) error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	if err := checkPaintBuf("PaintWindow", buf, pxW, pxH); err != nil {
		return err
	}
	return d.office.be.PaintWindow(d.h, windowID, buf, x, y, pxW, pxH)
}

// PaintWindowDPI paints a window with a DPI scale factor.
func (d *Document) PaintWindowDPI(windowID uint32, buf []byte, x, y, pxW, pxH int, dpiScale float64) error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	if err := checkPaintBuf("PaintWindowDPI", buf, pxW, pxH); err != nil {
		return err
	}
	return d.office.be.PaintWindowDPI(d.h, windowID, buf, x, y, pxW, pxH, dpiScale)
}

// PaintWindowForView paints a window for a specific view ID.
func (d *Document) PaintWindowForView(windowID uint32, view ViewID, buf []byte, x, y, pxW, pxH int, dpiScale float64) error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	if err := checkPaintBuf("PaintWindowForView", buf, pxW, pxH); err != nil {
		return err
	}
	return d.office.be.PaintWindowForView(d.h, windowID, int(view), buf, x, y, pxW, pxH, dpiScale)
}
