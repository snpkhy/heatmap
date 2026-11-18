package segmentation

import (
	"fmt"
	"math"

	"heatmap/internal/imgx"
)

// RunningAvgBGConfig holds the parameters for RunningAvgBG.
type RunningAvgBGConfig struct {
	// Alpha is the learning rate for pixels classified as background, in (0, 1].
	// Typical value: 0.05.
	Alpha float32

	// FGAlpha is the learning rate for pixels classified as foreground, in [0, Alpha].
	// A small non-zero value lets the model slowly absorb stopped objects while
	// preventing fast-moving objects from corrupting the background.
	// Default: 0.005.
	FGAlpha float32

	// MinThreshold is the minimum absolute difference required to consider motion.
	// It prevents sensor noise on completely static frames from triggering false positives.
	// Default: 15.0.
	MinThreshold float32
}

// DefaultRunningAvgBGConfig returns standard default settings.
func DefaultRunningAvgBGConfig() RunningAvgBGConfig {
	return RunningAvgBGConfig{
		Alpha:        0.05,
		FGAlpha:      0.005,
		MinThreshold: 15.0,
	}
}

func (c RunningAvgBGConfig) validate() error {
	if c.Alpha <= 0 || c.Alpha > 1 {
		return fmt.Errorf("segmentation: Alpha %v must be in (0, 1]", c.Alpha)
	}
	if c.FGAlpha < 0 || c.FGAlpha > c.Alpha {
		return fmt.Errorf("segmentation: FGAlpha %v must be in [0, Alpha %v]", c.FGAlpha, c.Alpha)
	}
	if c.MinThreshold < 0 {
		return fmt.Errorf("segmentation: MinThreshold %v must be non-negative", c.MinThreshold)
	}
	return nil
}

// RunningAvgBG implements a running-average background model paired with Otsu thresholding.
//
// On each frame:
//  1. The absolute difference between the incoming frame and the current background is computed:
//     D(x, y) = |I_t(x, y) - B_{t-1}(x, y)|
//  2. Otsu's method determines an optimal separation threshold T from D.
//     T is clamped to at least MinThreshold.
//  3. Pixels with D(x, y) >= T are marked as foreground (255), others as background (0).
//  4. The background is updated via exponential moving average:
//     B_t(x, y) = (1 - rate)·B_{t-1}(x, y) + rate·I_t(x, y)
//     where rate is Alpha for background pixels and FGAlpha for foreground pixels.
type RunningAvgBG struct {
	cfg    RunningAvgBGConfig
	bg     *imgx.Gray
	diff   *imgx.Gray
	frames int
}

// NewRunningAvgBG returns an uninitialized model ready to process a frame sequence.
func NewRunningAvgBG(cfg RunningAvgBGConfig) (*RunningAvgBG, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &RunningAvgBG{cfg: cfg}, nil
}

// Reset clears the accumulated background state.
func (m *RunningAvgBG) Reset() {
	m.bg = nil
	m.diff = nil
	m.frames = 0
}

// Frames returns the number of frames processed so far.
func (m *RunningAvgBG) Frames() int {
	return m.frames
}

// Background returns a clone of the current background model, or nil if no frames have been seen.
func (m *RunningAvgBG) Background() *imgx.Gray {
	if m.bg == nil {
		return nil
	}
	return m.bg.Clone()
}

// Difference returns a clone of the last frame difference, or nil if no frames have been seen.
func (m *RunningAvgBG) Difference() *imgx.Gray {
	if m.diff == nil {
		return nil
	}
	return m.diff.Clone()
}

// Apply processes the next frame and returns a binary motion mask (0 or 255).
// The first frame initializes the background model and returns an all-background mask.
func (m *RunningAvgBG) Apply(frame *imgx.Gray) (*imgx.Gray, error) {
	if frame == nil {
		return nil, fmt.Errorf("segmentation: input frame cannot be nil")
	}

	if m.bg == nil {
		m.bg = frame.Clone()
		m.diff = imgx.NewGray(frame.W, frame.H)
		m.frames = 1
		return imgx.NewGray(frame.W, frame.H), nil
	}

	if err := imgx.CheckSameSize(m.bg, frame); err != nil {
		return nil, fmt.Errorf("segmentation: %w", err)
	}

	mask := imgx.NewGray(frame.W, frame.H)
	n := len(frame.Pix)

	// Compute absolute difference
	for i := 0; i < n; i++ {
		d := float32(math.Abs(float64(frame.Pix[i] - m.bg.Pix[i])))
		m.diff.Pix[i] = d
	}

	// Calculate Otsu threshold on the difference image
	thresh := Otsu(m.diff)
	if thresh < m.cfg.MinThreshold {
		thresh = m.cfg.MinThreshold
	}

	// Segment and update background
	alpha := m.cfg.Alpha
	fgAlpha := m.cfg.FGAlpha

	for i := 0; i < n; i++ {
		if m.diff.Pix[i] > thresh {
			mask.Pix[i] = MaskForeground
			m.bg.Pix[i] = (1.0-fgAlpha)*m.bg.Pix[i] + fgAlpha*frame.Pix[i]
		} else {
			mask.Pix[i] = MaskBackground
			m.bg.Pix[i] = (1.0-alpha)*m.bg.Pix[i] + alpha*frame.Pix[i]
		}
	}

	m.frames++
	return mask, nil
}
