//go:build linux || darwin

package lok

// unpremultiplyBGRAToNRGBA copies src (premul BGRA, 4*pxW*pxH bytes,
// byte order B, G, R, A with RGB premultiplied by A) into dst
// (straight NRGBA, byte order R, G, B, A). len(dst) and len(src) must
// both equal 4*pxW*pxH — callers validate. α=0 pixels yield
// (0, 0, 0, 0) regardless of src RGB. src and dst may alias.
func unpremultiplyBGRAToNRGBA(dst, src []byte, pxW, pxH int) {
	n := 4 * pxW * pxH
	for i := 0; i < n; i += 4 {
		b, g, r, a := src[i], src[i+1], src[i+2], src[i+3]
		if a == 0 {
			dst[i], dst[i+1], dst[i+2], dst[i+3] = 0, 0, 0, 0
			continue
		}
		dst[i] = uint8(uint16(r) * 255 / uint16(a))
		dst[i+1] = uint8(uint16(g) * 255 / uint16(a))
		dst[i+2] = uint8(uint16(b) * 255 / uint16(a))
		dst[i+3] = a
	}
}
