//go:build linux || darwin

package lok

import (
	"fmt"
	"sync"
)

// DocumentType mirrors LOK_DOCTYPE_* from LibreOfficeKitEnums.h.
type DocumentType int

const (
	TypeText         DocumentType = iota // LOK_DOCTYPE_TEXT
	TypeSpreadsheet                      // LOK_DOCTYPE_SPREADSHEET
	TypePresentation                     // LOK_DOCTYPE_PRESENTATION
	TypeDrawing                          // LOK_DOCTYPE_DRAWING
	TypeOther                            // LOK_DOCTYPE_OTHER
)

// String gives a human label for the type; unknown integers fall
// back to "DocumentType(n)".
func (t DocumentType) String() string {
	switch t {
	case TypeText:
		return "Text"
	case TypeSpreadsheet:
		return "Spreadsheet"
	case TypePresentation:
		return "Presentation"
	case TypeDrawing:
		return "Drawing"
	case TypeOther:
		return "Other"
	}
	return fmt.Sprintf("DocumentType(%d)", int(t))
}

// Document is a single loaded document. A process may hold many at
// once, but every method serialises on the parent Office's mutex
// because LOK is not thread-safe.
type Document struct {
	office    *Office
	h         documentHandle
	origURL   string // cached for Save()
	tempPath  string // non-empty when created by LoadFromReader
	closeOnce sync.Once
	closed    bool
}

// LoadOption configures Load / LoadFromReader.
type LoadOption func(*loadOptions)

type loadOptions struct {
	password         string
	readOnly         bool
	lang             string
	macroSecurity    MacroSecurity
	macroSecuritySet bool
	batch            bool
	repair           bool
	filterOpts       string // verbatim pass-through; caller-formatted
}

// WithPassword attaches a password for a password-protected document.
// Wired through Office.SetDocumentPassword before the load call.
func WithPassword(pw string) LoadOption {
	return func(o *loadOptions) { o.password = pw }
}

// WithReadOnly opens the document read-only.
func WithReadOnly() LoadOption {
	return func(o *loadOptions) { o.readOnly = true }
}

// WithLanguage sets the document language tag (e.g. "en-US"). The
// value must not contain commas or '=' — those would corrupt the
// LOK options string; the implementation rejects such values at
// Load time with ErrInvalidOption.
func WithLanguage(lang string) LoadOption {
	return func(o *loadOptions) { o.lang = lang }
}

// MacroSecurity levels mirror LO's macro-security UI.
type MacroSecurity int

const (
	MacroSecurityLowest  MacroSecurity = 0
	MacroSecurityMedium  MacroSecurity = 1
	MacroSecurityHigh    MacroSecurity = 2 // LO default
	MacroSecurityHighest MacroSecurity = 3
)

// WithMacroSecurity sets the macro-security level for the load.
func WithMacroSecurity(level MacroSecurity) LoadOption {
	return func(o *loadOptions) { o.macroSecurity = level; o.macroSecuritySet = true }
}

// WithBatchMode opens in headless/non-interactive mode: LO will not
// prompt for anything, failing loads that would otherwise block on
// user input.
func WithBatchMode() LoadOption {
	return func(o *loadOptions) { o.batch = true }
}

// WithRepair asks LO to attempt to repair a corrupt/truncated
// document during load.
func WithRepair() LoadOption {
	return func(o *loadOptions) { o.repair = true }
}

// WithFilterOptions passes raw filter options through to
// documentLoadWithOptions VERBATIM — no escaping, no validation.
// The string must already be LOK-formatted (comma-separated
// key=value pairs). Prefer the typed With* helpers above.
func WithFilterOptions(opts string) LoadOption {
	return func(o *loadOptions) { o.filterOpts = opts }
}

func buildLoadOptions(opts []LoadOption) loadOptions {
	var lo loadOptions
	for _, fn := range opts {
		fn(&lo)
	}
	return lo
}

// Type returns the document's LOK type, or TypeOther on a closed doc.
func (d *Document) Type() DocumentType {
	d.office.mu.Lock()
	defer d.office.mu.Unlock()
	if d.closed {
		return TypeOther
	}
	return DocumentType(d.office.be.DocumentGetType(d.h))
}
