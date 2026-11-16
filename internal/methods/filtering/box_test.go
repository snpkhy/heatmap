package filtering

import (
	"math"
	"testing"

	"heatmap/internal/imgx"
)

func boxGray(t *testing.T, w, h int, pix []float32) *imgx.Gray {
	t.Helper()
	g, err := imgx.GrayFromSlice(w, h, pix)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestIntegralSum(t *testing.T) {
	// 1 2 3
	// 4 5 6
	// 7 8 9
	s := NewIntegral(boxGray(t, 3, 3, []float32{1, 2, 3, 4, 5, 6, 7, 8, 9}))
	tests := []struct {
		x0, y0, x1, y1 int
		want           float64
	}{
		{0, 0, 3, 3, 45},   // whole image
		{0, 0, 1, 1, 1},    // top-left pixel
		{1, 1, 3, 3, 28},   // 5+6+8+9
		{0, 1, 3, 2, 15},   // middle row
		{2, 0, 3, 3, 18},   // right column
		{-5, -5, 2, 2, 12}, // clipped: 1+2+4+5
		{1, 1, 9, 9, 28},   // clipped on the other side
		{2, 2, 2, 3, 0},    // empty
		{5, 5, 9, 9, 0},    // fully outside
	}
	for _, tt := range tests {
		if got := s.Sum(tt.x0, tt.y0, tt.x1, tt.y1); got != tt.want {
			t.Errorf("Sum(%d,%d,%d,%d) = %v, want %v", tt.x0, tt.y0, tt.x1, tt.y1, got, tt.want)
		}
	}
}

func TestBoxKnownValues(t *testing.T) {
	src := boxGray(t, 3, 3, []float32{1, 2, 3, 4, 5, 6, 7, 8, 9})
	got, err := Box(src, 3)
	if err != nil {
		t.Fatal(err)
	}
	// Centre: mean of 1..9 = 5. Corner (0,0): window clipped to 1 2 / 4 5 → 3.
	// Edge (1,0): 1 2 3 / 4 5 6 → 3.5.
	want := map[[2]int]float32{{1, 1}: 5, {0, 0}: 3, {1, 0}: 3.5, {2, 2}: 7}
	for p, w := range want {
		if v := got.At(p[0], p[1]); v != w {
			t.Errorf("(%d,%d) = %v, want %v", p[0], p[1], v, w)
		}
	}
}

func TestBoxKeepsFlatImage(t *testing.T) {
	src := imgx.NewGray(6, 5)
	for i := range src.Pix {
		src.Pix[i] = 42
	}
	got, err := Box(src, 5)
	if err != nil {
		t.Fatal(err)
	}
	for i, v := range got.Pix {
		if v != 42 {
			t.Fatalf("pixel %d = %v, want 42", i, v)
		}
	}
}

func TestBoxMatchesDirectMean(t *testing.T) {
	// The integral-image result must equal a plain k×k loop.
	src := imgx.NewGray(17, 11)
	for i := range src.Pix {
		src.Pix[i] = float32((i*37 + 11) % 256)
	}
	const k, r = 5, 2
	got, err := Box(src, k)
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < src.H; y++ {
		for x := 0; x < src.W; x++ {
			var sum float64
			n := 0
			for dy := -r; dy <= r; dy++ {
				for dx := -r; dx <= r; dx++ {
					if src.InBounds(x+dx, y+dy) {
						sum += float64(src.At(x+dx, y+dy))
						n++
					}
				}
			}
			if want := sum / float64(n); math.Abs(float64(got.At(x, y))-want) > 1e-3 {
				t.Fatalf("(%d,%d) = %v, want %v", x, y, got.At(x, y), want)
			}
		}
	}
}

func TestBoxWindowOneIsIdentity(t *testing.T) {
	src := boxGray(t, 2, 2, []float32{1, 2, 3, 4})
	got, err := Box(src, 1)
	if err != nil {
		t.Fatal(err)
	}
	for i := range src.Pix {
		if got.Pix[i] != src.Pix[i] {
			t.Fatalf("pixel %d = %v, want %v", i, got.Pix[i], src.Pix[i])
		}
	}
}

func TestBoxRejectsBadWindow(t *testing.T) {
	src := imgx.NewGray(2, 2)
	for _, k := range []int{0, -3, 2, 4} {
		if _, err := Box(src, k); err == nil {
			t.Errorf("k=%d: expected an error", k)
		}
	}
}

func benchmarkBox(b *testing.B, k int) {
	src := imgx.NewGray(320, 240)
	for i := range src.Pix {
		src.Pix[i] = float32(i % 256)
	}
	b.ResetTimer()
	for b.Loop() {
		if _, err := Box(src, k); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBox3x3(b *testing.B)   { benchmarkBox(b, 3) }
func BenchmarkBox15x15(b *testing.B) { benchmarkBox(b, 15) }
