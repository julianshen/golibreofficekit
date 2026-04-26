# Phase 11 — Advanced: macros, signing & certificates (design)

**Branch:** `feat/advanced`
**Status:** draft (ready for implementation)
**Predecessor:** Phase 10 (command values & window events, merged in PR #22)

## 1. Goals

Bind LibreOfficeKit's macro execution, document signing, certificate management,
and signature-state query APIs so Go programs can:

1. Execute LibreOffice macros identified by UNO URL.
2. Digitally sign a document with a PEM-encoded certificate and private key.
3. Insert and add X.509 certificates to a document's certificate store.
4. Query the cryptographic signature state of a loaded document.
5. Receive signature-status change events via the listener infrastructure.

This phase also adds `Office.FilterTypes` (a Phase 3 gap discovered during
Phase 11 research) and a small set of previously uncovered document-level APIs
(`paste`, `selectPart`, `moveSelectedParts`, `renderFont`) that are part of the
LOK 24.8 header but were missing from the coverage matrix.

## 2. Design deviation from original spec

The original design spec (§1, §6 Phase 11) says the advanced tier ships behind a
`lok_advanced` build tag. After analysis, this creates a conflict with the
existing `backend` interface pattern: Go interfaces require all methods to be
implemented by every concrete type, and gating methods behind a build tag would
require maintaining parallel stub files for every build configuration.

**Decision:** All Phase 11 code compiles unconditionally with `//go:build linux || darwin`
(same as every other phase). The "advanced" nature is expressed through:

- **Documentation:** godoc marks these functions as advanced, requiring specific
  LibreOffice features.
- **Testing tier:** Unit tests via `fakeBackend` run normally. Integration tests
  for signing require `LOK_TEST_CERTS` (paths to a cert and key); macro tests
  require `LOK_PATH`. Both skip gracefully when unavailable.
- **Coverage:** Unit-test coverage for advanced functions counts toward the
  ≥90% gate. Integration coverage is best-effort.

## 3. Spec corrections

The original coverage matrix placed `insertCertificate` and `addCertificate` on
`Office`. The vendored `LibreOfficeKit.h` declares both on
`LibreOfficeKitDocument` (document-level vtable). The Go API follows the header:

| Function              | Original spec | Corrected (matches header) |
|-----------------------|---------------|----------------------------|
| `insertCertificate`   | `Office`      | `Document`                 |
| `addCertificate`      | `Office`      | `Document`                 |
| `signDocument`        | `Office`      | `Office` (unchanged)       |
| `runMacro`            | `Office`      | `Office` (unchanged)       |
| `getSignatureState`   | `Document`    | `Document` (unchanged)     |

## 4. Architecture

Same four-layer pattern as previous phases.

```text
lok (public)           — Office.RunMacro, Office.SignDocument,
                          Document.InsertCertificate, Document.AddCertificate,
                          Document.SignatureState, Office.FilterTypes,
                          Document.Paste, Document.SelectPart, etc.
  └─ backend seam      — new methods on backend interface
       └─ realBackend  — forwarders calling lokc wrappers
           └─ internal/lokc  — C shims for advanced + gap-fill functions
               └─ LOK C ABI
```

Files added:

- `lok/advanced.go` — `RunMacro`, `SignDocument` on `Office`; `InsertCertificate`,
  `AddCertificate`, `SignatureState` on `Document`.
- `lok/advanced_test.go` — unit tests for advanced functions.
- `lok/filter_types.go` — `Office.FilterTypes` (Phase 3 gap fill).
- `lok/filter_types_test.go` — unit tests for FilterTypes.
- `lok/misc.go` — `Document.Paste`, `Document.SelectPart`,
  `Document.MoveSelectedParts`, `Document.RenderFont` (coverage-matrix gap fills).
- `lok/misc_test.go` — unit tests for misc document operations.
- `internal/lokc/advanced.go` — C shims + Go wrappers for `runMacro`,
  `signDocument`, `insertCertificate`, `addCertificate`, `getSignatureState`.
- `internal/lokc/advanced_test.go` — zero-handle / nil-pClass guard-rail tests.
- `internal/lokc/misc.go` — C shims + Go wrappers for `getFilterTypes`, `paste`,
  `selectPart`, `moveSelectedParts`, `renderFont`.
- `internal/lokc/misc_test.go` — guard-rail tests.

Files modified:

- `lok/backend.go` — add new interface methods.
- `lok/real_backend.go` — add forwarders.
- `lok/office_test.go` — add `fakeBackend` stubs for new methods.
- `lok/event.go` — add `EventTypeSignatureStatus` (= 40).
- `lok/errors.go` — add `ErrMacroFailed`, `ErrSignFailed` sentinels.
- `lok/integration_test.go` — add integration smoke tests (gated by env vars).

## 5. Public API

### 5.1 Office-level advanced

```go
// RunMacro executes the LibreOffice macro identified by url.
// url is a UNO URL (e.g. "macro:///Standard.Module1.Main()").
// Returns ErrMacroFailed if LOK rejects the macro.
func (o *Office) RunMacro(url string) error

// SignDocument signs the document at docURL with the given PEM-encoded
// certificate and private key. The document must not be open by this
// Office instance (LOK signs on-disk, not in-memory).
// Returns ErrSignFailed on failure.
func (o *Office) SignDocument(docURL string, certificate, privateKey []byte) error

// FilterTypes returns the list of document filters LO supports as a JSON
// string. The format is documented in LibreOfficeKit.h.
func (o *Office) FilterTypes() (string, error)
```

### 5.2 Document-level advanced

```go
// SignatureState represents the cryptographic signature state of a document.
type SignatureState int

const (
    SignatureNotSigned     SignatureState = 0 // NEITHER
    SignatureOK            SignatureState = 1 // signature valid
    SignatureNotValidated  SignatureState = 2 // present but not validated
    SignatureInvalid       SignatureState = 3 // signature invalid
    SignatureUnknown       SignatureState = 4 // unknown state
)

func (SignatureState) String() string

// SignatureState returns the document's current cryptographic signature state.
func (d *Document) SignatureState() (SignatureState, error)

// InsertCertificate inserts a certificate and private key into the document's
// certificate store. Used before signing to ensure the certificate is available.
func (d *Document) InsertCertificate(certificate, privateKey []byte) error

// AddCertificate adds a certificate (without private key) to the document's
// certificate store.
func (d *Document) AddCertificate(certificate []byte) error
```

### 5.3 Document-level misc (coverage-matrix gap fills)

```go
// Paste inserts data from the clipboard into the document.
func (d *Document) Paste(mimeType string, data []byte) error

// SelectPart selects or deselects a part (slide/sheet).
func (d *Document) SelectPart(part int, select bool) error

// MoveSelectedParts moves the selected parts to a new position.
func (d *Document) MoveSelectedParts(position int, duplicate bool) error

// RenderFont renders a glyph of the named font and returns the bitmap
// in premultiplied BGRA, along with the pixel dimensions.
func (d *Document) RenderFont(fontName, char string) (image []byte, width, height int, err error)
```

### 5.4 Event type addition

```go
// In lok/event.go, add:
EventTypeSignatureStatus EventType = 40 // LOK_CALLBACK_SIGNATURE_STATUS
```

## 6. Data flow

### 6.1 RunMacro

```text
[Go] office.RunMacro("macro:///Standard.Module1.Main()")
   │
   ▼
[lok] Office.RunMacro → guard (closed?) → backend.OfficeRunMacro
   │
   ▼
[lokc] OfficeRunMacro(h, url)
   │
   ▼
[LOK C] pClass->runMacro(pThis, url) → int (0 = success, non-zero = failure)
   │
   ▼
[lok] if non-zero → ErrMacroFailed; else nil
```

### 6.2 SignDocument

```text
[Go] office.SignDocument(url, certPEM, keyPEM)
   │
   ▼
[lok] Office.SignDocument → guard → backend.OfficeSignDocument
   │
   ▼
[lokc] OfficeSignDocument(h, url, cert, certLen, key, keyLen)
   │
   ▼
[LOK C] pClass->signDocument(pThis, url, cert, certLen, key, keyLen) → bool
   │
   ▼
[lok] if false → ErrSignFailed; else nil
```

### 6.3 SignatureState

```text
[Go] doc.SignatureState()
   │
   ▼
[lok] Document.SignatureState → guard → backend.DocumentGetSignatureState
   │
   ▼
[lokc] DocumentGetSignatureState(d)
   │
   ▼
[LOK C] pClass->getSignatureState(pThis) → int
   │
   ▼
[lok] SignatureState(int)
```

### 6.4 InsertCertificate / AddCertificate

```text
[Go] doc.InsertCertificate(cert, key)
   │
   ▼
[lokc] DocumentInsertCertificate(d, cert, certLen, key, keyLen)
   │
   ▼
[LOK C] pClass->insertCertificate(pThis, cert, certLen, key, keyLen) → bool
```

```text
[Go] doc.AddCertificate(cert)
   │
   ▼
[lokc] DocumentAddCertificate(d, cert, certLen)
   │
   ▼
[LOK C] pClass->addCertificate(pThis, cert, certLen) → bool
```

## 7. cgo safety

- `RunMacro` passes a C string (URL). No Go pointers stored in C.
- `SignDocument`, `InsertCertificate`, `AddCertificate` pass byte slices as
  `const unsigned char*` + length. The buffer is pinned for the synchronous call
  only; LOK does not retain the pointer.
- `getFilterTypes` returns a `char*` owned by the caller; wrapped with
  `copyAndFree`.
- `getSignatureState` returns an `int`; no allocation.
- `renderFont` returns a `unsigned char*` buffer owned by LOK; copied to Go with
  `C.GoBytes` and freed with `C.free`.
- No `//export` trampolines needed for Phase 11 (no new callbacks).

## 8. Error handling

New sentinels:

```go
var (
    ErrMacroFailed = errors.New("lok: macro execution failed")
    ErrSignFailed  = errors.New("lok: document signing failed")
)
```

Existing sentinels reused:

- `ErrClosed` — office or document is closed.
- `ErrUnsupported` — LOK vtable slot is NULL (older LO build).
- `*LOKError` — wraps LOK's `getError()` string when available.

`RunMacro` returns `ErrMacroFailed` when LOK's `runMacro` returns non-zero.
`SignDocument` returns `ErrSignFailed` when LOK's `signDocument` returns false.
`InsertCertificate` and `AddCertificate` return `ErrSignFailed` when LOK returns
false.
`SignatureState` returns `ErrUnsupported` when the vtable slot is NULL.

## 9. Testing

### 9.1 Unit tests

All use `fakeBackend` (no LOK required, run under plain `go test`):

- `RunMacro`: success path, `ErrClosed`, `ErrMacroFailed`, empty URL.
- `SignDocument`: success, `ErrClosed`, `ErrSignFailed`, empty URL, nil cert/key.
- `FilterTypes`: success, `ErrClosed`.
- `SignatureState`: success returns typed state, `ErrClosed`.
- `InsertCertificate` / `AddCertificate`: success, `ErrClosed`, `ErrSignFailed`.
- `Paste`: success, `ErrClosed`, empty data.
- `SelectPart` / `MoveSelectedParts`: success, `ErrClosed`.
- `RenderFont`: success, `ErrClosed`, empty font name.
- Guard-rail tests in `lokc`: zero-handle → `ErrNilDocument` / `ErrNilOffice`,
  nil-pClass → `ErrUnsupported`.

### 9.2 Integration tests (`lok/integration_test.go`)

All skip gracefully when prerequisites are missing:

- `TestIntegration_RunMacro`: requires `LOK_PATH`. Creates a test document,
  runs a simple macro, verifies no crash. May skip if LO doesn't ship macros.
- `TestIntegration_SignDocument`: requires `LOK_PATH` and `LOK_TEST_CERTS`
  (env var set to `<cert.pem>,<key.pem>` paths). Signs a document, loads it,
  checks `SignatureState()`.
- `TestIntegration_SignatureState`: requires `LOK_PATH`. Loads a document,
  checks that `SignatureState()` returns a valid state.
- `TestIntegration_FilterTypes`: requires `LOK_PATH`. Calls `FilterTypes()`,
  asserts valid JSON with known filter entries.

### 9.3 Coverage

- Unit-test coverage ≥ 90% for `lok` package (including new functions).
- `lokc` guard-rail tests cover zero-handle and nil-pClass paths.
- Integration tests are best-effort and do not count toward the gate.

## 10. Implementation order

1. Add new sentinels to `lok/errors.go`.
2. Add `EventTypeSignatureStatus` to `lok/event.go`.
3. Extend `backend` interface + `fakeBackend` stubs.
4. Add C shims + Go wrappers in `internal/lokc/advanced.go` and `lokc/misc.go`.
5. Add `realBackend` forwarders.
6. Implement public API in `lok/advanced.go`, `lok/filter_types.go`, `lok/misc.go`.
7. Write unit tests (`advanced_test.go`, `filter_types_test.go`, `misc_test.go`).
8. Add integration smoke tests.
9. Verify coverage ≥ 90%.

## 11. Out of scope / deferred

- **High-level macro helpers** (e.g. `office.RunBASIC(module, func, args)`).
  Keep binding low-level; callers construct UNO URLs.
- **Certificate parsing/validation.** Callers provide raw PEM bytes; no
  X.509 parsing inside `lok`.
- **URP (UNO Remote Protocol).** `startURP`/`stopURP` are deep integration
  points for LO Online; defer indefinitely.
- **`runLoop` / `joinThreads` / `setForkedChild`.** These are LO-internal
  process management functions not useful for typical binding consumers.
- **`extractRequest` / `setOption` (general).** `setOption` is already used
  internally for `setAuthor`; a general `SetOption(key, value)` could be added
  later if needed.
- **Accessibility APIs.** `getA11yFocusedParagraph`, `getA11yCaretPosition` —
  defer to a future phase.
- **Window text selection.** `setWindowTextSelection` — defer.
- **Text context removal.** `removeTextContext` — defer.
- **Edit mode / data area.** `getEditMode`, `getDataArea` — Calc-specific;
  defer.
- **Font orientation rendering.** `renderFontOrientation` — defer (niche).
- **Timezone per view.** `setViewTimezone` — already in the backend seam
  (added in Phase 4 area) but no public API yet; defer.
- **Allow-change-comments.** `setAllowChangeComments` — defer.

## 12. Notes on compatibility

- `runMacro` has been available since LibreOffice 6.0. The function is always
  present in LO 24.8's vtable.
- `signDocument` was added in LibreOffice 6.2. Always present in 24.8.
- `insertCertificate` and `addCertificate` are in the `LOK_USE_UNSTABLE_API`
  section. They may be absent in older builds — the C shim checks for NULL
  vtable entries and returns `ErrUnsupported`.
- `getSignatureState` is also in the unstable API section. Same NULL-vtable
  guard applies.
- `LOK_CALLBACK_SIGNATURE_STATUS` (value 40) is available since LO 7.5.
- `getFilterTypes` has been available since LibreOffice 6.0.
- `paste` is in the unstable API section.
- `selectPart` / `moveSelectedParts` are in the unstable API section.
- `renderFont` is in the unstable API section.

## 13. Coverage matrix updates

Functions added in Phase 11 (to be reflected in the coverage matrix):

| LOK function          | Phase | Go symbol                        |
|-----------------------|-------|----------------------------------|
| `runMacro`            | 11    | `Office.RunMacro`                |
| `signDocument`        | 11    | `Office.SignDocument`            |
| `getFilterTypes`      | 3→11  | `Office.FilterTypes`             |
| `insertCertificate`   | 11    | `Document.InsertCertificate`     |
| `addCertificate`      | 11    | `Document.AddCertificate`        |
| `getSignatureState`   | 11    | `Document.SignatureState`        |
| `paste`               | 11    | `Document.Paste`                 |
| `selectPart`          | 11    | `Document.SelectPart`            |
| `moveSelectedParts`   | 11    | `Document.MoveSelectedParts`     |
| `renderFont`          | 11    | `Document.RenderFont`            |

Note: `getFilterTypes` was originally assigned to Phase 3 but was not
implemented. Phase 11 fills this gap.
