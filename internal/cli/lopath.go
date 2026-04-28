// Package cli holds tiny helpers shared between the binaries under
// cmd/. None of these touch LibreOffice — they are CLI plumbing
// (path resolution, error exits) that doesn't belong in the public
// lok API.
package cli

import (
	"errors"
	"fmt"
	"os"
)

// DefaultLOPathCandidates is the auto-detect list when neither
// -lo-path nor $LOK_PATH is set. Order matters — the first existing
// directory wins. Mirrors the LibreOffice install paths documented
// in CLAUDE.md.
var DefaultLOPathCandidates = []string{
	"/usr/lib/libreoffice/program",                      // Debian/Ubuntu (apt)
	"/usr/lib64/libreoffice/program",                    // Fedora/RHEL
	"/Applications/LibreOffice.app/Contents/Frameworks", // macOS .app bundle
}

// ResolveLOPath returns explicit if non-empty (verifying it is a
// directory), otherwise the first existing directory in candidates.
// Returns an error if explicit is set but isn't a directory or
// doesn't exist, or if no candidate exists.
func ResolveLOPath(explicit string, candidates []string) (string, error) {
	if explicit != "" {
		st, err := os.Stat(explicit)
		if err != nil {
			return "", fmt.Errorf("lo-path %q: %w", explicit, err)
		}
		if !st.IsDir() {
			return "", fmt.Errorf("lo-path %q is not a directory", explicit)
		}
		return explicit, nil
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			return c, nil
		}
	}
	return "", errors.New("LibreOffice install not found; pass -lo-path or set $LOK_PATH")
}
