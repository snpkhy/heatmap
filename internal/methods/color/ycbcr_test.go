package color

import (
	"math"
	"testing"

	"heatmap/internal/imgx"
)

func TestNewYCbCrIsNeutralBlack(t *testing.T) {
	p := NewYCbCr(3, 2)
	if p.W != 3 || p.H != 2 || len(p.Y) != 6 {
		t.Fatalf("size %dx%d with %d pixels", p.W, p.H, len(p.Y))
	}
	for i := range p.Y {
		if p.Y[i] != 0 || p.Cb[i] != 128 || p.Cr[i] != 128 {
			t.Fatalf("pixel %d = (%v,%v,%v), want (0,128,128)", i, p.Y[i], p.Cb[i], p.Cr[i])
		}
	}
	rgb := YCbCrToRGB(p)
	for i := range rgb.R {
		if rgb.R[i] != 0 || rgb.G[i] != 0 || rgb.B[i] != 0 {
			t.Fatalf("neutral black converts to (%v,%v,%v)", rgb.R[i], rgb.G[i], rgb.B[i])
		}
	}
}

func TestYCbCrImageRoundTrip(t *testing.T) {
	src := imgx.NewRGB(4, 2)
	colors := [][3]float32{
		{0, 0, 0}, {255, 255, 255}, {255, 0, 0}, {0, 255, 0},
		{0, 0, 255}, {12, 200, 77}, {128, 128, 128}, {250, 5, 130},
	}
	for i, c := range colors {
		src.R[i], src.G[i], src.B[i] = c[0], c[1], c[2]
	}
	back := YCbCrToRGB(RGBToYCbCr(src))
	for i := range src.R {
		for _, d := range []float32{back.R[i] - src.R[i], back.G[i] - src.G[i], back.B[i] - src.B[i]} {
			if math.Abs(float64(d)) > 0.01 {
				t.Fatalf("pixel %d: %v → (%v,%v,%v)", i, colors[i], back.R[i], back.G[i], back.B[i])
			}
		}
	}
}

func TestYCbCrLumaMatchesRGBLumaAndSharesMemory(t *testing.T) {
	src := imgx.NewRGB(2, 2)
	for i := range src.R {
		src.R[i], src.G[i], src.B[i] = float32(10*i), float32(200-30*i), float32(60+40*i)
	}
	p := RGBToYCbCr(src)
	y := p.Luma()
	want := src.Luma()
	for i := range want.Pix {
		if math.Abs(float64(y.Pix[i]-want.Pix[i])) > 1e-3 {
			t.Errorf("Y[%d] = %v, RGB.Luma = %v", i, y.Pix[i], want.Pix[i])
		}
	}
	y.Pix[0] = 42 // p2 enhances the Y plane in place through Luma()
	if p.Y[0] != 42 {
		t.Error("Luma must share memory with the Y plane")
	}
}

func TestYCbCrToRGBClamps(t *testing.T) {
	p := NewYCbCr(1, 1)
	p.Y[0], p.Cr[0] = 250, 255 // R = 250 + 1.402·127 is far above 255
	rgb := YCbCrToRGB(p)
	if rgb.R[0] != 255 || rgb.G[0] < 0 || rgb.B[0] > 255 {
		t.Errorf("out-of-gamut pixel → (%v,%v,%v), want clamped to [0,255]", rgb.R[0], rgb.G[0], rgb.B[0])
	}
}
