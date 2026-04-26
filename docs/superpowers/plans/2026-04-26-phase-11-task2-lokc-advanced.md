# Phase 11 — Task 2: C shims + Go wrappers for advanced functions (lokc)

**Parent plan:** `docs/superpowers/plans/2026-04-26-phase-11-advanced.md`

---

## Files created

- `internal/lokc/advanced.go`
- `internal/lokc/advanced_test.go`

---

## C API reference

From `LibreOfficeKit.h`:

```c
// Office-level
int  (*runMacro)(LibreOfficeKit *pThis, const char* pURL);
bool (*signDocument)(LibreOfficeKit* pThis, const char* pUrl,
                     const unsigned char* pCertificateBinary, const int nCertificateBinarySize,
                     const unsigned char* pPrivateKeyBinary, const int nPrivateKeyBinarySize);

// Document-level
bool (*insertCertificate)(LibreOfficeKitDocument* pThis,
                          const unsigned char* pCertificateBinary, const int nCertificateBinarySize,
                          const unsigned char* pPrivateKeyBinary, const int nPrivateKeyBinarySize);
bool (*addCertificate)(LibreOfficeKitDocument* pThis,
                       const unsigned char* pCertificateBinary, const int nCertificateBinarySize);
int  (*getSignatureState)(LibreOfficeKitDocument* pThis);
```

---

## Step 1: Create `internal/lokc/advanced.go`

```go
//go:build linux || darwin

package lokc

/*
#cgo CFLAGS: -I${SRCDIR}/../../third_party/lok -DLOK_USE_UNSTABLE_API
#include <stdlib.h>
#include "LibreOfficeKit/LibreOfficeKit.h"

static int go_office_runMacro(LibreOfficeKit *p, const char *url) {
    if (p == NULL || p->pClass == NULL || p->pClass->runMacro == NULL) return -1;
    return p->pClass->runMacro(p, url);
}

static int go_office_signDocument(LibreOfficeKit *p, const char *url,
                                   const unsigned char *cert, int certLen,
                                   const unsigned char *key, int keyLen) {
    if (p == NULL || p->pClass == NULL || p->pClass->signDocument == NULL) return 0;
    return p->pClass->signDocument(p, url, cert, certLen, key, keyLen) ? 1 : 0;
}

static int go_doc_insertCertificate(LibreOfficeKitDocument *d,
                                     const unsigned char *cert, int certLen,
                                     const unsigned char *key, int keyLen) {
    if (d == NULL || d->pClass == NULL || d->pClass->insertCertificate == NULL) return 0;
    return d->pClass->insertCertificate(d, cert, certLen, key, keyLen) ? 1 : 0;
}

static int go_doc_addCertificate(LibreOfficeKitDocument *d,
                                  const unsigned char *cert, int certLen) {
    if (d == NULL || d->pClass == NULL || d->pClass->addCertificate == NULL) return 0;
    return d->pClass->addCertificate(d, cert, certLen) ? 1 : 0;
}

static int go_doc_getSignatureState(LibreOfficeKitDocument *d) {
    if (d == NULL || d->pClass == NULL || d->pClass->getSignatureState == NULL) return -1;
    return d->pClass->getSignatureState(d);
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

var (
	ErrNilOffice       = errors.New("lokc: office handle is invalid")
	ErrMacroFailed     = errors.New("lokc: runMacro returned non-zero")
	ErrSignFailed      = errors.New("lokc: sign operation returned false")
	ErrSignatureState  = errors.New("lokc: getSignatureState returned -1")
)

func OfficeRunMacro(h OfficeHandle, url string) error {
	if !h.IsValid() {
		return ErrNilOffice
	}
	cURL := C.CString(url)
	defer C.free(unsafe.Pointer(cURL))
	rc := C.go_office_runMacro(h.p, cURL)
	if rc == -1 {
		return ErrUnsupported
	}
	if rc != 0 {
		return ErrMacroFailed
	}
	return nil
}

func OfficeSignDocument(h OfficeHandle, url string, cert, key []byte) error {
	if !h.IsValid() {
		return ErrNilOffice
	}
	cURL := C.CString(url)
	defer C.free(unsafe.Pointer(cURL))
	var certPtr *C.uchar
	var certLen C.int
	if len(cert) > 0 {
		certPtr = (*C.uchar)(unsafe.Pointer(&cert[0]))
		certLen = C.int(len(cert))
	}
	var keyPtr *C.uchar
	var keyLen C.int
	if len(key) > 0 {
		keyPtr = (*C.uchar)(unsafe.Pointer(&key[0]))
		keyLen = C.int(len(key))
	}
	rc := C.go_office_signDocument(h.p, cURL, certPtr, certLen, keyPtr, keyLen)
	if rc == 0 {
		return ErrSignFailed
	}
	return nil
}

func DocumentInsertCertificate(d DocumentHandle, cert, key []byte) error {
	if !d.IsValid() {
		return ErrNilDocument
	}
	var certPtr *C.uchar
	var certLen C.int
	if len(cert) > 0 {
		certPtr = (*C.uchar)(unsafe.Pointer(&cert[0]))
		certLen = C.int(len(cert))
	}
	var keyPtr *C.uchar
	var keyLen C.int
	if len(key) > 0 {
		keyPtr = (*C.uchar)(unsafe.Pointer(&key[0]))
		keyLen = C.int(len(key))
	}
	rc := C.go_doc_insertCertificate(d.p, certPtr, certLen, keyPtr, keyLen)
	if rc == 0 {
		return ErrUnsupported
	}
	return nil
}

func DocumentAddCertificate(d DocumentHandle, cert []byte) error {
	if !d.IsValid() {
		return ErrNilDocument
	}
	var certPtr *C.uchar
	var certLen C.int
	if len(cert) > 0 {
		certPtr = (*C.uchar)(unsafe.Pointer(&cert[0]))
		certLen = C.int(len(cert))
	}
	rc := C.go_doc_addCertificate(d.p, certPtr, certLen)
	if rc == 0 {
		return ErrUnsupported
	}
	return nil
}

func DocumentGetSignatureState(d DocumentHandle) (int, error) {
	if !d.IsValid() {
		return -1, ErrNilDocument
	}
	rc := C.go_doc_getSignatureState(d.p)
	if rc == -1 {
		return -1, ErrUnsupported
	}
	return int(rc), nil
}
```

