# Phase 11 — Task 4: Extend backend interface + fakeBackend + realBackend

**Parent plan:** `docs/superpowers/plans/2026-04-26-phase-11-advanced.md`

**Prerequisites:** Tasks 2 and 3 completed (lokc wrappers exist).

---

## Files modified

- `lok/backend.go`
- `lok/real_backend.go`
- `lok/office_test.go`

---

## Step 1: Add methods to `backend` interface

In `lok/backend.go`, append to the `backend` interface (before the closing `}`):

```go
// Advanced operations (Phase 11).
OfficeRunMacro(h officeHandle, url string) error
OfficeSignDocument(h officeHandle, url string, cert, key []byte) error
OfficeGetFilterTypes(h officeHandle) (string, error)

DocumentInsertCertificate(d documentHandle, cert, key []byte) error
DocumentAddCertificate(d documentHandle, cert []byte) error
DocumentGetSignatureState(d documentHandle) (int, error)

DocumentPaste(d documentHandle, mimeType string, data []byte) error
DocumentSelectPart(d documentHandle, part int, select_ bool) error
DocumentMoveSelectedParts(d documentHandle, position int, duplicate bool) error
DocumentRenderFont(d documentHandle, fontName, char string) (buf []byte, w, h int, err error)
```

- [ ] Add interface methods; verify `go build ./lok` fails (realBackend missing methods)

## Step 2: Add realBackend forwarders

In `lok/real_backend.go`, before `var _ backend = realBackend{}`:

```go
// --- Advanced operations (Phase 11) ---

func (realBackend) OfficeRunMacro(h officeHandle, url string) error {
	return mapLokErr(lokc.OfficeRunMacro(must(h).h, url))
}

func (realBackend) OfficeSignDocument(h officeHandle, url string, cert, key []byte) error {
	return mapLokErr(lokc.OfficeSignDocument(must(h).h, url, cert, key))
}

func (realBackend) OfficeGetFilterTypes(h officeHandle) (string, error) {
	v, err := lokc.OfficeGetFilterTypes(must(h).h)
	return v, mapLokErr(err)
}

func (realBackend) DocumentInsertCertificate(d documentHandle, cert, key []byte) error {
	return mapLokErr(lokc.DocumentInsertCertificate(mustDoc(d).d, cert, key))
}

func (realBackend) DocumentAddCertificate(d documentHandle, cert []byte) error {
	return mapLokErr(lokc.DocumentAddCertificate(mustDoc(d).d, cert))
}

func (realBackend) DocumentGetSignatureState(d documentHandle) (int, error) {
	v, err := lokc.DocumentGetSignatureState(mustDoc(d).d)
	return v, mapLokErr(err)
}

func (realBackend) DocumentPaste(d documentHandle, mimeType string, data []byte) error {
	return mapLokErr(lokc.DocumentPaste(mustDoc(d).d, mimeType, data))
}

func (realBackend) DocumentSelectPart(d documentHandle, part int, select_ bool) error {
	return mapLokErr(lokc.DocumentSelectPart(mustDoc(d).d, part, select_))
}

func (realBackend) DocumentMoveSelectedParts(d documentHandle, position int, duplicate bool) error {
	return mapLokErr(lokc.DocumentMoveSelectedParts(mustDoc(d).d, position, duplicate))
}

func (realBackend) DocumentRenderFont(d documentHandle, fontName, char string) ([]byte, int, int, error) {
	buf, w, h, err := lokc.DocumentRenderFont(mustDoc(d).d, fontName, char)
	return buf, w, h, mapLokErr(err)
}
```

Also add the `lokc` import for `ErrNilOffice` and `ErrMacroFailed` / `ErrSignFailed`
translations to `mapLokErr` — extend `mapLokErr` to handle the new lokc sentinels:

```go
func mapLokErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, lokc.ErrUnsupported) {
		return ErrUnsupported
	}
	if errors.Is(err, lokc.ErrNilOffice) {
		return ErrClosed
	}
	if errors.Is(err, lokc.ErrMacroFailed) {
		return ErrMacroFailed
	}
	if errors.Is(err, lokc.ErrSignFailed) {
		return ErrSignFailed
	}
	return err
}
```

- [ ] Add forwarders; verify `go build ./...`

## Step 3: Add fakeBackend stubs

In `lok/office_test.go`, add fields to `fakeBackend`:

```go
// Phase 11: advanced + misc.
lastMacroURL         string
macroErr             error
lastSignURL          string
lastSignCert         []byte
lastSignKey          []byte
signErr              error
filterTypesResult    string
filterTypesErr       error
lastInsertCert       []byte
lastInsertKey        []byte
insertCertErr        error
lastAddCert          []byte
addCertErr           error
signatureStateResult int
signatureStateErr    error
lastPasteMime        string
lastPasteData        []byte
pasteErr             error
lastSelectPart       int
lastSelectSelect     bool
lastMovePos          int
lastMoveDup          bool
lastRenderFontName   string
lastRenderFontChar   string
renderFontBuf        []byte
renderFontW          int
renderFontH          int
renderFontErr        error
```

Add method implementations (all record calls and return configurable errors):

```go
func (f *fakeBackend) OfficeRunMacro(_ officeHandle, url string) error {
	f.lastMacroURL = url
	return f.macroErr
}

func (f *fakeBackend) OfficeSignDocument(_ officeHandle, url string, cert, key []byte) error {
	f.lastSignURL = url
	f.lastSignCert = cert
	f.lastSignKey = key
	return f.signErr
}

func (f *fakeBackend) OfficeGetFilterTypes(officeHandle) (string, error) {
	return f.filterTypesResult, f.filterTypesErr
}

func (f *fakeBackend) DocumentInsertCertificate(_ documentHandle, cert, key []byte) error {
	f.lastInsertCert = cert
	f.lastInsertKey = key
	return f.insertCertErr
}

func (f *fakeBackend) DocumentAddCertificate(_ documentHandle, cert []byte) error {
	f.lastAddCert = cert
	return f.addCertErr
}

func (f *fakeBackend) DocumentGetSignatureState(documentHandle) (int, error) {
	return f.signatureStateResult, f.signatureStateErr
}

func (f *fakeBackend) DocumentPaste(_ documentHandle, mime string, data []byte) error {
	f.lastPasteMime = mime
	f.lastPasteData = data
	return f.pasteErr
}

func (f *fakeBackend) DocumentSelectPart(_ documentHandle, part int, sel bool) error {
	f.lastSelectPart = part
	f.lastSelectSelect = sel
	return nil
}

func (f *fakeBackend) DocumentMoveSelectedParts(_ documentHandle, pos int, dup bool) error {
	f.lastMovePos = pos
	f.lastMoveDup = dup
	return nil
}

func (f *fakeBackend) DocumentRenderFont(_ documentHandle, fontName, char string) ([]byte, int, int, error) {
	f.lastRenderFontName = fontName
	f.lastRenderFontChar = char
	return f.renderFontBuf, f.renderFontW, f.renderFontH, f.renderFontErr
}
```

- [ ] Add stubs; verify `go test ./lok -count=1`

## Step 4: Full build + test

```bash
go build ./...
go test ./lok -race -count=1
```

- [ ] Builds and all existing tests pass
