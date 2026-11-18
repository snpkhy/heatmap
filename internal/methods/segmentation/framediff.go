package segmentation

import (
	"fmt"

	"heatmap/internal/imgx"
)

// FrameDiff is the simplest motion detector: a pixel is moving when its
// brightness changed by more than Threshold since the previous frame,
//
//	mask(x, y) = 255  if  |Iₜ(x, y) − Iₜ₋₁(x, y)| > Threshold,  else 0
//
// It needs no background model and reacts instantly to lighting changes that
// affect consecutive frames equally, but it only sees the edges of an object
// that moves (its uniform interior does not change between frames) and
// loses an object that stops.
//
// FrameDiff is stateful: feed it the frames of one sequence in order.
// It is not safe for concurrent use.
type FrameDiff struct {
	// Threshold is the smallest brightness change that counts as motion.
	Threshold float32

	prev *imgx.Gray
}

// NewFrameDiff returns a detector with the given threshold.
func NewFrameDiff(threshold float32) (*FrameDiff, error) {
	if threshold <= 0 {
		return nil, fmt.Errorf("segmentation: frame difference threshold must be positive, got %v", threshold)
	}
	return &FrameDiff{Threshold: threshold}, nil
}

// Apply compares the frame with the previous one and returns the motion mask
// (0 or 255). The first frame has nothing to compare with, so its mask is empty.
func (d *FrameDiff) Apply(frame *imgx.Gray) (*imgx.Gray, error) {
	mask := imgx.NewGray(frame.W, frame.H)
	if d.prev == nil {
		d.prev = frame.Clone()
		return mask, nil
	}
	if err := imgx.CheckSameSize(frame, d.prev); err != nil {
		return nil, fmt.Errorf("segmentation: frame difference: %w", err)
	}
	for i, v := range frame.Pix {
		diff := v - d.prev.Pix[i]
		if diff > d.Threshold || -diff > d.Threshold {
			mask.Pix[i] = MaskForeground
		}
	}
	copy(d.prev.Pix, frame.Pix)
	return mask, nil
}

// Reset forgets the previous frame, e.g. before a new sequence.
func (d *FrameDiff) Reset() {
	d.prev = nil
}
