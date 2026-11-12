package histogram

import "heatmap/internal/imgx"

// Levels is the number of intensity levels of an 8-bit image.
const Levels = 256

// Histogram counts how many pixels fall on each of the 256 intensity levels.
// Pixel values are rounded and clamped to [0, 255] first.
func Histogram(g *imgx.Gray) [Levels]int {
	var h [Levels]int
	for _, v := range g.Pix {
		h[imgx.ToByte(v)]++
	}
	return h
}

// EqualizationLUT returns the lookup table of global histogram equalization
// for the given histogram:
//
//	lut[v] = round( (cdf(v) − cdfMin) / (N − cdfMin) · 255 )
//
// where cdf is the cumulative histogram, cdfMin its first non-zero value and
// N the number of pixels. The darkest present level maps to 0 and the
// brightest to 255. For an image with a single level (N == cdfMin) the table
// is the identity, since there is no contrast to stretch.
func EqualizationLUT(h [Levels]int) [Levels]float32 {
	var lut [Levels]float32
	total, cdfMin := 0, 0
	for _, n := range h {
		total += n
		if cdfMin == 0 {
			cdfMin = n
		}
	}
	if total == cdfMin {
		for v := range lut {
			lut[v] = float32(v)
		}
		return lut
	}

	scale := float64(Levels-1) / float64(total-cdfMin)
	cdf := 0
	for v, n := range h {
		cdf += n
		if cdf == 0 {
			continue // levels below the darkest present one stay 0
		}
		lut[v] = float32(int(float64(cdf-cdfMin)*scale + 0.5))
	}
	return lut
}

// Equalize returns the globally histogram-equalized image: intensities are
// remapped so that their cumulative distribution becomes close to linear,
// which stretches the contrast over the full [0, 255] range.
//
// The mapping depends only on the order of the intensities, so a frame and the
// same frame under a different global gain equalize to (almost) the same image.
func Equalize(g *imgx.Gray) *imgx.Gray {
	lut := EqualizationLUT(Histogram(g))
	out := imgx.NewGray(g.W, g.H)
	for i, v := range g.Pix {
		out.Pix[i] = lut[imgx.ToByte(v)]
	}
	return out
}
