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
