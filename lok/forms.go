//go:build linux || darwin

package lok

// SendDialogEvent sends a dialog event identified by windowID.
// argsJSON is a JSON object whose structure depends on the event type.
func (d *Document) SendDialogEvent(windowID uint64, argsJSON string) error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	return d.office.be.SendDialogEvent(d.h, windowID, argsJSON)
}

// SendContentControlEvent sends an event for a content control.
func (d *Document) SendContentControlEvent(argsJSON string) error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	return d.office.be.SendContentControlEvent(d.h, argsJSON)
}

// SendFormFieldEvent sends an event for a form field.
func (d *Document) SendFormFieldEvent(argsJSON string) error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	return d.office.be.SendFormFieldEvent(d.h, argsJSON)
}
