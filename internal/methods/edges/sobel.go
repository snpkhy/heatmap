package edges

import (
	"fmt"
	"math"

	"heatmap/internal/imgx"
)

// Mask values shared with the metrics package: foreground is ≥ 128.
const (
	maskBackground = 0
	maskForeground = 255
)

// Sobel kernels for the horizontal (x) and vertical (y) derivative.
var (
	sobelX = [3][3]float32{
		{-1, 0, 1},
		{-2, 0, 2},
		{-1, 0, 1},
	}
	sobelY = [3][3]float32{
		{-1, -2, -1},
		{0, 0, 0},
		{1, 2, 1},
	}
)

// SobelMagnitude returns the gradient magnitude sqrt(Gx² + Gy²) of every pixel.
// Values are not clamped: they range from 0 on flat areas up to about 1442
// on a sharp black/white edge. Pixels outside the image replicate the border.
func SobelMagnitude(src *imgx.Gray) *imgx.Gray {
	dst := imgx.NewGray(src.W, src.H)
	for y := 0; y < src.H; y++ {
		for x := 0; x < src.W; x++ {
			var gx, gy float32
			for ky := 0; ky < 3; ky++ {
				for kx := 0; kx < 3; kx++ {
					v := src.AtClamped(x+kx-1, y+ky-1)
					gx += sobelX[ky][kx] * v
					gy += sobelY[ky][kx] * v
				}
			}
			dst.Set(x, y, float32(math.Sqrt(float64(gx*gx+gy*gy))))
		}
	}
	return dst
}

// SobelDiff detects motion as a change of edges rather than of brightness.
// It keeps a background model of gradient magnitudes and marks a pixel as
// moving when the current gradient differs from the background by more than
// Threshold. A global brightness change shifts all pixels equally and leaves
// gradients almost unchanged, so it is not reported as motion.
//
// SobelDiff is stateful: feed it the frames of one sequence in order.
type SobelDiff struct {
	// Threshold is the minimum |G − B| that counts as motion.
	Threshold float32
	// Alpha is the background learning rate in (0, 1]:
	// B ← (1 − Alpha)·B + Alpha·G. Small values adapt slowly.
	Alpha float32

	background *imgx.Gray
}

// NewSobelDiff returns a detector with the given threshold and learning rate.
func NewSobelDiff(threshold, alpha float32) (*SobelDiff, error) {
	if threshold <= 0 {
		return nil, fmt.Errorf("edges: sobel diff threshold must be positive, got %v", threshold)
	}
	if alpha <= 0 || alpha > 1 {
		return nil, fmt.Errorf("edges: sobel diff alpha must be in (0, 1], got %v", alpha)
	}
	return &SobelDiff{Threshold: threshold, Alpha: alpha}, nil
}

// Apply processes the next frame and returns its motion mask (0 or 255).
// The first frame only initialises the background, so its mask is empty.
func (s *SobelDiff) Apply(frame *imgx.Gray) (*imgx.Gray, error) {
	g := SobelMagnitude(frame)
	mask := imgx.NewGray(frame.W, frame.H)

	if s.background == nil {
		s.background = g
		return mask, nil
	}
	if err := imgx.CheckSameSize(g, s.background); err != nil {
		return nil, fmt.Errorf("edges: sobel diff: %w", err)
	}

	b := s.background.Pix
	for i, gv := range g.Pix {
		d := gv - b[i]
		if d < 0 {
			d = -d
		}
		if d > s.Threshold {
			mask.Pix[i] = maskForeground
		} else {
			mask.Pix[i] = maskBackground
		}
		b[i] = (1-s.Alpha)*b[i] + s.Alpha*gv
	}
	return mask, nil
}

// Reset forgets the background, e.g. before a new sequence.
func (s *SobelDiff) Reset() {
	s.background = nil
}
