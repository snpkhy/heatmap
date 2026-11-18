package methods_test

import (
	"testing"

	"heatmap/internal/imgx"
	"heatmap/internal/methods/color"
	"heatmap/internal/methods/histogram"
	"heatmap/internal/methods/segmentation"
)

// TestP2EndToEnd runs the three stages of pipeline p2 in order:
// RGB→YCbCr (color) → CLAHE (histogram) → frame differencing + region growing (segmentation)
// on a synthetic low-light / night surveillance scene.
//
// In dark scenes, raw pixel differences are subtle and obscured by color noise.
// Pipeline p2 extracts the pure luma channel (Y), amplifies local contrast via CLAHE,
// and segments the moving object using seed-based region growing.
func TestP2EndToEnd(t *testing.T) {
	const (
		w, h = 64, 64
	)

	// Step 0: Create a dark night background (RGB values in [10, 25])
	bgRGB := imgx.NewRGB(w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			base := float32(12 + ((x*3 + y*5) % 10))
			bgRGB.Set(x, y, base, base*0.8, base*1.2) // subtle dark tint
		}
	}

	// Frame 2: Dark background + moving object with slight contrast difference (e.g. base + 15)
	f2RGB := bgRGB.Clone()
	for y := 20; y < 40; y++ {
		for x := 20; x < 40; x++ {
			f2RGB.Set(x, y, 35, 30, 42) // subtle dark moving object
		}
	}

	// Initialize pipeline p2 components
	claheCfg := histogram.CLAHEConfig{
		GridX:     4,
		GridY:     4,
		ClipLimit: 3.0,
	}
	regionCfg := segmentation.RegionGrowingConfig{
		SeedThreshold:    25.0,
		GrowThreshold:    10.0,
		MaxIntensityDiff: 0,
		Connectivity:     8,
	}

	segmenter, err := segmentation.NewFrameDiffRegionGrowing(regionCfg)
	if err != nil {
		t.Fatal(err)
	}

	runPipeline := func(frameRGB *imgx.RGB) *imgx.Gray {
		t.Helper()

		// Stage 1: RGB -> YCbCr (isolate luma Y)
		ycbcr := color.RGBToYCbCr(frameRGB)
		luma := ycbcr.Luma()

		// Stage 2: CLAHE (enhance local contrast in dark regions)
		enhanced, err := histogram.CLAHE(luma, claheCfg)
		if err != nil {
			t.Fatal(err)
		}

		// Stage 3: Frame Differencing + Region Growing
		mask, err := segmenter.Apply(enhanced)
		if err != nil {
			t.Fatal(err)
		}

		return mask
	}

	// Frame 1: First frame initializes reference
	m1 := runPipeline(bgRGB)
	for _, v := range m1.Pix {
		if v != 0 {
			t.Fatal("first frame mask must be all background")
		}
	}

	// Frame 2: Processes motion
	m2 := runPipeline(f2RGB)

	// The moving object should be cleanly segmented as foreground
	for y := 22; y < 38; y++ {
		for x := 22; x < 38; x++ {
			if m2.At(x, y) != segmentation.MaskForeground {
				t.Fatalf("expected foreground inside moving object at (%d, %d), got %f", x, y, m2.At(x, y))
			}
		}
	}

	// Background areas well outside the moving object must remain background
	if m2.At(5, 5) != segmentation.MaskBackground || m2.At(55, 55) != segmentation.MaskBackground {
		t.Errorf("static background misclassified as motion")
	}
}
