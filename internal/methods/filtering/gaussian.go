package filtering

import (
	"fmt"
	"math"

	"heatmap/internal/imgx"
)

// Gaussian applies a 2D Gaussian blur with a k×k kernel and standard deviation sigma.
// Gaussian filtering attenuates high-frequency noise while preserving edges better
// than a box filter.
//
// The 2D Gaussian kernel is separable:
//
//	G(x, y) = G_1D(x) · G_1D(y), where G_1D(t) = exp(-t² / (2σ²))
//
// It is applied as two consecutive 1D passes (horizontal then vertical),
// taking O(k) operations per pixel instead of O(k²).
//
// k must be odd and positive; k = 1 returns an unchanged copy.
// If sigma <= 0, a standard default derived from k is used: σ = 0.3·((k−1)·0.5 − 1) + 0.8
// (or σ = 1.0 for k = 3).
// Pixels outside the image replicate the nearest border pixel.
func Gaussian(src *imgx.Gray, k int, sigma float64) (*imgx.Gray, error) {
	if k < 1 || k%2 == 0 {
		return nil, fmt.Errorf("filtering: gaussian kernel size must be odd and positive, got %d", k)
	}
	if k == 1 {
		return src.Clone(), nil
	}
	if sigma <= 0 {
		sigma = 0.3*(float64(k/2)-1.0) + 0.8
		if sigma <= 0 {
			sigma = 1.0
		}
	}

	kernel := gaussianKernel1D(k, sigma)
	r := k / 2

	// Horizontal pass
	tmp := imgx.NewGray(src.W, src.H)
	for y := 0; y < src.H; y++ {
		for x := 0; x < src.W; x++ {
			var sum float64
			for i := -r; i <= r; i++ {
				sum += float64(src.AtClamped(x+i, y)) * kernel[i+r]
			}
			tmp.Set(x, y, float32(sum))
		}
	}

	// Vertical pass
	dst := imgx.NewGray(src.W, src.H)
	for y := 0; y < src.H; y++ {
		for x := 0; x < src.W; x++ {
			var sum float64
			for i := -r; i <= r; i++ {
				sum += float64(tmp.AtClamped(x, y+i)) * kernel[i+r]
			}
			dst.Set(x, y, float32(sum))
		}
	}

	return dst, nil
}

// gaussianKernel1D generates a normalized 1D Gaussian kernel of size k with standard deviation sigma.
func gaussianKernel1D(k int, sigma float64) []float64 {
	kernel := make([]float64, k)
	r := k / 2
	twoSigmaSq := 2.0 * sigma * sigma
	var sum float64

	for i := -r; i <= r; i++ {
		v := math.Exp(-float64(i*i) / twoSigmaSq)
		kernel[i+r] = v
		sum += v
	}

	// Normalize so that kernel elements sum to 1
	for i := range kernel {
		kernel[i] /= sum
	}

	return kernel
}
