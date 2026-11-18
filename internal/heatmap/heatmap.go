package heatmap

import (
	"fmt"

	"heatmap/internal/imgx"
)

// maskThresh is the level from which a mask pixel counts as motion (as in metrics).
const maskThresh = 128

// Accumulator builds an activity heatmap from a sequence of motion masks:
//
//	H ← Decay·H + M,   M(x, y) = 1 where the mask marks motion, else 0
//
// With Decay = 1 the heatmap counts how many frames each pixel was moving.
// With Decay < 1 old activity fades out (a pixel last active n frames ago
// weighs Decayⁿ), which suits live views; 1/(1 − Decay) is the effective
// memory in frames.
//
// It is not safe for concurrent use.
type Accumulator struct {
	decay  float32
	w, h   int
	acc    []float32
	frames int
}

// NewAccumulator returns an empty heatmap. decay must be in (0, 1].
func NewAccumulator(decay float32) (*Accumulator, error) {
	if !(decay > 0 && decay <= 1) {
		return nil, fmt.Errorf("heatmap: decay %v must be in (0, 1]", decay)
	}
	return &Accumulator{decay: decay}, nil
}

// Add accumulates one motion mask. All masks must have the size of the first one.
func (a *Accumulator) Add(mask *imgx.Gray) error {
	if a.acc == nil {
		a.w, a.h = mask.W, mask.H
		a.acc = make([]float32, len(mask.Pix))
	} else if mask.W != a.w || mask.H != a.h {
		return fmt.Errorf("%w: mask %dx%d, heatmap %dx%d", imgx.ErrSizeMismatch, mask.W, mask.H, a.w, a.h)
	}
	for i, v := range mask.Pix {
		a.acc[i] *= a.decay
		if v >= maskThresh {
			a.acc[i]++
		}
	}
	a.frames++
	return nil
}

// Frames returns how many masks were added.
func (a *Accumulator) Frames() int { return a.frames }

// Size returns the heatmap size, or 0×0 before the first mask.
func (a *Accumulator) Size() (w, h int) { return a.w, a.h }

// Raw returns a copy of the accumulated values (frames of activity, decayed).
// It returns nil before the first mask.
func (a *Accumulator) Raw() *imgx.Gray {
	if a.acc == nil {
		return nil
	}
	out := imgx.NewGray(a.w, a.h)
	copy(out.Pix, a.acc)
	return out
}

// Max returns the largest accumulated value.
func (a *Accumulator) Max() float32 {
	var m float32
	for _, v := range a.acc {
		m = max(m, v)
	}
	return m
}

// Normalized returns the heatmap scaled to [0, 255] so that the most active
// pixel is 255. An all-quiet heatmap is all 0. It returns nil before the first mask.
func (a *Accumulator) Normalized() *imgx.Gray {
	if a.acc == nil {
		return nil
	}
	out := imgx.NewGray(a.w, a.h)
	peak := a.Max()
	if peak == 0 {
		return out
	}
	for i, v := range a.acc {
		out.Pix[i] = 255 * v / peak
	}
	return out
}

// Reset empties the heatmap.
func (a *Accumulator) Reset() {
	a.acc, a.w, a.h, a.frames = nil, 0, 0, 0
}
