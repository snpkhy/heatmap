package methods_test

import (
	"testing"

	"heatmap/internal/imgx"
	"heatmap/internal/methods/filtering"
	"heatmap/internal/methods/segmentation"
	"heatmap/internal/methods/texture"
)

// TestP4ShadowIsNotMotion runs the three stages of pipeline p4 in order
// (box filter → block-wise LBP → block threshold) on a synthetic scene:
// a textured background, then the same scene with a shadow and an object.
// The object must end up in the mask and the shadow must not.
func TestP4ShadowIsNotMotion(t *testing.T) {
	const (
		w, h  = 128, 48
		block = 16
	)
	bg := imgx.NewGray(w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			bg.Set(x, y, float32(40+((x*7+y*13)%5)*50))
		}
	}
	frame := bg.Clone()
	for y := 0; y < h; y++ {
		for x := 0; x < 48; x++ {
			frame.Set(x, y, frame.At(x, y)*0.5) // shadow over 3×3 blocks
		}
	}
	for y := 16; y < 32; y++ {
		for x := 96; x < 112; x++ {
			frame.Set(x, y, 210) // object
		}
	}

	lbp, err := texture.NewBlockLBP(block, 0.05, 0.05)
	if err != nil {
		t.Fatal(err)
	}
	run := func(g *imgx.Gray) *imgx.Gray {
		t.Helper()
		smooth, err := filtering.Box(g, 3)
		if err != nil {
			t.Fatal(err)
		}
		change, err := lbp.Apply(smooth)
		if err != nil {
			t.Fatal(err)
		}
		mask, err := segmentation.BlockThreshold(change, block, 100)
		if err != nil {
			t.Fatal(err)
		}
		return mask
	}

	run(bg) // the first frame initialises the background
	mask := run(frame)

	if mask.At(104, 24) != 255 {
		t.Error("the object block must be in the mask")
	}
	for by := 0; by < h; by += block {
		for bx := 0; bx < 48; bx += block {
			if mask.At(bx, by) != 0 {
				t.Errorf("shadow block at (%d,%d) must not be in the mask", bx, by)
			}
		}
	}
	if mask.At(72, 8) != 0 || mask.At(120, 40) != 0 {
		t.Error("unchanged blocks must stay background")
	}
}
