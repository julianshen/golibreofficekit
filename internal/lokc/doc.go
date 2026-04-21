// Package lokc is the thin cgo layer beneath the public lok package.
//
// It wraps libc dlopen/dlsym so callers can load the LibreOfficeKit
// runtime at process start. The package is internal; the idiomatic
// Go API lives in the lok package.
package lokc
