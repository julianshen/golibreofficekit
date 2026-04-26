//go:build linux || darwin

package lokc

/*
#cgo CFLAGS: -I${SRCDIR}/../../third_party/lok -DLOK_USE_UNSTABLE_API
#include <stdlib.h>
#include "LibreOfficeKit/LibreOfficeKit.h"

static int loke_office_run_macro(LibreOfficeKit *p, const char *url) {
    if (p == NULL || p->pClass == NULL || p->pClass->runMacro == NULL) return -1;
    return p->pClass->runMacro(p, url);
}

static int loke_office_sign_document(LibreOfficeKit *p, const char *url,
                                     const unsigned char *cert, int cert_len,
                                     const unsigned char *key, int key_len) {
    if (p == NULL || p->pClass == NULL || p->pClass->signDocument == NULL) return -1;
    return p->pClass->signDocument(p, url, cert, cert_len, key, key_len) ? 1 : 0;
}

static int loke_doc_insert_certificate(LibreOfficeKitDocument *d,
                                       const unsigned char *cert, int cert_len,
                                       const unsigned char *key, int key_len) {
    if (d == NULL || d->pClass == NULL || d->pClass->insertCertificate == NULL) return -1;
    return d->pClass->insertCertificate(d, cert, cert_len, key, key_len) ? 1 : 0;
}

static int loke_doc_add_certificate(LibreOfficeKitDocument *d,
                                    const unsigned char *cert, int cert_len) {
    if (d == NULL || d->pClass == NULL || d->pClass->addCertificate == NULL) return -1;
    return d->pClass->addCertificate(d, cert, cert_len) ? 1 : 0;
}

// loke_doc_get_signature_state returns the LOK state value on success,
// or -1 when the vtable slot is missing. LOK's own values are >= 0
// (NEITHER=0, OK=1, etc.), so -1 is unambiguous.
static int loke_doc_get_signature_state(LibreOfficeKitDocument *d) {
    if (d == NULL || d->pClass == NULL || d->pClass->getSignatureState == NULL) return -1;
    return d->pClass->getSignatureState(d);
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

// ErrNilOffice is returned by office wrappers when the supplied
// OfficeHandle is the zero value.
var ErrNilOffice = errors.New("lokc: office handle is invalid")

// ErrMacroFailed is returned by OfficeRunMacro when LOK's runMacro
// returns non-zero. lok.mapLokErr translates this to lok.ErrMacroFailed.
var ErrMacroFailed = errors.New("lokc: runMacro returned non-zero")

// ErrSignFailed is returned by signing wrappers when LOK reports
// failure. lok.mapLokErr translates this to lok.ErrSignFailed.
var ErrSignFailed = errors.New("lokc: sign operation returned false")

// OfficeRunMacro calls pClass->runMacro. Returns ErrUnsupported if the
// vtable slot is missing, ErrMacroFailed when LOK reports non-zero.
func OfficeRunMacro(h OfficeHandle, url string) error {
	if !h.IsValid() {
		return ErrNilOffice
	}
	cURL := C.CString(url)
	defer C.free(unsafe.Pointer(cURL))
	rc := C.loke_office_run_macro(h.p, cURL)
	if rc == -1 {
		return ErrUnsupported
	}
	if rc != 0 {
		return ErrMacroFailed
	}
	return nil
}

// OfficeSignDocument calls pClass->signDocument. cert and key are
// pinned for the synchronous call; LOK does not retain the pointer.
func OfficeSignDocument(h OfficeHandle, url string, cert, key []byte) error {
	if !h.IsValid() {
		return ErrNilOffice
	}
	cURL := C.CString(url)
	defer C.free(unsafe.Pointer(cURL))
	certPtr, certLen := bytesPtrLen(cert)
	keyPtr, keyLen := bytesPtrLen(key)
	rc := C.loke_office_sign_document(h.p, cURL, certPtr, certLen, keyPtr, keyLen)
	if rc == -1 {
		return ErrUnsupported
	}
	if rc == 0 {
		return ErrSignFailed
	}
	return nil
}

// DocumentInsertCertificate calls pClass->insertCertificate.
func DocumentInsertCertificate(d DocumentHandle, cert, key []byte) error {
	if !d.IsValid() {
		return ErrNilDocument
	}
	certPtr, certLen := bytesPtrLen(cert)
	keyPtr, keyLen := bytesPtrLen(key)
	rc := C.loke_doc_insert_certificate(d.p, certPtr, certLen, keyPtr, keyLen)
	if rc == -1 {
		return ErrUnsupported
	}
	if rc == 0 {
		return ErrSignFailed
	}
	return nil
}

// DocumentAddCertificate calls pClass->addCertificate.
func DocumentAddCertificate(d DocumentHandle, cert []byte) error {
	if !d.IsValid() {
		return ErrNilDocument
	}
	certPtr, certLen := bytesPtrLen(cert)
	rc := C.loke_doc_add_certificate(d.p, certPtr, certLen)
	if rc == -1 {
		return ErrUnsupported
	}
	if rc == 0 {
		return ErrSignFailed
	}
	return nil
}

// DocumentGetSignatureState calls pClass->getSignatureState. Returns
// (0, ErrUnsupported) when the vtable slot is missing.
func DocumentGetSignatureState(d DocumentHandle) (int, error) {
	if !d.IsValid() {
		return 0, ErrNilDocument
	}
	rc := C.loke_doc_get_signature_state(d.p)
	if rc == -1 {
		return 0, ErrUnsupported
	}
	return int(rc), nil
}

// bytesPtrLen returns a pointer to the first byte of b and its length
// as C types, or (nil, 0) for an empty/nil slice. Empty signature is
// safe to pass to LOK functions that take const unsigned char* + size.
func bytesPtrLen(b []byte) (*C.uchar, C.int) {
	if len(b) == 0 {
		return nil, 0
	}
	return (*C.uchar)(unsafe.Pointer(&b[0])), C.int(len(b))
}
