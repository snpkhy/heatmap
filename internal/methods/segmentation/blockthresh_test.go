package segmentation

import (
	"testing"

	"heatmap/internal/imgx"
)

func TestBlockThreshold(t *testing.T) {
	// 4×4 image, 2×2 blocks with means 10, 100 / 60, 50 and threshold 50.
	src, err := imgx.GrayFromSlice(4, 4, []float32{
		10, 10, 100, 100,
		10, 10, 100, 100,
		0, 240, 50, 50,
		0, 0, 50, 50,
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := BlockThreshold(src, 2, 50)
	if err != nil {
		t.Fatal(err)
	}
	want := []float32{
		0, 0, 255, 255,
		0, 0, 255, 255,
		255, 255, 0, 0, // mean 60: the whole block turns on, including dark pixels
		255, 255, 0, 0, // mean 50 is not above the threshold
	}
	for i, w := range want {
		if got.Pix[i] != w {
			t.Errorf("pixel %d = %v, want %v", i, got.Pix[i], w)
		}
	}
}

func TestBlockThresholdPartialBlocks(t *testing.T) {
	// 3×3 with block 2: the right column and bottom row form smaller blocks.
	src, err := imgx.GrayFromSlice(3, 3, []float32{
		0, 0, 200,
		0, 0, 200,
		0, 0, 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := BlockThreshold(src, 2, 100)
	if err != nil {
		t.Fatal(err)
	}
	want := []float32{
		0, 0, 255,
		0, 0, 255,
		0, 0, 0,
	}
	for i, w := range want {
		if got.Pix[i] != w {
			t.Errorf("pixel %d = %v, want %v", i, got.Pix[i], w)
		}
	}
}

func TestBlockThresholdIgnoresSingleNoisyPixel(t *testing.T) {
	src := imgx.NewGray(8, 8)
	src.Set(3, 3, 255) // block mean = 255/64 ≈ 4
	got, err := BlockThreshold(src, 8, 50)
	if err != nil {
		t.Fatal(err)
	}
	for i, v := range got.Pix {
		if v != 0 {
			t.Fatalf("pixel %d = %v, one noisy pixel must not turn the block on", i, v)
		}
	}
}

func TestBlockThresholdRejectsBadBlock(t *testing.T) {
	for _, block := range []int{0, -4} {
		if _, err := BlockThreshold(imgx.NewGray(2, 2), block, 50); err == nil {
			t.Errorf("block=%d: expected an error", block)
		}
	}
}
