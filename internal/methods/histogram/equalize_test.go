package histogram

import (
	"testing"

	"heatmap/internal/imgx"
)

func gray(t *testing.T, w, h int, pix ...float32) *imgx.Gray {
	t.Helper()
	g, err := imgx.GrayFromSlice(w, h, pix)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestHistogram(t *testing.T) {
	// 10.4 rounds to 10, 300 clamps to 255, −5 clamps to 0.
	h := Histogram(gray(t, 5, 1, 10, 10.4, 300, -5, 0))
	if h[10] != 2 || h[255] != 1 || h[0] != 2 {
		t.Errorf("h[10]=%d h[255]=%d h[0]=%d, want 2 1 2", h[10], h[255], h[0])
	}
}

func TestEqualizeHandComputed(t *testing.T) {
	// Levels 50, 100, 150 with counts 1, 2, 1 → cdf 1, 3, 4; cdfMin = 1, N = 4.
	//   50  → (1−1)/(4−1)·255 = 0
	//   100 → (3−1)/3·255     = 170
	//   150 → (4−1)/3·255     = 255
	got := Equalize(gray(t, 2, 2, 50, 100, 100, 150))
	want := []float32{0, 170, 170, 255}
	for i := range want {
		if got.Pix[i] != want[i] {
			t.Fatalf("Equalize = %v, want %v", got.Pix, want)
		}
	}
}

func TestEqualizeConstantImage(t *testing.T) {
	got := Equalize(gray(t, 2, 2, 77, 77, 77, 77))
	for _, v := range got.Pix {
		if v != 77 {
			t.Fatalf("constant image changed: %v", got.Pix)
		}
	}
}

func TestEqualizeStretchesToFullRange(t *testing.T) {
	// A low-contrast ramp 100..131 must end up spanning 0..255, in the same order.
	g := imgx.NewGray(32, 1)
	for x := range g.Pix {
		g.Pix[x] = float32(100 + x)
	}
	out := Equalize(g)
	if out.Pix[0] != 0 || out.Pix[31] != 255 {
		t.Errorf("range is [%v, %v], want [0, 255]", out.Pix[0], out.Pix[31])
	}
	for x := 1; x < len(out.Pix); x++ {
		if out.Pix[x] <= out.Pix[x-1] {
			t.Fatalf("order not preserved at %d: %v", x, out.Pix)
		}
	}
}

func TestEqualizeIgnoresGlobalGain(t *testing.T) {
	// Dimming the whole frame keeps the order of intensities, so the
	// equalized result is the same. This is what p7 relies on.
	a := imgx.NewGray(16, 16)
	b := imgx.NewGray(16, 16)
	for i := range a.Pix {
		a.Pix[i] = float32((i * 7) % 250)
		b.Pix[i] = a.Pix[i] * 0.6
	}
	ea, eb := Equalize(a), Equalize(b)
	for i := range ea.Pix {
		// Rounding 0.6·v to a byte may merge two neighbouring levels.
		if d := ea.Pix[i] - eb.Pix[i]; d > 2 || d < -2 {
			t.Fatalf("pixel %d: %v vs %v after dimming", i, ea.Pix[i], eb.Pix[i])
		}
	}
}
