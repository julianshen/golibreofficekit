package lok

// OptionalFeatures is a bitmask mirroring LibreOfficeKitOptionalFeatures
// from LibreOfficeKitEnums.h. Values are stable across the 24.x series.
type OptionalFeatures uint64

// The bit values mirror the 1ULL << N constants in
// LibreOfficeKitOptionalFeatures. Keep ordering identical to the
// upstream header; add new values at the end as upstream grows.
const (
	FeatureDocumentPassword              OptionalFeatures = 1 << 0 // LOK_FEATURE_DOCUMENT_PASSWORD
	FeatureDocumentPasswordToModify      OptionalFeatures = 1 << 1 // LOK_FEATURE_DOCUMENT_PASSWORD_TO_MODIFY
	FeaturePartInInvalidation            OptionalFeatures = 1 << 2 // LOK_FEATURE_PART_IN_INVALIDATION_CALLBACK
	FeatureNoTiledAnnotations            OptionalFeatures = 1 << 3 // LOK_FEATURE_NO_TILED_ANNOTATIONS
	FeatureRangeHeaders                  OptionalFeatures = 1 << 4 // LOK_FEATURE_RANGE_HEADERS
	FeatureViewIdInVisCursorInvalidation OptionalFeatures = 1 << 5 // LOK_FEATURE_VIEWID_IN_VISCURSOR_INVALIDATION_CALLBACK
)

// Has reports whether the given bit is set.
func (f OptionalFeatures) Has(bit OptionalFeatures) bool { return f&bit == bit }

// SetOptionalFeatures updates the LO optional-features mask.
// Returns ErrClosed if the Office has been closed.
func (o *Office) SetOptionalFeatures(f OptionalFeatures) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return ErrClosed
	}
	o.be.OfficeSetOptionalFeatures(o.h, uint64(f))
	return nil
}
