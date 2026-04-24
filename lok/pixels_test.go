//go:build linux || darwin

package lok

import (
	"bytes"
	"testing"
)

func TestUnpremultiplyBGRAToNRGBA_OpaqueRoundTrip(t *testing.T) {
	// Opaque white: BGRA premul (255, 255, 255, 255) → NRGBA (255, 255, 255, 255).
	src := []byte{255, 255, 255, 255}
	dst := make([]byte, 4)
	unpremultiplyBGRAToNRGBA(dst, src, 1, 1)
	want := []byte{255, 255, 255, 255}
	if !bytes.Equal(dst, want) {
		t.Errorf("got %v, want %v", dst, want)
	}
}

func TestUnpremultiplyBGRAToNRGBA_SwizzlesBGRAToRGBA(t *testing.T) {
	// Opaque red in premul BGRA = (B=0, G=0, R=255, A=255) → NRGBA (R=255, G=0, B=0, A=255).
	src := []byte{0, 0, 255, 255}
	dst := make([]byte, 4)
	unpremultiplyBGRAToNRGBA(dst, src, 1, 1)
	want := []byte{255, 0, 0, 255}
	if !bytes.Equal(dst, want) {
		t.Errorf("got %v, want %v", dst, want)
	}
}

func TestUnpremultiplyBGRAToNRGBA_UnpremultipliesHalfAlpha(t *testing.T) {
	// 50% red premul: (B=0, G=0, R=128, A=128) → straight (R=255, G=0, B=0, A=128).
	src := []byte{0, 0, 128, 128}
	dst := make([]byte, 4)
	unpremultiplyBGRAToNRGBA(dst, src, 1, 1)
	want := []byte{255, 0, 0, 128}
	if !bytes.Equal(dst, want) {
		t.Errorf("got %v, want %v", dst, want)
	}
}

func TestUnpremultiplyBGRAToNRGBA_ZeroAlpha(t *testing.T) {
	// α=0 → (0, 0, 0, 0) regardless of src RGB.
	src := []byte{200, 100, 50, 0}
	dst := make([]byte, 4)
	unpremultiplyBGRAToNRGBA(dst, src, 1, 1)
	want := []byte{0, 0, 0, 0}
	if !bytes.Equal(dst, want) {
		t.Errorf("got %v, want %v", dst, want)
	}
}

func TestUnpremultiplyBGRAToNRGBA_LowAlphaClamps(t *testing.T) {
	// α=1 with channels=1 → straight (255, 255, 255, 1) — 1*255/1 = 255.
	src := []byte{1, 1, 1, 1}
	dst := make([]byte, 4)
	unpremultiplyBGRAToNRGBA(dst, src, 1, 1)
	want := []byte{255, 255, 255, 1}
	if !bytes.Equal(dst, want) {
		t.Errorf("got %v, want %v", dst, want)
	}
}

func TestUnpremultiplyBGRAToNRGBA_TwoByTwo(t *testing.T) {
	// Four pixels, in raster order: opaque red, opaque green, opaque blue, half red.
	src := []byte{
		0, 0, 255, 255, // red
		0, 255, 0, 255, // green
		255, 0, 0, 255, // blue
		0, 0, 128, 128, // 50% red
	}
	dst := make([]byte, len(src))
	unpremultiplyBGRAToNRGBA(dst, src, 2, 2)
	want := []byte{
		255, 0, 0, 255,
		0, 255, 0, 255,
		0, 0, 255, 255,
		255, 0, 0, 128,
	}
	if !bytes.Equal(dst, want) {
		t.Errorf("got %v, want %v", dst, want)
	}
}

func TestUnpremultiplyBGRAToNRGBA_InPlace(t *testing.T) {
	// Contract: src and dst may alias. Future paint-convenience
	// wrappers rely on this — passing img.Pix as both src and dst
	// halves allocation per tile. Assert the in-place result equals
	// the non-aliased result for the same input.
	input := []byte{
		0, 0, 255, 255, // red
		0, 0, 128, 128, // 50% red
	}

	dst := make([]byte, len(input))
	unpremultiplyBGRAToNRGBA(dst, input, 2, 1)

	inPlace := append([]byte(nil), input...)
	unpremultiplyBGRAToNRGBA(inPlace, inPlace, 2, 1)

	if !bytes.Equal(dst, inPlace) {
		t.Errorf("in-place: got %v, want %v", inPlace, dst)
	}
}
