package heatmap

import (
	"errors"
	"testing"

	"heatmap/internal/imgx"
)

func mask(w, h int, on ...int) *imgx.Gray {
	m := imgx.NewGray(w, h)
	for _, i := range on {
		m.Pix[i] = 255
	}
	return m
}

func TestAccumulatorCounts(t *testing.T) {
	a, err := NewAccumulator(1)
	if err != nil {
		t.Fatal(err)
	}
	if a.Raw() != nil || a.Normalized() != nil {
		t.Fatal("empty accumulator must return nil images")
	}
	for range 3 {
		if err := a.Add(mask(3, 1, 0)); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.Add(mask(3, 1, 0, 1)); err != nil {
		t.Fatal(err)
	}
	half := mask(3, 1)
	half.Pix[2] = 127 // below the mask threshold
	if err := a.Add(half); err != nil {
		t.Fatal(err)
	}
	raw := a.Raw()
	if raw.Pix[0] != 4 || raw.Pix[1] != 1 || raw.Pix[2] != 0 {
		t.Errorf("raw %v, want [4 1 0]", raw.Pix)
	}
	if a.Frames() != 5 || a.Max() != 4 {
		t.Errorf("frames %d max %v", a.Frames(), a.Max())
	}
	if w, h := a.Size(); w != 3 || h != 1 {
		t.Errorf("size %dx%d", w, h)
	}
	n := a.Normalized()
	if n.Pix[0] != 255 || n.Pix[1] != 255.0/4 || n.Pix[2] != 0 {
		t.Errorf("normalized %v", n.Pix)
	}
	raw.Pix[0] = -1
	if a.Raw().Pix[0] != 4 {
		t.Error("Raw must return a copy")
	}
}

func TestAccumulatorDecay(t *testing.T) {
	a, _ := NewAccumulator(0.5)
	_ = a.Add(mask(1, 1, 0))
	_ = a.Add(mask(1, 1))
	_ = a.Add(mask(1, 1))
	if v := a.Raw().Pix[0]; v != 0.25 {
		t.Errorf("after two quiet frames %v, want 0.25", v)
	}
}

func TestAccumulatorQuietAndErrors(t *testing.T) {
	a, _ := NewAccumulator(1)
	_ = a.Add(mask(2, 2))
	for _, v := range a.Normalized().Pix {
		if v != 0 {
			t.Fatal("quiet heatmap must normalize to 0")
		}
	}
	if err := a.Add(mask(3, 2)); !errors.Is(err, imgx.ErrSizeMismatch) {
		t.Errorf("err = %v, want ErrSizeMismatch", err)
	}
	a.Reset()
	if a.Frames() != 0 || a.Add(mask(3, 2)) != nil {
		t.Error("Reset must allow a new size")
	}
	for _, d := range []float32{0, -1, 1.5} {
		if _, err := NewAccumulator(d); err == nil {
			t.Errorf("decay %v: want error", d)
		}
	}
}

func TestColorizeEndpointsAndMonotonicBrightness(t *testing.T) {
	g := imgx.NewGray(256, 1)
	for i := range g.Pix {
		g.Pix[i] = float32(i)
	}
	c, err := Colorize(g, Inferno)
	if err != nil {
		t.Fatal(err)
	}
	if r, gg, b := c.At(0, 0); r != 0 || gg != 0 || b != 4 {
		t.Errorf("inferno(0) = %v %v %v", r, gg, b)
	}
	if r, gg, b := c.At(255, 0); r != 252 || gg != 255 || b != 164 {
		t.Errorf("inferno(255) = %v %v %v", r, gg, b)
	}
	prev := float32(-1)
	for x := range 256 {
		r, gg, b := c.At(x, 0)
		l := 0.299*r + 0.587*gg + 0.114*b
		if l < prev-0.5 {
			t.Fatalf("inferno brightness drops at %d: %v < %v", x, l, prev)
		}
		prev = l
	}
	if _, err := Colorize(g, Jet); err != nil {
		t.Error(err)
	}
	if _, err := Colorize(g, "rainbow"); err == nil {
		t.Error("unknown map: want error")
	}
}

func TestOverlay(t *testing.T) {
	frame := imgx.NewRGB(3, 1)
	for i := range frame.R {
		frame.R[i], frame.G[i], frame.B[i] = 100, 100, 100
	}
	heat, _ := imgx.GrayFromSlice(3, 1, []float32{0, 10, 255})
	out, err := Overlay(frame, heat, Inferno, 0.6, 20)
	if err != nil {
		t.Fatal(err)
	}
	if r, _, _ := out.At(0, 0); r != 100 {
		t.Error("quiet pixel must keep the frame")
	}
	if r, _, _ := out.At(1, 0); r != 100 {
		t.Error("pixel below minLevel must keep the frame")
	}
	// Full heat: 0.4·100 + 0.6·(252, 255, 164).
	near := func(a, b float32) bool { return a-b < 0.01 && b-a < 0.01 }
	if r, g, b := out.At(2, 0); !near(r, 0.4*100+0.6*252) || !near(g, 0.4*100+0.6*255) || !near(b, 0.4*100+0.6*164) {
		t.Errorf("hot pixel = %v %v %v", r, g, b)
	}
	if frame.R[2] != 100 {
		t.Error("the frame must not change")
	}
	if _, err := Overlay(frame, imgx.NewGray(2, 1), Inferno, 0.5, 0); !errors.Is(err, imgx.ErrSizeMismatch) {
		t.Errorf("err = %v", err)
	}
	if _, err := Overlay(frame, heat, Inferno, 2, 0); err == nil {
		t.Error("alpha 2: want error")
	}
	if _, err := Overlay(frame, heat, "x", 0.5, 0); err == nil {
		t.Error("unknown map: want error")
	}
}
