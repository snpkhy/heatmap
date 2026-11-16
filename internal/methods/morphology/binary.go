package morphology

import (
	"fmt"

	"heatmap/internal/imgx"
)

// Mask values shared with the metrics package: foreground is ≥ 128.
const (
	maskThresh     = 128
	maskBackground = 0
	maskForeground = 255
)

// Dilate grows foreground regions: a pixel becomes foreground when any pixel
// of its k×k square neighbourhood is foreground. Pixels outside the image
// are ignored. k must be odd and positive.
func Dilate(src *imgx.Gray, k int) (*imgx.Gray, error) {
	return apply(src, k, true)
}

// Erode shrinks foreground regions: a pixel stays foreground only when every
// pixel of its k×k square neighbourhood is foreground. Pixels outside the
// image are ignored, so objects touching the border are not eaten away.
// k must be odd and positive.
func Erode(src *imgx.Gray, k int) (*imgx.Gray, error) {
	return apply(src, k, false)
}

// Closing is dilation followed by erosion with the same k×k square.
// It fills holes and gaps narrower than k and joins nearby fragments,
// while keeping the outer outline of objects roughly in place.
func Closing(src *imgx.Gray, k int) (*imgx.Gray, error) {
	d, err := Dilate(src, k)
	if err != nil {
		return nil, err
	}
	return Erode(d, k)
}

// apply runs dilation (grow = true) or erosion (grow = false) on a binary mask.
func apply(src *imgx.Gray, k int, grow bool) (*imgx.Gray, error) {
	if k < 1 || k%2 == 0 {
		return nil, fmt.Errorf("morphology: window size must be odd and positive, got %d", k)
	}
	r := k / 2
	dst := imgx.NewGray(src.W, src.H)

	for y := 0; y < src.H; y++ {
		for x := 0; x < src.W; x++ {
			// Dilation looks for any foreground pixel, erosion for any background one.
			// When found, the answer is known and the window scan stops early.
			result := !grow
		scan:
			for dy := -r; dy <= r; dy++ {
				for dx := -r; dx <= r; dx++ {
					if !src.InBounds(x+dx, y+dy) {
						continue
					}
					fg := src.At(x+dx, y+dy) >= maskThresh
					if fg == grow {
						result = grow
						break scan
					}
				}
			}
			if result {
				dst.Set(x, y, maskForeground)
			} else {
				dst.Set(x, y, maskBackground)
			}
		}
	}
	return dst, nil
}
