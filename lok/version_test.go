package lok

import (
	"errors"
	"testing"
)

func TestVersionInfo_ParsesJSON(t *testing.T) {
	fb := &fakeBackend{version: `{"ProductName":"LibreOffice","ProductVersion":"24.8.7.2","BuildId":"abc123"}`}
	withFakeBackend(t, fb)
	o, err := New("/install")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer o.Close()

	vi, err := o.VersionInfo()
	if err != nil {
		t.Fatalf("VersionInfo: %v", err)
	}
	if vi.ProductName != "LibreOffice" || vi.ProductVersion != "24.8.7.2" || vi.BuildID != "abc123" {
		t.Errorf("unexpected: %+v", vi)
	}
}

func TestVersionInfo_EmptyStringIsError(t *testing.T) {
	fb := &fakeBackend{version: ""}
	withFakeBackend(t, fb)
	o, err := New("/install")
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	_, err = o.VersionInfo()
	var lokErr *LOKError
	if !errors.As(err, &lokErr) {
		t.Errorf("want *LOKError, got %T %v", err, err)
	}
}

func TestVersionInfo_InvalidJSONIsError(t *testing.T) {
	fb := &fakeBackend{version: "not json"}
	withFakeBackend(t, fb)
	o, err := New("/install")
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	_, err = o.VersionInfo()
	var lokErr *LOKError
	if !errors.As(err, &lokErr) {
		t.Errorf("want *LOKError for invalid JSON, got %T %v", err, err)
	}
}

func TestVersionInfo_AfterCloseErrors(t *testing.T) {
	withFakeBackend(t, &fakeBackend{version: `{"ProductName":"x"}`})
	o, err := New("/install")
	if err != nil {
		t.Fatal(err)
	}
	o.Close()
	if _, err := o.VersionInfo(); !errors.Is(err, ErrClosed) {
		t.Errorf("want ErrClosed, got %v", err)
	}
}
