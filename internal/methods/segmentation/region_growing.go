package segmentation

import (
	"fmt"
	"math"

	"heatmap/internal/imgx"
)

// RegionGrowingConfig holds parameters for region growing segmentation from motion seeds.
type RegionGrowingConfig struct {
	// SeedThreshold is the high motion difference threshold used to locate definitive motion seeds.
	// Must be > 0. Typical value: 25.0 - 35.0.
	SeedThreshold float32

	// GrowThreshold is the lower motion difference threshold for expanding into connected neighbors.
	// Must be > 0 and <= SeedThreshold. Typical value: 8.0 - 15.0.
	GrowThreshold float32

	// MaxIntensityDiff restricts region expansion to neighbors whose intensity is close to the current pixel,
	// keeping the region from leaking into the background. If <= 0, intensity constraint is omitted.
	// Typical value: 20.0.
	MaxIntensityDiff float32

	// Connectivity specifies neighbor topology: 4 or 8. Default is 8.
	Connectivity int
}

// DefaultRegionGrowingConfig returns standard parameters for motion region growing.
func DefaultRegionGrowingConfig() RegionGrowingConfig {
	return RegionGrowingConfig{
		SeedThreshold:    30.0,
		GrowThreshold:    12.0,
		MaxIntensityDiff: 25.0,
		Connectivity:     8,
	}
}

func (c RegionGrowingConfig) validate() error {
	if c.SeedThreshold <= 0 {
		return fmt.Errorf("segmentation: SeedThreshold %v must be positive", c.SeedThreshold)
	}
	if c.GrowThreshold <= 0 || c.GrowThreshold > c.SeedThreshold {
		return fmt.Errorf("segmentation: GrowThreshold %v must be in (0, SeedThreshold %v]", c.GrowThreshold, c.SeedThreshold)
	}
	if c.Connectivity != 4 && c.Connectivity != 8 {
		return fmt.Errorf("segmentation: Connectivity must be 4 or 8, got %d", c.Connectivity)
	}
	return nil
}

// AbsDiff computes the absolute pixel-wise difference |curr - prev|.
func AbsDiff(curr, prev *imgx.Gray) (*imgx.Gray, error) {
	if curr == nil || prev == nil {
		return nil, fmt.Errorf("segmentation: curr and prev cannot be nil")
	}
	if err := imgx.CheckSameSize(curr, prev); err != nil {
		return nil, err
	}

	diff := imgx.NewGray(curr.W, curr.H)
	for i := range diff.Pix {
		diff.Pix[i] = float32(math.Abs(float64(curr.Pix[i] - prev.Pix[i])))
	}
	return diff, nil
}

// RegionGrowing segments motion by finding high-confidence motion seeds in diff,
// then expanding into neighboring pixels that meet both the lower motion threshold
// and intensity similarity.
func RegionGrowing(diff, frame *imgx.Gray, cfg RegionGrowingConfig) (*imgx.Gray, error) {
	if diff == nil {
		return nil, fmt.Errorf("segmentation: diff cannot be nil")
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	if frame != nil {
		if err := imgx.CheckSameSize(diff, frame); err != nil {
			return nil, err
		}
	}

	w, h := diff.W, diff.H
	mask := imgx.NewGray(w, h)

	// Queue for Breadth-First Search (BFS)
	type pt struct{ x, y int }
	queue := make([]pt, 0, w*h/16)

	// Step 1: Find initial high-confidence motion seeds
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if diff.At(x, y) >= cfg.SeedThreshold {
				mask.Set(x, y, MaskForeground)
				queue = append(queue, pt{x, y})
			}
		}
	}

	// Neighbor offsets
	dx4 := []int{0, 1, 0, -1}
	dy4 := []int{-1, 0, 1, 0}
	dx8 := []int{-1, 0, 1, -1, 1, -1, 0, 1}
	dy8 := []int{-1, -1, -1, 0, 0, 1, 1, 1}

	dx, dy := dx8, dy8
	if cfg.Connectivity == 4 {
		dx, dy = dx4, dy4
	}

	// Step 2: Flood-fill expand into connected neighbors
	for head := 0; head < len(queue); head++ {
		p := queue[head]
		curVal := float32(0)
		if frame != nil {
			curVal = frame.At(p.x, p.y)
		}

		for k := 0; k < len(dx); k++ {
			nx := p.x + dx[k]
			ny := p.y + dy[k]

			if !diff.InBounds(nx, ny) || mask.At(nx, ny) == MaskForeground {
				continue
			}

			// Must exceed lower motion threshold
			if diff.At(nx, ny) < cfg.GrowThreshold {
				continue
			}

			// If intensity similarity constraint is enabled, check neighbor intensity
			if frame != nil && cfg.MaxIntensityDiff > 0 {
				nVal := frame.At(nx, ny)
				if math.Abs(float64(nVal-curVal)) > float64(cfg.MaxIntensityDiff) {
					continue
				}
			}

			mask.Set(nx, ny, MaskForeground)
			queue = append(queue, pt{nx, ny})
		}
	}

	return mask, nil
}

// FrameDiffRegionGrowing tracks consecutive frames, calculates frame difference,
// and applies region growing segmentation.
type FrameDiffRegionGrowing struct {
	cfg  RegionGrowingConfig
	prev *imgx.Gray
}

// NewFrameDiffRegionGrowing creates a stateful frame-differencing region grower.
func NewFrameDiffRegionGrowing(cfg RegionGrowingConfig) (*FrameDiffRegionGrowing, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &FrameDiffRegionGrowing{cfg: cfg}, nil
}

// Reset clears the stored previous frame.
func (m *FrameDiffRegionGrowing) Reset() {
	m.prev = nil
}

// Apply processes the incoming frame.
// The first frame initializes the previous reference and returns an empty mask.
func (m *FrameDiffRegionGrowing) Apply(frame *imgx.Gray) (*imgx.Gray, error) {
	if frame == nil {
		return nil, fmt.Errorf("segmentation: frame cannot be nil")
	}

	if m.prev == nil {
		m.prev = frame.Clone()
		return imgx.NewGray(frame.W, frame.H), nil
	}

	diff, err := AbsDiff(frame, m.prev)
	if err != nil {
		return nil, err
	}

	mask, err := RegionGrowing(diff, frame, m.cfg)
	if err != nil {
		return nil, err
	}

	m.prev = frame.Clone()
	return mask, nil
}
