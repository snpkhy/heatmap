package filtering

import (
	"math"
	"testing"

	"heatmap/internal/imgx"
)

func TestGaussianConstantImage(t *testing.T) {
	// A constant image must remain unchanged under Gaussian blur.
	const val = float32(100)
	w, h := 16, 16
	src := imgx.NewGray(w, h)
	for i := range src.Pix {
		src.Pix[i] = val
	}

	got, err := Gaussian(src, 5, 1.0)
	if err != nil {
		t.Fatal(err)
	}

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if math.Abs(float64(got.At(x, y)-val)) > 1e-4 {
				t.Fatalf("at (%d, %d): got %f, want %f", x, y, got.At(x, y), val)
			}
		}
	}
}

func TestGaussianSymmetry(t *testing.T) {
	// A single impulse at the center of an odd-sized image must produce a symmetric response.
	w, h := 15, 15
	cx, cy := 7, 7
	src := imgx.NewGray(w, h)
	src.Set(cx, cy, 255)

	got, err := Gaussian(src, 5, 1.2)
	if err != nil {
		t.Fatal(err)
	}

	// Check 8-way symmetry around the center
	for d := 1; d <= 2; d++ {
		c := got.At(cx+d, cy)
		if math.Abs(float64(got.At(cx-d, cy)-c)) > 1e-4 ||
			math.Abs(float64(got.At(cx, cy+d)-c)) > 1e-4 ||
			math.Abs(float64(got.At(cx, cy-d)-c)) > 1e-4 {
			t.Errorf("cardinal symmetry violated at distance %d", d)
		}

		diag := got.At(cx+d, cy+d)
		if math.Abs(float64(got.At(cx-d, cy-d)-diag)) > 1e-4 ||
			math.Abs(float64(got.At(cx+d, cy-d)-diag)) > 1e-4 ||
			math.Abs(float64(got.At(cx-d, cy+d)-diag)) > 1e-4 {
			t.Errorf("diagonal symmetry violated at distance %d", d)
		}
	}

	// The center must remain the maximum
	if got.At(cx, cy) <= got.At(cx+1, cy) {
		t.Errorf("center %f must be strictly greater than neighbors %f", got.At(cx, cy), got.At(cx+1, cy))
	}
}

func TestGaussianK1(t *testing.T) {
	src := imgx.NewGray(5, 5)
	src.Set(2, 2, 42)
	got, err := Gaussian(src, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.At(2, 2) != 42 {
		t.Errorf("got %v, want 42", got.At(2, 2))
	}
}

func TestGaussianRejectsBadWindow(t *testing.T) {
	src := imgx.NewGray(4, 4)
	for _, k := range []int{0, -1, 2, 4} {
		if _, err := Gaussian(src, k, 1.0); err == nil {
			t.Errorf("k=%d: expected an error", k)
		}
	}
}

func BenchmarkGaussian5x5(b *testing.B) {
	src := imgx.NewGray(320, 240)
	for i := range src.Pix {
		src.Pix[i] = float32(i % 256)
	}
	b.ResetTimer()
	for b.Loop() {
		if _, err := Gaussian(src, 5, 1.0); err != nil {
			b.Fatal(err)
		}
	}
}
