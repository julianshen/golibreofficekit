//go:build linux || darwin

package lok

// FilterTypes returns the list of document filters LibreOffice
// supports as a JSON string. The shape is not documented in the LOK
// header; callers that need to parse it should treat it as opaque or
// validate against an empirical fixture from the target LO version.
// Returns ErrClosed when the office has been closed and
// ErrUnsupported when the LO build does not expose getFilterTypes.
func (o *Office) FilterTypes() (string, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return "", ErrClosed
	}
	return o.be.OfficeGetFilterTypes(o.h)
}
