// Package lokc is the thin cgo layer beneath the public lok package.
//
// It wraps libc dlopen/dlsym so callers can load the LibreOfficeKit
// runtime at process start. The package is internal; the public,
// idiomatic API lives under the lok package (added in Phase 2).
package lokc
