# Phase 11 — Task 8: Integration smoke tests

**Parent plan:** `docs/superpowers/plans/2026-04-26-phase-11-advanced.md`

**Prerequisites:** Tasks 5–7 completed (public API implemented).

---

## Files modified

- `lok/integration_test.go`

---

## Step 1: Add integration tests

Append to `lok/integration_test.go` (behind `//go:build lok_integration`):

### TestIntegration_FilterTypes

```go
func TestIntegration_FilterTypes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	lokPath := os.Getenv("LOK_PATH")
	if lokPath == "" {
		t.Skip("LOK_PATH not set")
	}
	o, err := New(lokPath)
	if err != nil {
		t.Skipf("cannot create Office: %v", err)
	}
	defer o.Close()

	ft, err := o.FilterTypes()
	if err != nil {
		t.Fatalf("FilterTypes: %v", err)
	}
	if ft == "" {
		t.Error("FilterTypes returned empty string")
	}
	t.Logf("FilterTypes length: %d bytes", len(ft))
}
```

### TestIntegration_SignatureState

```go
func TestIntegration_SignatureState(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	lokPath := os.Getenv("LOK_PATH")
	if lokPath == "" {
		t.Skip("LOK_PATH not set")
	}
	o, err := New(lokPath)
	if err != nil {
		t.Skipf("cannot create Office: %v", err)
	}
	defer o.Close()

	doc, err := o.Load("testdata/hello.odt")
	if err != nil {
		t.Skipf("cannot load document: %v", err)
	}
	defer doc.Close()

	state, err := doc.SignatureState()
	if err != nil {
		t.Fatalf("SignatureState: %v", err)
	}
	t.Logf("SignatureState: %s", state)
}
```

### TestIntegration_RunMacro

```go
func TestIntegration_RunMacro(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	lokPath := os.Getenv("LOK_PATH")
	if lokPath == "" {
		t.Skip("LOK_PATH not set")
	}
	o, err := New(lokPath)
	if err != nil {
		t.Skipf("cannot create Office: %v", err)
	}
	defer o.Close()

	// Most LO installs don't ship user macros, so expect failure
	// but verify no crash.
	err = o.RunMacro("macro:///Standard.Module1.Main()")
	if err != nil {
		t.Logf("RunMacro returned expected error: %v", err)
	}
}
```

### TestIntegration_SignDocument

```go
func TestIntegration_SignDocument(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	lokPath := os.Getenv("LOK_PATH")
	if lokPath == "" {
		t.Skip("LOK_PATH not set")
	}
	certEnv := os.Getenv("LOK_TEST_CERTS")
	if certEnv == "" {
		t.Skip("LOK_TEST_CERTS not set (expected: /path/to/cert.pem,/path/to/key.pem)")
	}
	parts := strings.SplitN(certEnv, ",", 2)
	if len(parts) != 2 {
		t.Skipf("LOK_TEST_CERTS malformed: %q", certEnv)
	}
	certPEM, err := os.ReadFile(parts[0])
	if err != nil {
		t.Fatalf("read cert: %v", err)
	}
	keyPEM, err := os.ReadFile(parts[1])
	if err != nil {
		t.Fatalf("read key: %v", err)
	}

	o, err := New(lokPath)
	if err != nil {
		t.Skipf("cannot create Office: %v", err)
	}
	defer o.Close()

	// Create a temp document to sign.
	tmpDoc := filepath.Join(t.TempDir(), "sign-test.odt")
	doc, err := o.Load("testdata/hello.odt")
	if err != nil {
		t.Skipf("cannot load source document: %v", err)
	}
	if err := doc.SaveAs(tmpDoc, "odt", ""); err != nil {
		doc.Close()
		t.Fatalf("SaveAs: %v", err)
	}
	doc.Close()

	fileURL := "file://" + tmpDoc
	if err := o.SignDocument(fileURL, certPEM, keyPEM); err != nil {
		t.Logf("SignDocument: %v (may be expected without proper certs)", err)
	}
}
```

- [ ] Add tests and verify `go build ./lok`

## Step 2: Verify compilation

```bash
go build -tags=lok_integration ./lok
```

- [ ] Compiles cleanly

## Step 3: Run integration tests (best-effort)

```bash
LOK_PATH=/usr/lib/libreoffice/program go test -tags=lok_integration -run 'TestIntegration_FilterTypes|TestIntegration_SignatureState|TestIntegration_RunMacro|TestIntegration_SignDocument' ./lok -v -timeout 60s || true
```

- [ ] Tests run (SKIP or PASS acceptable; FAIL only if code bug)
