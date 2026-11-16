package texture

import (
	"errors"
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

// textured returns a w×h image with a repeating high-contrast pattern.
func textured(w, h int) *imgx.Gray {
	g := imgx.NewGray(w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			g.Set(x, y, float32(40+((x*7+y*13)%5)*50))
		}
	}
	return g
}

func mustDetector(t *testing.T, block int, margin, alpha float32) *BlockLBP {
	t.Helper()
	d, err := NewBlockLBP(block, margin, alpha)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func mustApply(t *testing.T, d *BlockLBP, g *imgx.Gray) *imgx.Gray {
	t.Helper()
	out, err := d.Apply(g)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestLBPHandComputed(t *testing.T) {
	// Neighbours of the centre (50), clockwise from top-left:
	// 90 10 60 / 50 80 20 / 70 30 → bits 1 0 1 1 1 0 1 0 → 1+4+8+16+64 = 93.
	src := mustGray(t, 3, 3, []float32{
		90, 10, 60,
		30, 50, 50,
		70, 20, 80,
	})
	if got := LBP(src, 0).At(1, 1); got != 93 {
		t.Errorf("code = %v, want 93", got)
	}
}

func TestLBPMargin(t *testing.T) {
	// Without a margin a flat area gives all ones. With a 5 % margin the
	// limit around 100 is 105, so small noise (±2) gives a stable code 0.
	flat := mustGray(t, 3, 3, []float32{100, 100, 100, 100, 100, 100, 100, 100, 100})
	if got := LBP(flat, 0).At(1, 1); got != 255 {
		t.Errorf("flat, margin 0: code = %v, want 255", got)
	}
	noisy := mustGray(t, 3, 3, []float32{102, 98, 101, 99, 100, 102, 98, 101, 99})
	if got := LBP(noisy, 0.05).At(1, 1); got != 0 {
		t.Errorf("noisy, margin 0.05: code = %v, want 0", got)
	}
}

func TestLBPIgnoresBrightnessScale(t *testing.T) {
	// A shadow multiplies brightness; the order of the pixels stays the same.
	src := textured(12, 9)
	shadow := src.Clone()
	for i := range shadow.Pix {
		shadow.Pix[i] *= 0.5
	}
	a, b := LBP(src, 0.05), LBP(shadow, 0.05)
	for i := range a.Pix {
		if a.Pix[i] != b.Pix[i] {
			t.Fatalf("pixel %d: code %v became %v under a shadow", i, a.Pix[i], b.Pix[i])
		}
	}
}

func TestBlockLBPFirstFrameIsZero(t *testing.T) {
	d := mustDetector(t, 8, 0.05, 0.05)
	for i, v := range mustApply(t, d, textured(16, 16)).Pix {
		if v != 0 {
			t.Fatalf("pixel %d = %v, want 0 on the first frame", i, v)
		}
	}
}

func TestBlockLBPShadowVersusObject(t *testing.T) {
	// 48×24 frame of 8×8 blocks. A shadow covers the left 24×24 pixels,
	// a flat object hides the texture of the block at (40, 8).
	bg := textured(48, 24)
	frame := bg.Clone()
	for y := 0; y < 24; y++ {
		for x := 0; x < 24; x++ {
			frame.Set(x, y, frame.At(x, y)*0.5)
		}
	}
	for y := 8; y < 16; y++ {
		for x := 40; x < 48; x++ {
			frame.Set(x, y, 200)
		}
	}
	d := mustDetector(t, 8, 0.05, 0.05)
	mustApply(t, d, bg)
	out := mustApply(t, d, frame)

	if v := out.At(8, 8); v != 0 {
		t.Errorf("block inside the shadow = %v, want 0", v)
	}
	// Along the shadow's edge only one column of pixels changes its code.
	if v := out.At(16, 8); v == 0 || v > 60 {
		t.Errorf("block on the shadow's edge = %v, want a small change", v)
	}
	if v := out.At(40, 8); v < 180 {
		t.Errorf("object block = %v, want a large change", v)
	}
	// A block is filled with one value.
	if out.At(40, 8) != out.At(47, 15) {
		t.Error("pixels of one block must share the same value")
	}
}

func TestBlockLBPBackgroundAdapts(t *testing.T) {
	// With Alpha = 1 the background becomes the last frame.
	bg := textured(8, 8)
	obj := imgx.NewGray(8, 8)
	d := mustDetector(t, 8, 0.05, 1)
	mustApply(t, d, bg)
	if v := mustApply(t, d, obj).At(0, 0); v == 0 {
		t.Fatal("a new object must change the map")
	}
	if v := mustApply(t, d, obj).At(0, 0); v != 0 {
		t.Fatalf("static object = %v, want 0 once absorbed into the background", v)
	}
}

func TestBlockLBPPartialBlocks(t *testing.T) {
	// 10×10 with block 8: the cut-off blocks are normalised by their own area.
	bg := textured(10, 10)
	d := mustDetector(t, 8, 0.05, 0.05)
	mustApply(t, d, bg)
	for i, v := range mustApply(t, d, bg).Pix {
		if v != 0 {
			t.Fatalf("pixel %d = %v, want 0 for an unchanged frame", i, v)
		}
	}
}

func TestBlockLBPRejectsSizeChange(t *testing.T) {
	d := mustDetector(t, 8, 0.05, 0.05)
	mustApply(t, d, imgx.NewGray(16, 16))
	if _, err := d.Apply(imgx.NewGray(16, 8)); !errors.Is(err, imgx.ErrSizeMismatch) {
		t.Fatalf("err = %v, want ErrSizeMismatch", err)
	}
	d.Reset()
	mustApply(t, d, imgx.NewGray(16, 8))
}

func TestNewBlockLBPRejectsBadParams(t *testing.T) {
	bad := []struct {
		block         int
		margin, alpha float32
	}{
		{1, 0.05, 0.1}, {0, 0.05, 0.1}, {8, -0.1, 0.1}, {8, 0.05, 0}, {8, 0.05, 1.5},
	}
	for _, p := range bad {
		if _, err := NewBlockLBP(p.block, p.margin, p.alpha); err == nil {
			t.Errorf("block=%d margin=%v alpha=%v: expected an error", p.block, p.margin, p.alpha)
		}
	}
}

func BenchmarkBlockLBP(b *testing.B) {
	src := textured(320, 240)
	d, err := NewBlockLBP(8, 0.05, 0.05)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for b.Loop() {
		if _, err := d.Apply(src); err != nil {
			b.Fatal(err)
		}
	}
}
