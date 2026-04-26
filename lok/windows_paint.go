//go:build linux || darwin

package lok

// PaintWindow paints a window into the provided buffer. The buffer must
// have length 4*pxW*pxH; bytes are premultiplied BGRA, matching
// PaintTileRaw. x, y, pxW, pxH are interpreted in the window's own
// coordinate space (LO renders dialog/floating windows in pixels);
// see LibreOfficeKit.h for paintWindow semantics.
func (d *Document) PaintWindow(windowID uint32, buf []byte, x, y, pxW, pxH int) error {
	if err := checkPaintBuf("PaintWindow", buf, pxW, pxH); err != nil {
		return err
	}
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	return d.office.be.PaintWindow(d.h, windowID, buf, x, y, pxW, pxH)
}

// PaintWindowDPI paints a window with a DPI scale factor.
func (d *Document) PaintWindowDPI(windowID uint32, buf []byte, x, y, pxW, pxH int, dpiScale float64) error {
	if err := checkPaintBuf("PaintWindowDPI", buf, pxW, pxH); err != nil {
		return err
	}
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	return d.office.be.PaintWindowDPI(d.h, windowID, buf, x, y, pxW, pxH, dpiScale)
}

// PaintWindowForView paints a window for a specific view ID.
func (d *Document) PaintWindowForView(windowID uint32, view ViewID, buf []byte, x, y, pxW, pxH int, dpiScale float64) error {
	if err := checkPaintBuf("PaintWindowForView", buf, pxW, pxH); err != nil {
		return err
	}
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	return d.office.be.PaintWindowForView(d.h, windowID, int(view), buf, x, y, pxW, pxH, dpiScale)
}
