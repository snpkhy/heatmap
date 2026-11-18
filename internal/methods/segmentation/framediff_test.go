package segmentation

import (
	"errors"
	"math"
	"testing"

	"heatmap/internal/imgx"
)

func TestFrameDiffMarksChangedPixels(t *testing.T) {
	d, err := NewFrameDiff(20)
	if err != nil {
		t.Fatal(err)
	}
	first, _ := imgx.GrayFromSlice(4, 1, []float32{100, 100, 100, 100})
	second, _ := imgx.GrayFromSlice(4, 1, []float32{100, 120, 79, 200})
	third, _ := imgx.GrayFromSlice(4, 1, []float32{100, 120, 79, 200})

	mask, err := d.Apply(first)
	if err != nil {
		t.Fatal(err)
	}
	for i, v := range mask.Pix {
		if v != 0 {
			t.Fatalf("first frame: pixel %d = %v, want empty mask", i, v)
		}
	}

	mask, err = d.Apply(second)
	if err != nil {
		t.Fatal(err)
	}
	// +20 is not above the threshold, −21 and +100 are.
	want := []float32{0, 0, 255, 255}
	for i, w := range want {
		if mask.Pix[i] != w {
			t.Errorf("second frame: pixel %d = %v, want %v", i, mask.Pix[i], w)
		}
	}

	// Compared with the previous frame, not the first one: nothing changed.
	mask, err = d.Apply(third)
	if err != nil {
		t.Fatal(err)
	}
	for i, v := range mask.Pix {
		if v != 0 {
			t.Errorf("third frame: pixel %d = %v, want 0", i, v)
		}
	}
}

func TestFrameDiffDoesNotKeepCallerFrame(t *testing.T) {
	d, _ := NewFrameDiff(10)
	frame, _ := imgx.GrayFromSlice(2, 1, []float32{0, 0})
	if _, err := d.Apply(frame); err != nil {
		t.Fatal(err)
	}
	frame.Pix[0] = 50 // reusing the buffer must not change the stored frame
	mask, err := d.Apply(frame)
	if err != nil {
		t.Fatal(err)
	}
	if mask.Pix[0] != 255 || mask.Pix[1] != 0 {
		t.Errorf("mask %v, want [255 0]", mask.Pix)
	}
}

func TestFrameDiffSizeMismatchAndReset(t *testing.T) {
	d, _ := NewFrameDiff(10)
	if _, err := d.Apply(imgx.NewGray(4, 4)); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Apply(imgx.NewGray(3, 4)); !errors.Is(err, imgx.ErrSizeMismatch) {
		t.Fatalf("err = %v, want ErrSizeMismatch", err)
	}
	d.Reset()
	if _, err := d.Apply(imgx.NewGray(3, 4)); err != nil {
		t.Errorf("after Reset: %v", err)
	}
}

func TestNewFrameDiffRejectsBadThreshold(t *testing.T) {
	nan := float32(math.NaN())
	inf := float32(math.Inf(1))
	for _, th := range []float32{0, -5, nan, inf} {
		if _, err := NewFrameDiff(th); err == nil {
			t.Errorf("threshold %v: want error", th)
		}
	}
}
