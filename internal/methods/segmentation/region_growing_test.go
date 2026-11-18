package segmentation

import (
	"testing"

	"heatmap/internal/imgx"
)

func TestFrameDiff(t *testing.T) {
	a := imgx.NewGray(2, 2)
	b := imgx.NewGray(2, 2)

	a.Set(0, 0, 100)
	b.Set(0, 0, 60)

	diff, err := FrameDiff(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if diff.At(0, 0) != 40 {
		t.Errorf("got diff %v, want 40", diff.At(0, 0))
	}
}

func TestRegionGrowingExpandsFromSeed(t *testing.T) {
	w, h := 16, 16
	diff := imgx.NewGray(w, h)

	// High difference seed at (8, 8) = 50
	diff.Set(8, 8, 50)

	// Surrounding pixels have moderate difference (20), above GrowThreshold (12)
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			diff.Set(8+dx, 8+dy, 20)
		}
	}

	// Isolated weak noise at (2, 2) = 5 (below GrowThreshold)
	diff.Set(2, 2, 5)

	cfg := RegionGrowingConfig{
		SeedThreshold:    30.0,
		GrowThreshold:    12.0,
		MaxIntensityDiff: 0, // ignore intensity check
		Connectivity:     8,
	}

	mask, err := RegionGrowing(diff, nil, cfg)
	if err != nil {
		t.Fatal(err)
	}

	// The connected 5x5 region around (8, 8) must all be foreground
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			if mask.At(8+dx, 8+dy) != MaskForeground {
				t.Errorf("pixel (%d, %d) should be foreground", 8+dx, 8+dy)
			}
		}
	}

	// Pixels far away and weak noise must be background
	if mask.At(2, 2) != MaskBackground || mask.At(0, 0) != MaskBackground {
		t.Errorf("weak noise or background pixel misclassified as foreground")
	}
}

func TestFrameDiffRegionGrowingTwoFrames(t *testing.T) {
	w, h := 32, 32
	f1 := imgx.NewGray(w, h)
	f2 := f1.Clone()

	// Moving object in frame 2
	for y := 10; y < 20; y++ {
		for x := 10; x < 20; x++ {
			f2.Set(x, y, 100)
		}
	}

	grower, err := NewFrameDiffRegionGrowing(DefaultRegionGrowingConfig())
	if err != nil {
		t.Fatal(err)
	}

	m1, err := grower.Apply(f1)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range m1.Pix {
		if v != MaskBackground {
			t.Fatal("first frame mask must be background")
		}
	}

	m2, err := grower.Apply(f2)
	if err != nil {
		t.Fatal(err)
	}

	if m2.At(15, 15) != MaskForeground {
		t.Errorf("center of moving object at (15, 15) must be foreground")
	}
	if m2.At(2, 2) != MaskBackground {
		t.Errorf("background at (2, 2) must be background")
	}
}
