package color

import (
	"math"
	"testing"

	"heatmap/internal/imgx"
)

func near(a, b, tol float32) bool { return math.Abs(float64(a-b)) <= float64(tol) }

func TestPixelToHSVKnownColors(t *testing.T) {
	tests := []struct {
		name    string
		r, g, b float32
		h, s, v float32
	}{
		{"black", 0, 0, 0, 0, 0, 0},
		{"white", 255, 255, 255, 0, 0, 255},
		{"gray", 100, 100, 100, 0, 0, 100},
		{"red", 255, 0, 0, 0, 1, 255},
		{"green", 0, 255, 0, 120, 1, 255},
		{"blue", 0, 0, 255, 240, 1, 255},
		{"yellow", 255, 255, 0, 60, 1, 255},
		{"cyan", 0, 255, 255, 180, 1, 255},
		{"magenta", 255, 0, 255, 300, 1, 255},
		// max = R = 200, min = 50, d = 150: H = 60·(100−50)/150 = 20°, S = 150/200
		{"orange-ish", 200, 100, 50, 20, 0.75, 200},
		// max = R, G < B gives a negative angle that wraps: 60·(0−128)/255 + 360
		{"pink wraps", 255, 0, 128, 360 - 60*128.0/255, 1, 255},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, s, v := PixelToHSV(tt.r, tt.g, tt.b)
			if !near(h, tt.h, 1e-3) || !near(s, tt.s, 1e-5) || !near(v, tt.v, 1e-5) {
				t.Errorf("PixelToHSV(%v, %v, %v) = (%v, %v, %v), want (%v, %v, %v)",
					tt.r, tt.g, tt.b, h, s, v, tt.h, tt.s, tt.v)
			}
		})
	}
}

func TestHSVRoundTrip(t *testing.T) {
	// Every combination of a few levels per channel must survive RGB→HSV→RGB.
	levels := []float32{0, 1, 37, 128, 200, 255}
	for _, r := range levels {
		for _, g := range levels {
			for _, b := range levels {
				h, s, v := PixelToHSV(r, g, b)
				if h < 0 || h >= 360 || s < 0 || s > 1 {
					t.Fatalf("PixelToHSV(%v, %v, %v) out of range: h=%v s=%v", r, g, b, h, s)
				}
				r2, g2, b2 := PixelToRGB(h, s, v)
				if !near(r, r2, 1e-2) || !near(g, g2, 1e-2) || !near(b, b2, 1e-2) {
					t.Fatalf("round trip (%v, %v, %v) → (%v, %v, %v)", r, g, b, r2, g2, b2)
				}
			}
		}
	}
}

func TestPixelToRGBHueOutsideRange(t *testing.T) {
	// 480° and −240° are the same angle as 120° (green).
	for _, h := range []float32{120, 480, -240} {
		r, g, b := PixelToRGB(h, 1, 255)
		if !near(r, 0, 1e-3) || !near(g, 255, 1e-3) || !near(b, 0, 1e-3) {
			t.Errorf("PixelToRGB(%v, 1, 255) = (%v, %v, %v), want green", h, r, g, b)
		}
	}
}

func TestImageConversions(t *testing.T) {
	c := imgx.NewRGB(2, 1)
	c.Set(0, 0, 255, 0, 0)
	c.Set(1, 0, 10, 20, 30)

	p := RGBToHSV(c)
	if p.W != 2 || p.H != 1 {
		t.Fatalf("HSV size %dx%d, want 2x1", p.W, p.H)
	}
	if v := p.Value(); v.At(0, 0) != 255 || v.At(1, 0) != 30 {
		t.Errorf("V plane = %v, want [255 30]", v.Pix)
	}
	back := HSVToRGB(p)
	for i := range c.R {
		if !near(c.R[i], back.R[i], 1e-2) || !near(c.G[i], back.G[i], 1e-2) || !near(c.B[i], back.B[i], 1e-2) {
			t.Errorf("pixel %d did not survive the round trip", i)
		}
	}
}

func TestChromaticity(t *testing.T) {
	c := imgx.NewRGB(4, 1)
	c.Set(0, 0, 100, 50, 50) // sum 200 → r = 0.5, g = b = 0.25
	c.Set(1, 0, 50, 25, 25)  // the same color in shadow, half as bright
	c.Set(2, 0, 2, 3, 1)     // too dark: sum 6 < 30
	c.Set(3, 0, 0, 0, 0)     // black

	out := Chromaticity(c, DefaultMinSum)

	if r, g, b := out.At(0, 0); !near(r, 127.5, 1e-3) || !near(g, 63.75, 1e-3) || !near(b, 63.75, 1e-3) {
		t.Errorf("chromaticity = (%v, %v, %v), want (127.5, 63.75, 63.75)", r, g, b)
	}
	r0, g0, b0 := out.At(0, 0)
	r1, g1, b1 := out.At(1, 0)
	if !near(r0, r1, 1e-3) || !near(g0, g1, 1e-3) || !near(b0, b1, 1e-3) {
		t.Errorf("a shadowed pixel changed chromaticity: (%v, %v, %v) vs (%v, %v, %v)", r0, g0, b0, r1, g1, b1)
	}
	for x := 2; x < 4; x++ {
		if r, g, b := out.At(x, 0); !near(r, 85, 1e-3) || !near(g, 85, 1e-3) || !near(b, 85, 1e-3) {
			t.Errorf("dark pixel %d = (%v, %v, %v), want neutral 85", x, r, g, b)
		}
	}
}

func TestChromaValue(t *testing.T) {
	c := imgx.NewRGB(2, 1)
	c.Set(0, 0, 100, 50, 50)
	c.Set(1, 0, 50, 25, 25) // shadow: same chromaticity, half the value

	f := ChromaValue(c, DefaultMinSum)

	if !near(f.R[0], f.R[1], 1e-3) || !near(f.G[0], f.G[1], 1e-3) {
		t.Errorf("chromaticity planes differ under a shadow: R %v, G %v", f.R, f.G)
	}
	if f.B[0] != 100 || f.B[1] != 50 {
		t.Errorf("value plane = %v, want [100 50]", f.B)
	}
}
