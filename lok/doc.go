// Package lok is the public, idiomatic Go binding for LibreOfficeKit.
//
// Use New to load and initialise LibreOffice; use the returned
// *Office to open documents, register callbacks, and so on. There
// may only be one live *Office per process (LibreOffice's own
// constraint); calling New a second time while the first is open
// returns ErrAlreadyInitialised.
//
// The typical install path is:
//
//   - Linux (Fedora): /usr/lib64/libreoffice/program
//   - Linux (Debian): /usr/lib/libreoffice/program
//   - macOS:          inside the LibreOffice.app bundle
//
// LOK is not free-threaded; this package serialises all LOK entry
// points with an Office-wide mutex so most callers do not need to
// think about it.
package lok
