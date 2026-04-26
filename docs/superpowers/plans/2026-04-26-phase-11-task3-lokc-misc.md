# Phase 11 — Task 3: C shims + Go wrappers for gap-fill functions (lokc)

**Parent plan:** `docs/superpowers/plans/2026-04-26-phase-11-advanced.md`

---

## Files created

- `internal/lokc/misc.go`
- `internal/lokc/misc_test.go`

---

## C API reference

From `LibreOfficeKit.h`:

```c
// Office-level
char* (*getFilterTypes)(LibreOfficeKit* pThis);

// Document-level
bool (*paste)(LibreOfficeKitDocument* pThis, const char* pMimeType,
              const char* pData, size_t nSize);
void (*selectPart)(LibreOfficeKitDocument* pThis, int nPart, int nSelect);
void (*moveSelectedParts)(LibreOfficeKitDocument* pThis, int nPosition, bool bDuplicate);
unsigned char* (*renderFont)(LibreOfficeKitDocument* pThis, const char* pFontName,
                              const char* pChar, int* pFontWidth, int* pFontHeight);
```

---

## Step 1: Create `internal/lokc/misc.go`

```go
//go:build linux || darwin

package lokc

/*
#cgo CFLAGS: -I${SRCDIR}/../../third_party/lok -DLOK_USE_UNSTABLE_API
#include <stdlib.h>
#include "LibreOfficeKit/LibreOfficeKit.h"

static char* go_office_getFilterTypes(LibreOfficeKit *p) {
    if (p == NULL || p->pClass == NULL || p->pClass->getFilterTypes == NULL) return NULL;
    return p->pClass->getFilterTypes(p);
}

static int go_doc_paste(LibreOfficeKitDocument *d, const char *mime,
                         const char *data, int size) {
    if (d == NULL || d->pClass == NULL || d->pClass->paste == NULL) return 0;
    return d->pClass->paste(d, mime, data, (size_t)size) ? 1 : 0;
}

static void go_doc_selectPart(LibreOfficeKitDocument *d, int part, int sel) {
    if (d == NULL || d->pClass == NULL || d->pClass->selectPart == NULL) return;
    d->pClass->selectPart(d, part, sel);
}

static void go_doc_moveSelectedParts(LibreOfficeKitDocument *d, int pos, int dup) {
    if (d == NULL || d->pClass == NULL || d->pClass->moveSelectedParts == NULL) return;
    d->pClass->moveSelectedParts(d, pos, (bool)dup);
}

static unsigned char* go_doc_renderFont(LibreOfficeKitDocument *d, const char *fontName,
                                         const char *ch, int *outW, int *outH) {
    if (d == NULL || d->pClass == NULL || d->pClass->renderFont == NULL) return NULL;
    return d->pClass->renderFont(d, fontName, ch, outW, outH);
}
*/
import "C"

import (
	"unsafe"
)

var ErrPasteFailed = errors.New("lokc: paste returned false")

func OfficeGetFilterTypes(h OfficeHandle) (string, error) {
	if !h.IsValid() {
		return "", ErrNilOffice
	}
	s := C.go_office_getFilterTypes(h.p)
	if s == nil {
		return "", ErrUnsupported
	}
	return copyAndFree(s), nil
}

func DocumentPaste(d DocumentHandle, mimeType string, data []byte) error {
	if !d.IsValid() {
		return ErrNilDocument
	}
	cMime := C.CString(mimeType)
	defer C.free(unsafe.Pointer(cMime))
	var dataPtr *C.char
	var dataLen C.int
	if len(data) > 0 {
		dataPtr = (*C.char)(unsafe.Pointer(&data[0]))
		dataLen = C.int(len(data))
	}
	rc := C.go_doc_paste(d.p, cMime, dataPtr, dataLen)
	if rc == 0 {
		return ErrUnsupported
	}
	return nil
}

func DocumentSelectPart(d DocumentHandle, part int, select_ bool) error {
	if !d.IsValid() {
		return ErrNilDocument
	}
	sel := 0
	if select_ {
		sel = 1
	}
	C.go_doc_selectPart(d.p, C.int(part), C.int(sel))
	return nil
}

func DocumentMoveSelectedParts(d DocumentHandle, position int, duplicate bool) error {
	if !d.IsValid() {
		return ErrNilDocument
	}
	dup := 0
	if duplicate {
		dup = 1
	}
	C.go_doc_moveSelectedParts(d.p, C.int(position), C.int(dup))
	return nil
}

func DocumentRenderFont(d DocumentHandle, fontName, char string) ([]byte, int, int, error) {
	if !d.IsValid() {
		return nil, 0, 0, ErrNilDocument
	}
	cFont := C.CString(fontName)
	defer C.free(unsafe.Pointer(cFont))
	cChar := C.CString(char)
	defer C.free(unsafe.Pointer(cChar))
	var w, h C.int
	ptr := C.go_doc_renderFont(d.p, cFont, cChar, &w, &h)
	if ptr == nil {
		return nil, 0, 0, ErrUnsupported
	}
	size := 4 * int(w) * int(h)
	buf := C.GoBytes(unsafe.Pointer(ptr), C.int(size))
	C.free(unsafe.Pointer(ptr))
	return buf, int(w), int(h), nil
}
```

- [ ] Create file and verify `go build ./internal/lokc`

## Step 2: Create `internal/lokc/misc_test.go`

```go
//go:build linux || darwin

package lokc

import "testing"

func TestOfficeGetFilterTypes_ZeroHandle(t *testing.T) {
	if _, err := OfficeGetFilterTypes(OfficeHandle{}); err != ErrNilOffice {
		t.Errorf("zero handle: err=%v, want ErrNilOffice", err)
	}
}

func TestDocumentPaste_ZeroHandle(t *testing.T) {
	if err := DocumentPaste(DocumentHandle{}, "text/plain", nil); err != ErrNilDocument {
		t.Errorf("zero handle: err=%v, want ErrNilDocument", err)
	}
	if err := DocumentPaste(newFakeDoc(t), "text/plain", nil); err != ErrUnsupported {
		t.Errorf("nil pClass: err=%v, want ErrUnsupported", err)
	}
}

func TestDocumentSelectPart_ZeroHandle(t *testing.T) {
	if err := DocumentSelectPart(DocumentHandle{}, 0, true); err != ErrNilDocument {
		t.Errorf("zero handle: err=%v, want ErrNilDocument", err)
	}
}

func TestDocumentMoveSelectedParts_ZeroHandle(t *testing.T) {
	if err := DocumentMoveSelectedParts(DocumentHandle{}, 0, false); err != ErrNilDocument {
		t.Errorf("zero handle: err=%v, want ErrNilDocument", err)
	}
}

func TestDocumentRenderFont_ZeroHandle(t *testing.T) {
	if _, _, _, err := DocumentRenderFont(DocumentHandle{}, "Arial", "A"); err != ErrNilDocument {
		t.Errorf("zero handle: err=%v, want ErrNilDocument", err)
	}
	if _, _, _, err := DocumentRenderFont(newFakeDoc(t), "Arial", "A"); err != ErrUnsupported {
		t.Errorf("nil pClass: err=%v, want ErrUnsupported", err)
	}
}
```

- [ ] Create file and verify `go test ./internal/lokc -run 'TestOfficeGetFilter|TestDocument.*Paste|TestDocumentSelect|TestDocumentMove|TestDocumentRenderFont' -v`

## Step 3: Full build + test

```bash
go build ./...
go test ./internal/lokc -race -count=1
```

- [ ] Builds and tests pass
