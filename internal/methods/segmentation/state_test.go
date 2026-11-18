package segmentation

import (
	"errors"
	"testing"

	"heatmap/internal/imgx"
)

func TestRunningAvgBGAccessorsAndReset(t *testing.T) {
	m, err := NewRunningAvgBG(DefaultRunningAvgBGConfig())
	if err != nil {
		t.Fatal(err)
	}
	if m.Frames() != 0 || m.Background() != nil || m.Difference() != nil {
		t.Fatal("a fresh model must have no state")
	}
	bg := imgx.NewGray(8, 8)
	for i := range bg.Pix {
		bg.Pix[i] = 50
	}
	if _, err := m.Apply(bg); err != nil {
		t.Fatal(err)
	}
	frame := bg.Clone()
	frame.Set(3, 3, 250)
	if _, err := m.Apply(frame); err != nil {
		t.Fatal(err)
	}
	if m.Frames() != 2 {
		t.Errorf("Frames = %d, want 2", m.Frames())
	}
	d := m.Difference()
	if d.At(3, 3) != 200 || d.At(0, 0) != 0 {
		t.Errorf("difference at object %v, at background %v; want 200, 0", d.At(3, 3), d.At(0, 0))
	}
	b := m.Background()
	// The foreground pixel learns with FGAlpha = 0.005: 50 + 0.005·200 = 51.
	if v := b.At(3, 3); v < 50.99 || v > 51.01 {
		t.Errorf("background at object %v, want 51", v)
	}
	b.Pix[0], d.Pix[0] = -1, -1 // returned images are copies
	if m.Background().Pix[0] != 50 || m.Difference().Pix[0] != 0 {
		t.Error("Background and Difference must return clones")
	}

	if _, err := m.Apply(imgx.NewGray(4, 4)); !errors.Is(err, imgx.ErrSizeMismatch) {
		t.Errorf("size mismatch: err = %v", err)
	}
	if _, err := m.Apply(nil); err == nil {
		t.Error("nil frame: want error")
	}
	m.Reset()
	if m.Frames() != 0 || m.Background() != nil || m.Difference() != nil {
		t.Error("Reset must clear the state")
	}
	if _, err := m.Apply(imgx.NewGray(4, 4)); err != nil {
		t.Errorf("after Reset a new size is fine: %v", err)
	}
}

func TestRegionGrowingConfigValidation(t *testing.T) {
	bad := []RegionGrowingConfig{
		{SeedThreshold: 0, GrowThreshold: 1, Connectivity: 8},
		{SeedThreshold: 10, GrowThreshold: 0, Connectivity: 8},
		{SeedThreshold: 10, GrowThreshold: 11, Connectivity: 8},
		{SeedThreshold: 10, GrowThreshold: 5, Connectivity: 6},
	}
	for i, cfg := range bad {
		if _, err := NewFrameDiffRegionGrowing(cfg); err == nil {
			t.Errorf("case %d: NewFrameDiffRegionGrowing: want error", i)
		}
		if _, err := RegionGrowing(imgx.NewGray(2, 2), nil, cfg); err == nil {
			t.Errorf("case %d: RegionGrowing: want error", i)
		}
	}
}

func TestRegionGrowingInputErrors(t *testing.T) {
	cfg := DefaultRegionGrowingConfig()
	if _, err := RegionGrowing(nil, nil, cfg); err == nil {
		t.Error("nil diff: want error")
	}
	if _, err := RegionGrowing(imgx.NewGray(2, 2), imgx.NewGray(3, 2), cfg); !errors.Is(err, imgx.ErrSizeMismatch) {
		t.Errorf("size mismatch: err = %v", err)
	}
	if _, err := AbsDiff(nil, imgx.NewGray(1, 1)); err == nil {
		t.Error("AbsDiff nil: want error")
	}
	if _, err := AbsDiff(imgx.NewGray(1, 2), imgx.NewGray(2, 1)); !errors.Is(err, imgx.ErrSizeMismatch) {
		t.Errorf("AbsDiff size mismatch: err = %v", err)
	}
}

func TestRegionGrowingConnectivity(t *testing.T) {
	// Seed at (1,1); a grow-level pixel touches it only diagonally at (2,2).
	diff, _ := imgx.GrayFromSlice(4, 4, []float32{
		0, 0, 0, 0,
		0, 50, 0, 0,
		0, 0, 20, 0,
		0, 0, 0, 0,
	})
	cfg := RegionGrowingConfig{SeedThreshold: 30, GrowThreshold: 12, Connectivity: 4}
	m4, err := RegionGrowing(diff, nil, cfg)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Connectivity = 8
	m8, err := RegionGrowing(diff, nil, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if m4.At(1, 1) != MaskForeground || m4.At(2, 2) != MaskBackground {
		t.Error("4-connectivity must not grow across a corner")
	}
	if m8.At(2, 2) != MaskForeground {
		t.Error("8-connectivity must grow across a corner")
	}
}

func TestRegionGrowingIntensityConstraint(t *testing.T) {
	// Both neighbours of the seed changed enough, but only the left one has
	// an intensity close to the seed's in the current frame.
	diff, _ := imgx.GrayFromSlice(3, 1, []float32{20, 50, 20})
	frame, _ := imgx.GrayFromSlice(3, 1, []float32{105, 100, 200})
	cfg := RegionGrowingConfig{SeedThreshold: 30, GrowThreshold: 12, MaxIntensityDiff: 25, Connectivity: 4}
	mask, err := RegionGrowing(diff, frame, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if mask.At(0, 0) != MaskForeground || mask.At(2, 0) != MaskBackground {
		t.Errorf("mask %v, want [255 255 0]", mask.Pix)
	}
}

func TestFrameDiffRegionGrowingResetAndErrors(t *testing.T) {
	g, err := NewFrameDiffRegionGrowing(DefaultRegionGrowingConfig())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.Apply(nil); err == nil {
		t.Error("nil frame: want error")
	}
	if _, err := g.Apply(imgx.NewGray(4, 4)); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Apply(imgx.NewGray(5, 4)); !errors.Is(err, imgx.ErrSizeMismatch) {
		t.Errorf("size mismatch: err = %v", err)
	}
	g.Reset()
	mask, err := g.Apply(imgx.NewGray(5, 4))
	if err != nil {
		t.Fatalf("after Reset: %v", err)
	}
	for _, v := range mask.Pix {
		if v != MaskBackground {
			t.Fatal("first frame after Reset must give an empty mask")
		}
	}
}
