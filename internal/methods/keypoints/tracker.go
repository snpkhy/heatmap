package keypoints

import (
	"fmt"

	"heatmap/internal/imgx"
)

// TrackerConfig holds the parameters of Tracker.
// Start from DefaultTrackerConfig and change what you need.
type TrackerConfig struct {
	// Harris configures corner detection in the previous frame.
	Harris HarrisConfig
	// Block is the side of the matched window in pixels, odd and ≥ 3.
	Block int
	// Search is the largest displacement in x and y, in pixels, that block
	// matching tries. It bounds the speed that can be measured.
	Search int
	// MaxCost rejects matches whose mean absolute difference per pixel is
	// above it: the corner was occluded or changed too much to be trusted.
	MaxCost float32
	// MinMotion is the displacement in pixels from which a corner counts as
	// moving. Corners of the static background match with (0, 0).
	MinMotion float64
	// MaskRadius is the radius of the disc drawn around every moving corner
	// in the motion mask.
	MaskRadius int
}

// DefaultTrackerConfig returns parameters for 320×240 surveillance frames.
func DefaultTrackerConfig() TrackerConfig {
	return TrackerConfig{
		Harris:     DefaultHarrisConfig(),
		Block:      9,
		Search:     8,
		MaxCost:    20,
		MinMotion:  1,
		MaskRadius: 4,
	}
}

func (c TrackerConfig) validate() error {
	if err := c.Harris.validate(); err != nil {
		return err
	}
	switch {
	case c.Block < 3 || c.Block%2 == 0:
		return fmt.Errorf("keypoints: tracker Block %d must be odd and at least 3", c.Block)
	case c.Search < 1:
		return fmt.Errorf("keypoints: tracker Search %d must be at least 1", c.Search)
	case c.MaxCost <= 0:
		return fmt.Errorf("keypoints: tracker MaxCost %v must be positive", c.MaxCost)
	case c.MinMotion < 0 || c.MaskRadius < 0:
		return fmt.Errorf("keypoints: tracker MinMotion %v and MaskRadius %d must not be negative",
			c.MinMotion, c.MaskRadius)
	}
	return nil
}

// Tracker follows corners from frame to frame, the tracking stage of p6.
// For every new frame it detects Harris corners in the previous frame, finds
// each of them in the new frame by block matching, and keeps the confident
// matches (Cost ≤ MaxCost) that moved by at least MinMotion pixels.
//
// Tracker is stateful: feed it the frames of one sequence in order.
// It is not safe for concurrent use.
type Tracker struct {
	cfg  TrackerConfig
	prev *imgx.Gray
}

// NewTracker returns a tracker, or an error if the configuration is invalid.
func NewTracker(cfg TrackerConfig) (*Tracker, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &Tracker{cfg: cfg}, nil
}

// Apply processes the next frame and returns the motion vectors of the
// moving corners together with a motion mask (see MotionMask). The first
// frame has no predecessor: it yields no vectors and an empty mask.
func (t *Tracker) Apply(frame *imgx.Gray) ([]Match, *imgx.Gray, error) {
	if t.prev == nil {
		t.prev = frame.Clone()
		return nil, imgx.NewGray(frame.W, frame.H), nil
	}
	if err := imgx.CheckSameSize(frame, t.prev); err != nil {
		return nil, nil, fmt.Errorf("keypoints: tracker: %w", err)
	}
	corners, err := Harris(t.prev, t.cfg.Harris)
	if err != nil {
		return nil, nil, err
	}
	all, err := MatchBlocks(t.prev, frame, corners, t.cfg.Block, t.cfg.Search)
	if err != nil {
		return nil, nil, err
	}
	moving := all[:0]
	for _, m := range all {
		if m.Cost <= t.cfg.MaxCost && m.Magnitude() >= t.cfg.MinMotion && m.Magnitude() > 0 {
			moving = append(moving, m)
		}
	}
	copy(t.prev.Pix, frame.Pix)
	return moving, MotionMask(frame.W, frame.H, moving, t.cfg.MaskRadius), nil
}

// Reset forgets the previous frame, e.g. before a new sequence.
func (t *Tracker) Reset() {
	t.prev = nil
}

// MotionMask draws a w×h mask (0 or 255) with a filled disc of the given
// radius at the current position (X+DX, Y+DY) of every match. It turns the
// sparse corner motion into a mask that the heatmap and metrics can use.
func MotionMask(w, h int, matches []Match, radius int) *imgx.Gray {
	mask := imgx.NewGray(w, h)
	r2 := radius * radius
	for _, m := range matches {
		cx, cy := m.X+m.DX, m.Y+m.DY
		for y := max(cy-radius, 0); y <= min(cy+radius, h-1); y++ {
			for x := max(cx-radius, 0); x <= min(cx+radius, w-1); x++ {
				if dx, dy := x-cx, y-cy; dx*dx+dy*dy <= r2 {
					mask.Pix[y*w+x] = 255
				}
			}
		}
	}
	return mask
}
