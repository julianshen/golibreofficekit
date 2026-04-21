package lok

import "testing"

func TestOptionalFeatures_BitwiseOr(t *testing.T) {
	f := FeatureDocumentPassword | FeatureNoTiledAnnotations
	if !f.Has(FeatureDocumentPassword) {
		t.Error("missing FeatureDocumentPassword")
	}
	if !f.Has(FeatureNoTiledAnnotations) {
		t.Error("missing FeatureNoTiledAnnotations")
	}
	if f.Has(FeaturePartInInvalidation) {
		t.Error("FeaturePartInInvalidation should not be set")
	}
}

func TestSetOptionalFeatures_PassesMaskThrough(t *testing.T) {
	fb := &recordingBackend{fakeBackend: fakeBackend{version: "{}"}}
	withFakeBackend(t, &fb.fakeBackend)
	// Replace the backend with the recording one so SetOptionalFeatures
	// is captured.
	setBackend(fb)
	t.Cleanup(func() { setBackend(&fb.fakeBackend) })

	o, err := New("/install")
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	mask := FeatureDocumentPassword | FeatureViewIdInVisCursorInvalidation
	if err := o.SetOptionalFeatures(mask); err != nil {
		t.Fatal(err)
	}
	if fb.lastFeatures != uint64(mask) {
		t.Errorf("want 0x%x, got 0x%x", uint64(mask), fb.lastFeatures)
	}
}

type recordingBackend struct {
	fakeBackend
	lastFeatures uint64
}

func (r *recordingBackend) OfficeSetOptionalFeatures(h officeHandle, f uint64) {
	r.lastFeatures = f
}

func TestSetOptionalFeatures_AfterCloseErrors(t *testing.T) {
	withFakeBackend(t, &fakeBackend{})
	o, err := New("/install")
	if err != nil {
		t.Fatal(err)
	}
	o.Close()
	if err := o.SetOptionalFeatures(FeatureDocumentPassword); err != ErrClosed {
		t.Errorf("want ErrClosed, got %v", err)
	}
}
