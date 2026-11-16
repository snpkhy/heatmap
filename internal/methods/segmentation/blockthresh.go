package segmentation

import (
	"fmt"

	"heatmap/internal/imgx"
)

// blockForeground is the mask value of a foreground block (metrics: foreground ≥ 128).
const blockForeground = 255

// BlockThreshold segments an image block by block instead of pixel by pixel.
// The image is split into block×block cells; a cell whose mean value is
// above threshold becomes foreground (255) as a whole, the rest stay 0.
//
// Deciding per block makes the mask robust to single noisy pixels at the
// cost of a coarser outline. Cells cut off by the right and bottom borders
// are averaged over the pixels they actually contain.
func BlockThreshold(src *imgx.Gray, block int, threshold float32) (*imgx.Gray, error) {
	if block < 1 {
		return nil, fmt.Errorf("segmentation: block threshold block size must be positive, got %d", block)
	}
	dst := imgx.NewGray(src.W, src.H)

	for by := 0; by < src.H; by += block {
		y1 := min(by+block, src.H)
		for bx := 0; bx < src.W; bx += block {
			x1 := min(bx+block, src.W)

			var sum float64
			for y := by; y < y1; y++ {
				for x := bx; x < x1; x++ {
					sum += float64(src.At(x, y))
				}
			}
			mean := sum / float64((x1-bx)*(y1-by))
			if mean <= float64(threshold) {
				continue
			}
			for y := by; y < y1; y++ {
				for x := bx; x < x1; x++ {
					dst.Set(x, y, blockForeground)
				}
			}
		}
	}
	return dst, nil
}
