package methods_test

import (
	"testing"

	"heatmap/internal/imgx"
	"heatmap/internal/methods/filtering"
	"heatmap/internal/methods/morphology"
	"heatmap/internal/methods/segmentation"
)

// TestP1EndToEnd runs the three stages of pipeline p1 in order:
// Gaussian filter (filtering) → running-average + Otsu (segmentation) → opening/closing (morphology)
// on a synthetic video sequence:
// A textured background with sensor noise, followed by a moving object containing a hole.
// The moving object must be detected cleanly, isolated noise removed, and the internal hole closed.
func TestP1EndToEnd(t *testing.T) {
	const (
		w, h = 128, 96
	)

	// Base background scene
	bg := imgx.NewGray(w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			bg.Set(x, y, float32(60+((x*3+y*7)%30)))
		}
	}

	// Model setup
	bgModel, err := segmentation.NewRunningAvgBG(segmentation.RunningAvgBGConfig{
		Alpha:        0.05,
		FGAlpha:      0.005,
		MinThreshold: 15.0,
	})
	if err != nil {
		t.Fatal(err)
	}

	runPipeline := func(frame *imgx.Gray) *imgx.Gray {
		t.Helper()

		// Stage 1: Gaussian filter
		blurred, err := filtering.Gaussian(frame, 3, 1.0)
		if err != nil {
			t.Fatal(err)
		}

		// Stage 2: Running-average background subtraction + Otsu threshold
		rawMask, err := bgModel.Apply(blurred)
		if err != nil {
			t.Fatal(err)
		}

		// Stage 3: Morphology (Opening to eliminate isolated noise, Closing to fill holes)
		opened, err := morphology.Opening(rawMask, 3)
		if err != nil {
			t.Fatal(err)
		}
		closed, err := morphology.Closing(opened, 3)
		if err != nil {
			t.Fatal(err)
		}

		return closed
	}

	// Frame 1: Learn background
	m1 := runPipeline(bg)
	for _, v := range m1.Pix {
		if v != 0 {
			t.Fatal("first frame mask must be empty")
		}
	}

	// Frame 2: Background + isolated noise dots + a moving bright object (with a 1px hole in its middle)
	f2 := bg.Clone()

	// 1-pixel noise spikes (must be eliminated by Gaussian + Opening)
	f2.Set(10, 10, 240)
	f2.Set(100, 80, 240)

	// Moving object at [40..60, 40..60]
	for y := 40; y < 60; y++ {
		for x := 40; x < 60; x++ {
			f2.Set(x, y, 210)
		}
	}
	// 1-pixel hole inside the object (must be closed by Closing)
	f2.Set(50, 50, bg.At(50, 50))

	mask := runPipeline(f2)

	// Verify isolated noise pixels are NOT in mask
	if mask.At(10, 10) != 0 {
		t.Errorf("isolated noise at (10, 10) should have been filtered out")
	}
	if mask.At(100, 80) != 0 {
		t.Errorf("isolated noise at (100, 80) should have been filtered out")
	}

	// Verify the moving object is solid foreground, including the closed hole at (50, 50)
	for y := 42; y < 58; y++ {
		for x := 42; x < 58; x++ {
			if mask.At(x, y) != 255 {
				t.Errorf("expected foreground at (%d, %d), got %f", x, y, mask.At(x, y))
			}
		}
	}

	// Verify background remains background
	if mask.At(5, 5) != 0 || mask.At(115, 85) != 0 {
		t.Error("unchanged background misclassified as motion")
	}
}
