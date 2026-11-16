package edges

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

func filled(w, h int, v float32) *imgx.Gray {
	g := imgx.NewGray(w, h)
	for i := range g.Pix {
		g.Pix[i] = v
	}
	return g
}

func TestSobelMagnitudeFlatIsZero(t *testing.T) {
	g := SobelMagnitude(filled(4, 4, 77))
	for i, v := range g.Pix {
		if v != 0 {
			t.Fatalf("pixel %d = %v, want 0 on a flat image", i, v)
		}
	}
}

func TestSobelMagnitudeVerticalEdge(t *testing.T) {
	// Columns 0 0 100 100: at x=1 and x=2 the kernel sees a 0→100 step,
	// Gx = (1+2+1)·100 = 400, Gy = 0. Far from the edge the gradient is 0.
	src := mustGray(t, 4, 3, []float32{
		0, 0, 100, 100,
		0, 0, 100, 100,
		0, 0, 100, 100,
	})
	g := SobelMagnitude(src)
	want := []float32{0, 400, 400, 0}
	for y := 0; y < 3; y++ {
		for x, w := range want {
			if v := g.At(x, y); v != w {
				t.Errorf("(%d,%d) = %v, want %v", x, y, v, w)
			}
		}
	}
}

func TestSobelDiffFirstFrameIsEmpty(t *testing.T) {
	s, err := NewSobelDiff(50, 0.05)
	if err != nil {
		t.Fatal(err)
	}
	mask, err := s.Apply(filled(3, 3, 10))
	if err != nil {
		t.Fatal(err)
	}
	for i, v := range mask.Pix {
		if v != maskBackground {
			t.Fatalf("pixel %d = %v, want empty mask on the first frame", i, v)
		}
	}
}

func TestSobelDiffIgnoresBrightnessChange(t *testing.T) {
	// The whole frame gets 60 brighter: edges stay, so no motion.
	frame := mustGray(t, 4, 3, []float32{
		0, 0, 100, 100,
		0, 0, 100, 100,
		0, 0, 100, 100,
	})
	brighter := frame.Clone()
	for i := range brighter.Pix {
		brighter.Pix[i] += 60
	}
	s, err := NewSobelDiff(50, 0.05)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(frame); err != nil {
		t.Fatal(err)
	}
	mask, err := s.Apply(brighter)
	if err != nil {
		t.Fatal(err)
	}
	for i, v := range mask.Pix {
		if v != maskBackground {
			t.Fatalf("pixel %d = %v, a global brightness change must not be motion", i, v)
		}
	}
}

func TestSobelDiffDetectsNewObject(t *testing.T) {
	// A bright square appears in the middle of a flat 7x7 frame.
	bg := filled(7, 7, 20)
	obj := bg.Clone()
	for y := 2; y <= 4; y++ {
		for x := 2; x <= 4; x++ {
			obj.Set(x, y, 200)
		}
	}
	s, err := NewSobelDiff(50, 0.05)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(bg); err != nil {
		t.Fatal(err)
	}
	mask, err := s.Apply(obj)
	if err != nil {
		t.Fatal(err)
	}
	if mask.At(2, 2) != maskForeground {
		t.Error("the object's corner should be marked as motion")
	}
	if mask.At(0, 0) != maskBackground || mask.At(6, 6) != maskBackground {
		t.Error("far corners of the frame should stay background")
	}
}

func TestSobelDiffBackgroundAdapts(t *testing.T) {
	// With Alpha = 1 the background becomes the last frame, so a static
	// object is motion once and then disappears from the mask.
	bg := filled(5, 5, 0)
	obj := bg.Clone()
	obj.Set(2, 2, 255)
	s, err := NewSobelDiff(50, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []*imgx.Gray{bg, obj} {
		if _, err := s.Apply(f); err != nil {
			t.Fatal(err)
		}
	}
	mask, err := s.Apply(obj)
	if err != nil {
		t.Fatal(err)
	}
	for i, v := range mask.Pix {
		if v != maskBackground {
			t.Fatalf("pixel %d = %v, a static object must be absorbed into the background", i, v)
		}
	}
}

func TestSobelDiffRejectsSizeChange(t *testing.T) {
	s, err := NewSobelDiff(50, 0.05)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(imgx.NewGray(3, 3)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(imgx.NewGray(4, 3)); err == nil {
		t.Fatal("expected an error when the frame size changes")
	}
	s.Reset()
	if _, err := s.Apply(imgx.NewGray(4, 3)); err != nil {
		t.Fatalf("after Reset a new size must be accepted: %v", err)
	}
}

func TestNewSobelDiffRejectsBadParams(t *testing.T) {
	bad := []struct{ thr, alpha float32 }{
		{0, 0.1}, {-1, 0.1}, {50, 0}, {50, -0.5}, {50, 1.5},
	}
	for _, p := range bad {
		if _, err := NewSobelDiff(p.thr, p.alpha); err == nil {
			t.Errorf("threshold=%v alpha=%v: expected an error", p.thr, p.alpha)
		}
	}
}

func BenchmarkSobelMagnitude(b *testing.B) {
	src := imgx.NewGray(320, 240)
	for i := range src.Pix {
		src.Pix[i] = float32(i % 256)
	}
	b.ResetTimer()
	for b.Loop() {
		SobelMagnitude(src)
	}
}
