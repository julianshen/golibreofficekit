//go:build linux || darwin

package lok

// ClipboardItem is a single per-view clipboard entry. Data is nil
// when LOK had no payload for the corresponding MimeType (GetClipboard
// preserves request order; unsupported MIME types come back as
// zero-Data entries).
type ClipboardItem struct {
	MimeType string
	Data     []byte
}

// GetClipboard reads the per-view clipboard. A nil (or empty)
// mimeTypes slice asks LOK for every MIME type it offers natively;
// a populated slice requests those specific types, returning one
// ClipboardItem per request in request order (unavailable entries
// come back with Data == nil).
func (d *Document) GetClipboard(mimeTypes []string) ([]ClipboardItem, error) {
	for _, m := range mimeTypes {
		if err := validateMime(m); err != nil {
			return nil, err
		}
	}
	unlock, err := d.guard()
	if err != nil {
		return nil, err
	}
	defer unlock()
	// Normalise empty slice to nil for the backend — both map to C
	// NULL in the real backend.
	var reqMimes []string
	if len(mimeTypes) > 0 {
		reqMimes = mimeTypes
	}
	inner, err := d.office.be.DocumentGetClipboard(d.h, reqMimes)
	if err != nil {
		return nil, err
	}
	out := make([]ClipboardItem, len(inner))
	for i, it := range inner {
		out[i] = ClipboardItem{MimeType: it.MimeType, Data: it.Data}
	}
	return out, nil
}
