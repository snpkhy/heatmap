package color

import "heatmap/internal/imgx"

// DefaultMinSum is the default R+G+B below which a pixel is treated as too
// dark to have a reliable chromaticity (see Chromaticity).
const DefaultMinSum = 30

// chromaScale maps chromaticity fractions from [0, 1] to the usual [0, 255] range.
const chromaScale = 255

// Chromaticity returns the chromaticity-normalized image
//
//	r = R / (R+G+B),  g = G / (R+G+B),  b = B / (R+G+B)
//
// with every plane scaled to [0, 255], so the three planes of a pixel sum to 255.
// Scaling all channels of a pixel by the same factor (a shadow, a brighter or
// dimmer lamp) leaves its chromaticity unchanged, which is why it is used to
// separate real objects from illumination effects.
//
// When R+G+B < minSum the division amplifies sensor noise, so such pixels are
// reported as neutral gray (r = g = b = 85).
func Chromaticity(c *imgx.RGB, minSum float32) *imgx.RGB {
	out := imgx.NewRGB(c.W, c.H)
	for i := range c.R {
		out.R[i], out.G[i], out.B[i] = pixelChroma(c.R[i], c.G[i], c.B[i], minSum)
	}
	return out
}

// ChromaValue builds the shadow-aware feature image used by pipeline p7.
// The planes of the result are not colors:
//
//	R plane: chromaticity r, scaled to [0, 255]
//	G plane: chromaticity g, scaled to [0, 255]
//	B plane: HSV value V = max(R, G, B)
//
// The two chromaticity planes ignore brightness, and V carries only brightness,
// so a later stage can tell "the color changed" from "it only got darker".
func ChromaValue(c *imgx.RGB, minSum float32) *imgx.RGB {
	out := imgx.NewRGB(c.W, c.H)
	for i := range c.R {
		r, g, b := c.R[i], c.G[i], c.B[i]
		out.R[i], out.G[i], _ = pixelChroma(r, g, b, minSum)
		out.B[i] = max(r, g, b)
	}
	return out
}

func pixelChroma(r, g, b, minSum float32) (cr, cg, cb float32) {
	sum := r + g + b
	if sum < minSum || sum <= 0 {
		const neutral = chromaScale / 3.0
		return neutral, neutral, neutral
	}
	k := chromaScale / sum
	return r * k, g * k, b * k
}
