//go:build linux || darwin

package lok

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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

// Load opens a document from the filesystem. The path is converted
// to a file:// URL before being passed to LOK. Variadic options
// switch to documentLoadWithOptions when any option that needs the
// options string is present; otherwise documentLoad is used.
func (o *Office) Load(path string, opts ...LoadOption) (*Document, error) {
	if path == "" {
		return nil, ErrPathRequired
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return nil, ErrClosed
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, &LOKError{Op: "Load", Detail: err.Error(), err: err}
	}
	fileURL := (&url.URL{Scheme: "file", Path: abs}).String()

	lo := buildLoadOptions(opts)

	// Validate options BEFORE registering the password. An invalid
	// option must not leave a password callback armed for a URL that
	// this call will never actually load.
	optsStr, err := composeLoadOptions(lo)
	if err != nil {
		return nil, err
	}
	if lo.password != "" {
		o.be.OfficeSetDocumentPassword(o.h, fileURL, lo.password)
	}

	var h documentHandle
	if optsStr != "" {
		h, err = o.be.DocumentLoadWithOptions(o.h, fileURL, optsStr)
	} else {
		h, err = o.be.DocumentLoad(o.h, fileURL)
	}
	if err != nil {
		return nil, err
	}

	doc := &Document{
		office:  o,
		h:       h,
		origURL: fileURL,
	}
	return doc, nil
}

// composeLoadOptions turns the typed LoadOption values into the raw
// options string LOK accepts at documentLoadWithOptions. LOK parses
// the string as comma-separated key=value pairs. Returns ("", nil)
// when no typed option or filterOpts is set. Returns ("",
// ErrInvalidOption) if lang contains a reserved character.
func composeLoadOptions(lo loadOptions) (string, error) {
	if strings.ContainsAny(lo.lang, ",=") {
		return "", ErrInvalidOption
	}
	var parts []string
	if lo.readOnly {
		parts = append(parts, "ReadOnly=1")
	}
	if lo.lang != "" {
		parts = append(parts, "Language="+lo.lang)
	}
	if lo.macroSecuritySet {
		parts = append(parts, fmt.Sprintf("MacroSecurityLevel=%d", lo.macroSecurity))
	}
	if lo.batch {
		parts = append(parts, "Batch=1")
	}
	if lo.repair {
		parts = append(parts, "Repair=1")
	}
	if lo.filterOpts != "" {
		parts = append(parts, lo.filterOpts)
	}
	if len(parts) == 0 {
		return "", nil
	}
	return strings.Join(parts, ","), nil
}

// Close destroys the LOK document handle and cleans up any temp file
// created by LoadFromReader. Idempotent.
func (d *Document) Close() error {
	d.closeOnce.Do(func() {
		d.office.mu.Lock()
		defer d.office.mu.Unlock()
		d.office.be.DocumentDestroy(d.h)
		d.closed = true
		if d.tempPath != "" {
			_ = os.Remove(d.tempPath)
		}
	})
	return nil
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

// SaveAs writes the document to path in the given LO format (e.g.
// "odt", "pdf", "docx", "png"). filterOpts passes extra
// filter-specific tokens verbatim (e.g. "SkipImages=1"). An empty
// path returns ErrPathRequired; calls on a closed document return
// ErrClosed.
func (d *Document) SaveAs(path, format, filterOpts string) error {
	if path == "" {
		return ErrPathRequired
	}
	d.office.mu.Lock()
	defer d.office.mu.Unlock()
	if d.closed {
		return ErrClosed
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return &LOKError{Op: "SaveAs", Detail: err.Error(), err: err}
	}
	fileURL := (&url.URL{Scheme: "file", Path: abs}).String()
	if err := d.office.be.DocumentSaveAs(d.h, fileURL, format, filterOpts); err != nil {
		return &LOKError{Op: "SaveAs", Detail: err.Error(), err: err}
	}
	return nil
}

// Save re-saves the document to its original URL. LOK has no
// dedicated save() vtable entry, so Save is implemented as saveAs
// to origURL with no format/filter changes. Single critical section
// — sync.Mutex is non-reentrant, so the implementation must not
// call d.SaveAs from inside this method.
func (d *Document) Save() error {
	d.office.mu.Lock()
	defer d.office.mu.Unlock()
	if d.closed {
		return ErrClosed
	}
	if err := d.office.be.DocumentSaveAs(d.h, d.origURL, "", ""); err != nil {
		return &LOKError{Op: "Save", Detail: err.Error(), err: err}
	}
	return nil
}

// LoadFromReader streams r into a temp file under os.TempDir, then
// calls Load on that path. filter is used as the temp file's
// extension when non-empty (so LO auto-detects the format); pass
// "" to let LO guess from content. The temp file is removed in
// Document.Close.
//
// Accepts the same LoadOption values as Load.
func (o *Office) LoadFromReader(r io.Reader, filter string, opts ...LoadOption) (*Document, error) {
	suffix := ""
	if filter != "" {
		suffix = "." + filter
	}
	f, err := os.CreateTemp("", "lokdoc-*"+suffix)
	if err != nil {
		return nil, &LOKError{Op: "LoadFromReader", Detail: err.Error(), err: err}
	}
	tempPath := f.Name()
	if _, err := io.Copy(f, r); err != nil {
		_ = f.Close()
		_ = os.Remove(tempPath)
		return nil, &LOKError{Op: "LoadFromReader", Detail: err.Error(), err: err}
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tempPath)
		return nil, &LOKError{Op: "LoadFromReader", Detail: err.Error(), err: err}
	}
	doc, err := o.Load(tempPath, opts...)
	if err != nil {
		_ = os.Remove(tempPath)
		return nil, err
	}
	doc.tempPath = tempPath
	return doc, nil
}
