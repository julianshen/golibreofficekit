//go:build linux || darwin

package lok

// PostWindowKeyEvent posts a key event to a specific window.
func (d *Document) PostWindowKeyEvent(windowID uint32, typ KeyEventType, charCode, keyCode int) error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	if err := requireInt32Key("PostWindowKeyEvent", charCode, keyCode); err != nil {
		return err
	}
	return d.office.be.PostWindowKeyEvent(d.h, windowID, int(typ), charCode, keyCode)
}

// PostWindowMouseEvent posts a mouse event to a specific window.
func (d *Document) PostWindowMouseEvent(windowID uint32, typ MouseEventType, x, y int64, count int, buttons MouseButton, mods Modifier) error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	if err := requireInt32XY("PostWindowMouseEvent", x, y); err != nil {
		return err
	}
	return d.office.be.PostWindowMouseEvent(d.h, windowID, int(typ), x, y, count, int(buttons), int(mods))
}

// PostWindowGestureEvent posts a gesture event (pan/zoom) to a window.
func (d *Document) PostWindowGestureEvent(windowID uint32, typ string, x, y, offset int64) error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	return d.office.be.PostWindowGestureEvent(d.h, windowID, typ, x, y, offset)
}

// PostWindowExtTextInputEvent posts extended text input to a window.
func (d *Document) PostWindowExtTextInputEvent(windowID uint32, typ int, text string) error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	return d.office.be.PostWindowExtTextInputEvent(d.h, windowID, typ, text)
}

// ResizeWindow changes the size of a window.
func (d *Document) ResizeWindow(windowID uint32, w, h int) error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	if w <= 0 || h <= 0 {
		return ErrInvalidOption
	}
	return d.office.be.ResizeWindow(d.h, windowID, w, h)
}
