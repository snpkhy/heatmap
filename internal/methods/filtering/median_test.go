package filtering

import (
	"testing"

	"heatmap/internal/imgx"
)

func mustGray(t *testing.T, w, h int, pix []float32) *imgx.Gray {
	t.Helper()
	g, err := imgx.GrayFromSlice(w, h, pix)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestMedianRemovesImpulse(t *testing.T) {
	// A single white pixel (salt) on a flat background disappears.
	src := mustGray(t, 3, 3, []float32{
		10, 10, 10,
		10, 255, 10,
		10, 10, 10,
	})
	got, err := Median(src, 3)
	if err != nil {
		t.Fatal(err)
	}
	for i, v := range got.Pix {
		if v != 10 {
			t.Fatalf("pixel %d = %v, want 10", i, v)
		}
	}
}

func TestMedianKeepsStepEdge(t *testing.T) {
	// A vertical edge between 0 and 100 must stay exactly where it is.
	src := mustGray(t, 4, 3, []float32{
		0, 0, 100, 100,
		0, 0, 100, 100,
		0, 0, 100, 100,
	})
	got, err := Median(src, 3)
	if err != nil {
		t.Fatal(err)
	}
	for i, v := range got.Pix {
		if v != src.Pix[i] {
			t.Fatalf("pixel %d = %v, want %v", i, v, src.Pix[i])
		}
	}
}

func TestMedianKnownValue(t *testing.T) {
	// Centre window 1..9 → median 5. Corner (0,0) with border replication:
	// 1 1 2 / 1 1 2 / 4 4 5 → sorted 1 1 1 1 2 2 4 4 5 → median 2.
	src := mustGray(t, 3, 3, []float32{
		1, 2, 3,
		4, 5, 6,
		7, 8, 9,
	})
	got, err := Median(src, 3)
	if err != nil {
		t.Fatal(err)
	}
	if v := got.At(1, 1); v != 5 {
		t.Errorf("centre = %v, want 5", v)
	}
	if v := got.At(0, 0); v != 2 {
		t.Errorf("corner = %v, want 2", v)
	}
}

func TestMedianDoesNotModifySource(t *testing.T) {
	src := mustGray(t, 3, 1, []float32{0, 255, 0})
	if _, err := Median(src, 3); err != nil {
		t.Fatal(err)
	}
	if src.Pix[1] != 255 {
		t.Fatal("Median modified its input")
	}
}

func TestMedianWindowOneIsIdentity(t *testing.T) {
	src := mustGray(t, 2, 2, []float32{1, 2, 3, 4})
	got, err := Median(src, 1)
	if err != nil {
		t.Fatal(err)
	}
	for i := range src.Pix {
		if got.Pix[i] != src.Pix[i] {
			t.Fatalf("pixel %d = %v, want %v", i, got.Pix[i], src.Pix[i])
		}
	}
}

func TestMedianRejectsBadWindow(t *testing.T) {
	src := imgx.NewGray(2, 2)
	for _, k := range []int{0, -3, 2, 4} {
		if _, err := Median(src, k); err == nil {
			t.Errorf("k=%d: expected an error", k)
		}
	}
}

func BenchmarkMedian3x3(b *testing.B) {
	src := imgx.NewGray(320, 240)
	for i := range src.Pix {
		src.Pix[i] = float32(i % 256)
	}
	b.ResetTimer()
	for b.Loop() {
		if _, err := Median(src, 3); err != nil {
			b.Fatal(err)
		}
	}
}