- [ ] Create file and verify `go build ./internal/lokc`

## Step 2: Create `internal/lokc/advanced_test.go`

Follow the established pattern: zero handle → sentinel error; calloc'd fake
(pClass == NULL) → ErrUnsupported. Use `newFakeDoc(t)` from `document_test_helper.go`.

```go
//go:build linux || darwin

package lokc

import "testing"

func TestOfficeRunMacro_ZeroHandle(t *testing.T) {
	if err := OfficeRunMacro(OfficeHandle{}, "macro:///x"); err != ErrNilOffice {
		t.Errorf("zero handle: err=%v, want ErrNilOffice", err)
	}
}

func TestOfficeSignDocument_ZeroHandle(t *testing.T) {
	if err := OfficeSignDocument(OfficeHandle{}, "url", nil, nil); err != ErrNilOffice {
		t.Errorf("zero handle: err=%v, want ErrNilOffice", err)
	}
}

func TestDocumentInsertCertificate_ZeroHandle(t *testing.T) {
	if err := DocumentInsertCertificate(DocumentHandle{}, nil, nil); err != ErrNilDocument {
		t.Errorf("zero handle: err=%v, want ErrNilDocument", err)
	}
	// nil pClass → ErrUnsupported
	if err := DocumentInsertCertificate(newFakeDoc(t), nil, nil); err != ErrUnsupported {
		t.Errorf("nil pClass: err=%v, want ErrUnsupported", err)
	}
}

func TestDocumentAddCertificate_ZeroHandle(t *testing.T) {
	if err := DocumentAddCertificate(DocumentHandle{}, nil); err != ErrNilDocument {
		t.Errorf("zero handle: err=%v, want ErrNilDocument", err)
	}
	if err := DocumentAddCertificate(newFakeDoc(t), nil); err != ErrUnsupported {
		t.Errorf("nil pClass: err=%v, want ErrUnsupported", err)
	}
}

func TestDocumentGetSignatureState_ZeroHandle(t *testing.T) {
	if _, err := DocumentGetSignatureState(DocumentHandle{}); err != ErrNilDocument {
		t.Errorf("zero handle: err=%v, want ErrNilDocument", err)
	}
	if _, err := DocumentGetSignatureState(newFakeDoc(t)); err != ErrUnsupported {
		t.Errorf("nil pClass: err=%v, want ErrUnsupported", err)
	}
}
```

- [ ] Create file and verify `go test ./internal/lokc -run 'TestOffice|TestDocument.*Certificate|TestDocumentGetSignature' -v`

## Step 3: Full build + test

```bash
go build ./...
go test ./internal/lokc -race -count=1
```

- [ ] Builds and tests pass
