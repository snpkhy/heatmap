package filtering

import (
	"fmt"

	"heatmap/internal/imgx"
)

// Integral is a summed-area table (integral image) of a Gray image.
// Cell (x, y) holds the sum of all pixels above and to the left of (x, y),
// so the sum of any rectangle is found from four cells in constant time.
type Integral struct {
	W, H int
	// sum has (W+1)×(H+1) cells; the first row and column are zero.
	// float64 is required: a 320×240 frame already sums past float32 precision.
	sum []float64
}

// NewIntegral builds the integral image of src in a single pass:
//
//	S(x, y) = src(x−1, y−1) + S(x−1, y) + S(x, y−1) − S(x−1, y−1)
func NewIntegral(src *imgx.Gray) *Integral {
	stride := src.W + 1
	s := &Integral{W: src.W, H: src.H, sum: make([]float64, stride*(src.H+1))}
	for y := 1; y <= src.H; y++ {
		for x := 1; x <= src.W; x++ {
			s.sum[y*stride+x] = float64(src.At(x-1, y-1)) +
				s.sum[y*stride+x-1] + s.sum[(y-1)*stride+x] - s.sum[(y-1)*stride+x-1]
		}
	}
	return s
}

// Sum returns the sum of the pixels with x0 ≤ x < x1 and y0 ≤ y < y1.
// The rectangle is clipped to the image; an empty rectangle sums to 0.
func (s *Integral) Sum(x0, y0, x1, y1 int) float64 {
	x0, x1 = max(x0, 0), min(x1, s.W)
	y0, y1 = max(y0, 0), min(y1, s.H)
	if x0 >= x1 || y0 >= y1 {
		return 0
	}
	stride := s.W + 1
	return s.sum[y1*stride+x1] - s.sum[y0*stride+x1] - s.sum[y1*stride+x0] + s.sum[y0*stride+x0]
}

// Box replaces every pixel with the mean of its k×k neighbourhood. Thanks to
// the integral image the cost per pixel does not depend on k.
//
// k must be odd and positive; k = 1 returns an unchanged copy.
// Near the border the window is clipped and the mean is taken over the
// pixels that lie inside the image.
func Box(src *imgx.Gray, k int) (*imgx.Gray, error) {
	if k < 1 || k%2 == 0 {
		return nil, fmt.Errorf("filtering: box window size must be odd and positive, got %d", k)
	}
	r := k / 2
	integral := NewIntegral(src)
	dst := imgx.NewGray(src.W, src.H)

	for y := 0; y < src.H; y++ {
		y0, y1 := max(y-r, 0), min(y+r+1, src.H)
		for x := 0; x < src.W; x++ {
			x0, x1 := max(x-r, 0), min(x+r+1, src.W)
			area := float64((x1 - x0) * (y1 - y0))
			dst.Set(x, y, float32(integral.Sum(x0, y0, x1, y1)/area))
		}
	}
	return dst, nil
}
