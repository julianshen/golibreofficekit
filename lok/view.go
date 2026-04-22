//go:build linux || darwin

package lok

// ViewID is a LibreOfficeKit view identifier. LOK uses `int` —
// ViewID exists for self-documenting call sites.
type ViewID int

// guard locks the Office mutex and verifies the document is still
// open. Callers defer the returned unlock func. Factors the
// Lock/defer-Unlock/closed-check pattern used by every view method.
func (d *Document) guard() (unlock func(), err error) {
	d.office.mu.Lock()
	if d.closed {
		d.office.mu.Unlock()
		return func() {}, ErrClosed
	}
	return d.office.mu.Unlock, nil
}

// CreateView creates a new view on the document and returns its ID.
// Returns ErrClosed on a closed document; a *LOKError wrapping
// ErrViewCreateFailed when LOK returns -1.
func (d *Document) CreateView() (ViewID, error) {
	unlock, err := d.guard()
	if err != nil {
		return 0, err
	}
	defer unlock()
	id := d.office.be.DocumentCreateView(d.h)
	if id < 0 {
		return 0, &LOKError{Op: "CreateView", Detail: "LOK returned -1", err: ErrViewCreateFailed}
	}
	return ViewID(id), nil
}

// CreateViewWithOptions forwards a raw options string to
// pClass->createViewWithOptions. Same error contract as CreateView.
func (d *Document) CreateViewWithOptions(options string) (ViewID, error) {
	unlock, err := d.guard()
	if err != nil {
		return 0, err
	}
	defer unlock()
	id := d.office.be.DocumentCreateViewWithOptions(d.h, options)
	if id < 0 {
		return 0, &LOKError{Op: "CreateViewWithOptions", Detail: "LOK returned -1", err: ErrViewCreateFailed}
	}
	return ViewID(id), nil
}

// DestroyView removes the view. LOK returns void, so errors surface
// only from the closed-doc check.
func (d *Document) DestroyView(id ViewID) error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	d.office.be.DocumentDestroyView(d.h, int(id))
	return nil
}

// SetView activates the view. LOK returns void; caller should
// confirm via View() if the ID is trusted.
func (d *Document) SetView(id ViewID) error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	d.office.be.DocumentSetView(d.h, int(id))
	return nil
}

// View returns the currently-active view ID.
func (d *Document) View() (ViewID, error) {
	unlock, err := d.guard()
	if err != nil {
		return 0, err
	}
	defer unlock()
	return ViewID(d.office.be.DocumentGetView(d.h)), nil
}

// Views returns the IDs of all live views in document order.
// Returns (nil, nil) when zero views are live, (nil, *LOKError)
// when the backend call itself fails.
func (d *Document) Views() ([]ViewID, error) {
	unlock, err := d.guard()
	if err != nil {
		return nil, err
	}
	defer unlock()
	raw, ok := d.office.be.DocumentGetViewIds(d.h)
	if !ok {
		return nil, &LOKError{Op: "Views", Detail: "LOK getViewIds failed"}
	}
	if len(raw) == 0 {
		return nil, nil
	}
	out := make([]ViewID, len(raw))
	for i, v := range raw {
		out[i] = ViewID(v)
	}
	return out, nil
}

// SetViewLanguage sets the UI language tag for a specific view.
func (d *Document) SetViewLanguage(id ViewID, lang string) error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	d.office.be.DocumentSetViewLanguage(d.h, int(id), lang)
	return nil
}

// SetViewReadOnly toggles the read-only state of a specific view.
func (d *Document) SetViewReadOnly(id ViewID, readOnly bool) error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	d.office.be.DocumentSetViewReadOnly(d.h, int(id), readOnly)
	return nil
}

// SetAccessibilityState turns the per-view accessibility pipeline
// (a11y tree generation, focus reporting) on or off.
func (d *Document) SetAccessibilityState(id ViewID, enabled bool) error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	d.office.be.DocumentSetAccessibilityState(d.h, int(id), enabled)
	return nil
}

// SetViewTimezone sets the IANA tz name (e.g. "Europe/Berlin") for
// the given view. Empty string falls back to LO's default.
func (d *Document) SetViewTimezone(id ViewID, tz string) error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	d.office.be.DocumentSetViewTimezone(d.h, int(id), tz)
	return nil
}
