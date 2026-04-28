//go:build linux || darwin

package lok

// ViewID is a LibreOfficeKit view identifier. LOK uses `int` —
// ViewID exists for self-documenting call sites.
type ViewID int

// guard locks the Office mutex and verifies both the document and
// its parent Office are still open. Callers defer the returned
// unlock func. Factors the Lock/defer-Unlock/closed-check pattern
// used by every view method.
//
// Checking office.closed too matters: Office.Close destroys the LOK
// backend state; view methods invoked on a still-alive Document
// after the Office went away would crash or read garbage. ErrClosed
// is the right response either way.
func (d *Document) guard() (unlock func(), err error) {
	d.office.mu.Lock()
	if d.closed || d.office.closed {
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

// DestroyView removes the view. LOK returns void, so the only
// observable failure modes are ErrClosed and ErrUnsupported (the
// vtable slot is NULL on a stripped LO build).
func (d *Document) DestroyView(id ViewID) error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	return d.office.be.DocumentDestroyView(d.h, int(id))
}

// SetView activates the view. LOK returns void; caller should
// confirm via View() if the ID is trusted. Returns ErrUnsupported
// when the LOK build does not expose setView (vtable slot NULL).
func (d *Document) SetView(id ViewID) error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	return d.office.be.DocumentSetView(d.h, int(id))
}

// View returns the currently-active view ID. A -1 return from the
// backend (no active view, missing vtable entry, or vtable failure)
// surfaces as *LOKError so callers don't get a silent zero/-1
// leaking through.
func (d *Document) View() (ViewID, error) {
	unlock, err := d.guard()
	if err != nil {
		return 0, err
	}
	defer unlock()
	id := d.office.be.DocumentGetView(d.h)
	if id < 0 {
		return 0, &LOKError{Op: "View", Detail: "LOK returned -1"}
	}
	return ViewID(id), nil
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
// Returns ErrUnsupported when the LOK build does not expose
// setViewLanguage (vtable slot NULL).
func (d *Document) SetViewLanguage(id ViewID, lang string) error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	return d.office.be.DocumentSetViewLanguage(d.h, int(id), lang)
}

// SetViewReadOnly toggles the read-only state of a specific view.
// Returns ErrUnsupported when the LOK build does not expose
// setViewReadOnly (vtable slot NULL).
func (d *Document) SetViewReadOnly(id ViewID, readOnly bool) error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	return d.office.be.DocumentSetViewReadOnly(d.h, int(id), readOnly)
}

// SetAccessibilityState turns the per-view accessibility pipeline
// (a11y tree generation, focus reporting) on or off. Returns
// ErrUnsupported when the LOK build does not expose
// setAccessibilityState (vtable slot NULL).
func (d *Document) SetAccessibilityState(id ViewID, enabled bool) error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	return d.office.be.DocumentSetAccessibilityState(d.h, int(id), enabled)
}

// SetViewTimezone sets the IANA tz name (e.g. "Europe/Berlin") for
// the given view. Empty string falls back to LO's default. Returns
// ErrUnsupported when the LOK build does not expose setViewTimezone
// (vtable slot NULL).
func (d *Document) SetViewTimezone(id ViewID, tz string) error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	return d.office.be.DocumentSetViewTimezone(d.h, int(id), tz)
}
