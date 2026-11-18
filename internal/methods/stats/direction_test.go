package stats

import (
	"math"
	"testing"
)

func TestAngleScreenConvention(t *testing.T) {
	// Image y grows downwards: (0, −1) points up on screen.
	cases := []struct{ dx, dy, want float64 }{
		{1, 0, 0}, {1, -1, 45}, {0, -1, 90}, {-1, 0, 180}, {0, 1, 270}, {1, 1, 315},
	}
	for _, c := range cases {
		if got := Angle(c.dx, c.dy); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("Angle(%v, %v) = %v, want %v", c.dx, c.dy, got, c.want)
		}
	}
}

func TestDirectionHistogramBins(t *testing.T) {
	h, err := NewDirectionHistogram(8, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	// Bin 0 covers [−22.5°, 22.5°), bin 1 [22.5°, 67.5°), …
	for angle, want := range map[float64]int{0: 0, 22.4: 0, 22.5: 1, 337.5: 0, 337.4: 7, 90: 2, 180: 4, 359.9: 0} {
		if got := h.Bin(angle); got != want {
			t.Errorf("Bin(%v) = %d, want %d", angle, got, want)
		}
	}
	if c := h.BinCenter(3); c != 135 {
		t.Errorf("BinCenter(3) = %v, want 135", c)
	}
}

func TestDirectionHistogramCounts(t *testing.T) {
	h, err := NewDirectionHistogram(4, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	h.Add(3, 0)      // right
	h.Add(5, 1)      // right
	h.Add(0, -2)     // up
	h.Add(-4, 0)     // left
	h.Add(0.5, 0)    // too short
	if h.Add(0, 0) { // zero vector
		t.Error("zero vector must not be counted")
	}
	want := []float64{2, 1, 1, 0}
	for i, w := range want {
		if h.Bins[i] != w {
			t.Errorf("bin %d = %v, want %v", i, h.Bins[i], w)
		}
	}
	if a, ok := h.Dominant(); !ok || a != 0 {
		t.Errorf("Dominant = %v, %v, want 0, true", a, ok)
	}
	norm := h.Normalized()
	if norm[0] != 0.5 || norm[1] != 0.25 || h.Total() != 4 {
		t.Errorf("normalized %v, total %v", norm, h.Total())
	}
	h.Reset()
	if _, ok := h.Dominant(); ok || h.Total() != 0 {
		t.Error("Reset must empty the histogram")
	}
	if n := h.Normalized(); n[0] != 0 {
		t.Error("empty histogram must normalize to zeros")
	}
}

func TestDirectionHistogramWeighted(t *testing.T) {
	h, err := NewDirectionHistogram(4, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	h.Add(1, 0)
	h.Add(1, 0)
	h.Add(0, 6) // one fast vector down outweighs two slow ones right
	if h.Bins[3] != 6 || h.Bins[0] != 2 {
		t.Errorf("bins %v, want [2 0 0 6]", h.Bins)
	}
	if a, _ := h.Dominant(); a != 270 {
		t.Errorf("Dominant = %v, want 270", a)
	}
}

func TestNewDirectionHistogramErrors(t *testing.T) {
	if _, err := NewDirectionHistogram(1, 0, false); err == nil {
		t.Error("1 bin: want error")
	}
	if _, err := NewDirectionHistogram(8, -1, false); err == nil {
		t.Error("negative magnitude: want error")
	}
}
