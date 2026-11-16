package edges

import (
	"math"
	"testing"

	"heatmap/internal/imgx"
)

func near(a, b float32) bool { return math.Abs(float64(a-b)) <= 1e-3 }

func TestPrewittFlatImage(t *testing.T) {
	g := imgx.NewGray(5, 4)
	for i := range g.Pix {
		g.Pix[i] = 123
	}
	for i, v := range Prewitt(g).Pix {
		if v != 0 {
			t.Fatalf("flat image has gradient %v at %d", v, i)
		}
	}
}

func TestPrewittVerticalStep(t *testing.T) {
	// Columns 0–2 are black, 3–5 white. Only the two columns next to the
	// step see it, and the ⅓ normalization makes the response exactly 255.
	g := imgx.NewGray(6, 4)
	for y := range g.H {
		for x := 3; x < g.W; x++ {
			g.Set(x, y, 255)
		}
	}
	gx, gy := PrewittXY(g)
	mag := Prewitt(g)
	for y := range g.H {
		for x := range g.W {
			want := float32(0)
			if x == 2 || x == 3 {
				want = 255
			}
			if !near(gx.At(x, y), want) || gy.At(x, y) != 0 || !near(mag.At(x, y), want) {
				t.Errorf("(%d,%d): gx=%v gy=%v mag=%v, want gx=mag=%v gy=0",
					x, y, gx.At(x, y), gy.At(x, y), mag.At(x, y), want)
			}
		}
	}
}

func TestPrewittHorizontalStepSign(t *testing.T) {
	// Bright on top, dark below: intensity falls with y, so Gy is negative.
	g := imgx.NewGray(4, 4)
	for x := range g.W {
		g.Set(x, 0, 90)
		g.Set(x, 1, 90)
	}
	gx, gy := PrewittXY(g)
	if gx.At(1, 1) != 0 || !near(gy.At(1, 1), -90) || !near(gy.At(1, 2), -90) {
		t.Errorf("gx=%v gy(1,1)=%v gy(1,2)=%v, want 0 −90 −90", gx.At(1, 1), gy.At(1, 1), gy.At(1, 2))
	}
}

func TestPrewittHandComputedPixel(t *testing.T) {
	// 1 2 3
	// 4 5 6     centre: Gx = ((3+6+9) − (1+4+7))/3 = 2
	// 7 8 9             Gy = ((7+8+9) − (1+2+3))/3 = 6
	g, err := imgx.GrayFromSlice(3, 3, []float32{1, 2, 3, 4, 5, 6, 7, 8, 9})
	if err != nil {
		t.Fatal(err)
	}
	gx, gy := PrewittXY(g)
	if !near(gx.At(1, 1), 2) || !near(gy.At(1, 1), 6) {
		t.Errorf("centre: gx=%v gy=%v, want 2 6", gx.At(1, 1), gy.At(1, 1))
	}
	if want := float32(math.Sqrt(40)); !near(Prewitt(g).At(1, 1), want) {
		t.Errorf("magnitude = %v, want %v", Prewitt(g).At(1, 1), want)
	}
}

func TestPrewittIgnoresBrightnessOffset(t *testing.T) {
	a := imgx.NewGray(8, 8)
	b := imgx.NewGray(8, 8)
	for i := range a.Pix {
		a.Pix[i] = float32((i * 13) % 97)
		b.Pix[i] = a.Pix[i] + 40
	}
	ma, mb := Prewitt(a), Prewitt(b)
	for i := range ma.Pix {
		if !near(ma.Pix[i], mb.Pix[i]) {
			t.Fatalf("pixel %d: %v vs %v after adding a constant", i, ma.Pix[i], mb.Pix[i])
		}
	}
}

func TestPrewittSinglePixelImage(t *testing.T) {
	g := imgx.NewGray(1, 1)
	g.Pix[0] = 200
	if v := Prewitt(g).Pix[0]; v != 0 {
		t.Errorf("1×1 image has gradient %v", v)
	}
}
