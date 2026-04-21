package lok

import "encoding/json"

// VersionInfo is LibreOffice's version payload.
type VersionInfo struct {
	ProductName    string `json:"ProductName"`
	ProductVersion string `json:"ProductVersion"`
	BuildID        string `json:"BuildId"`
}

// VersionInfo returns the parsed version payload. Returns ErrClosed
// if the Office has been closed.
func (o *Office) VersionInfo() (VersionInfo, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return VersionInfo{}, ErrClosed
	}
	raw := o.be.OfficeGetVersionInfo(o.h)
	if raw == "" {
		return VersionInfo{}, &LOKError{Op: "VersionInfo", Detail: "empty response"}
	}
	var vi VersionInfo
	if err := json.Unmarshal([]byte(raw), &vi); err != nil {
		return VersionInfo{}, &LOKError{Op: "VersionInfo", Detail: err.Error()}
	}
	return vi, nil
}
