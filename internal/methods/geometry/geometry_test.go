package geometry

import (
	"errors"
	"math"
	"testing"

	"heatmap/internal/imgx"
)

func nearPt(a, b Point) bool {
	return math.Abs(a.X-b.X) < 1e-6 && math.Abs(a.Y-b.Y) < 1e-6
}

func mustApply(t *testing.T, h Homography, p Point) Point {
	t.Helper()
	q, ok := h.Apply(p)
	if !ok {
		t.Fatalf("%v maps to infinity", p)
	}
	return q
}

var unitSquare = [4]Point{{0, 0}, {1, 0}, {1, 1}, {0, 1}}

func TestHomographyFromPointsTranslationAndScale(t *testing.T) {
	dst := [4]Point{{10, 20}, {13, 20}, {13, 22}, {10, 22}} // ×3, ×2, then +(10, 20)
	h, err := HomographyFromPoints(unitSquare, dst)
	if err != nil {
		t.Fatal(err)
	}
	want := Homography{3, 0, 10, 0, 2, 20, 0, 0, 1}
	for i := range h {
		if math.Abs(h[i]-want[i]) > 1e-9 {
			t.Fatalf("h = %v, want %v", h, want)
		}
	}
	if q := mustApply(t, h, Point{0.5, 0.5}); !nearPt(q, Point{11.5, 21}) {
		t.Errorf("centre maps to %v, want (11.5, 21)", q)
	}
}

func TestHomographyFromPointsPerspective(t *testing.T) {
	// A trapezoid as a camera sees a rectangular floor: far edge shorter.
	src := [4]Point{{120, 40}, {200, 40}, {310, 230}, {10, 230}}
	dst := [4]Point{{0, 0}, {99, 0}, {99, 199}, {0, 199}}
	h, err := HomographyFromPoints(src, dst)
	if err != nil {
		t.Fatal(err)
	}
	for i := range 4 {
		if q := mustApply(t, h, src[i]); !nearPt(q, dst[i]) {
			t.Errorf("corner %d maps to %v, want %v", i, q, dst[i])
		}
	}
	// Straight lines stay straight: the midpoint of the near edge maps onto
	// the near edge of the plan.
	if q := mustApply(t, h, Point{160, 230}); math.Abs(q.Y-199) > 1e-6 {
		t.Errorf("near-edge midpoint maps to %v, want y = 199", q)
	}
	// It is not affine: the trapezoid's centre is not the plan's centre.
	if q := mustApply(t, h, Point{160, 135}); math.Abs(q.Y-99.5) < 5 {
		t.Errorf("perspective should move the centre, got %v", q)
	}
}

func TestHomographyInverseAndMul(t *testing.T) {
	src := [4]Point{{120, 40}, {200, 40}, {310, 230}, {10, 230}}
	dst := [4]Point{{0, 0}, {99, 0}, {99, 199}, {0, 199}}
	h, err := HomographyFromPoints(src, dst)
	if err != nil {
		t.Fatal(err)
	}
	inv, err := h.Inverse()
	if err != nil {
		t.Fatal(err)
	}
	for i := range 4 {
		if q := mustApply(t, inv, dst[i]); !nearPt(q, src[i]) {
			t.Errorf("inverse maps %v to %v, want %v", dst[i], q, src[i])
		}
	}
	id := h.Mul(inv)
	for i := range id {
		if math.Abs(id[i]/id[8]-Identity[i]) > 1e-9 {
			t.Fatalf("h·h⁻¹ = %v, want identity", id)
		}
	}
}

func TestHomographyDegenerate(t *testing.T) {
	collinear := [4]Point{{0, 0}, {1, 1}, {2, 2}, {0, 5}}
	if _, err := HomographyFromPoints(collinear, unitSquare); !errors.Is(err, ErrDegenerate) {
		t.Errorf("collinear source: err = %v, want ErrDegenerate", err)
	}
	if _, err := HomographyFromPoints(unitSquare, collinear); !errors.Is(err, ErrDegenerate) {
		t.Errorf("collinear target: err = %v, want ErrDegenerate", err)
	}
	if _, err := (Homography{}).Inverse(); !errors.Is(err, ErrDegenerate) {
		t.Errorf("zero matrix inverse: err = %v, want ErrDegenerate", err)
	}
}

func TestApplyAtInfinity(t *testing.T) {
	h := Homography{1, 0, 0, 0, 1, 0, 1, 0, 0} // w = x
	if _, ok := h.Apply(Point{0, 3}); ok {
		t.Error("point with w = 0 must not map")
	}
}

func ramp(w, h int) *imgx.Gray {
	g := imgx.NewGray(w, h)
	for i := range g.Pix {
		g.Pix[i] = float32(i)
	}
	return g
}

func TestWarpTranslation(t *testing.T) {
	src := ramp(5, 4)
	shift := Homography{1, 0, 2, 0, 1, 1, 0, 0, 1} // +2 right, +1 down
	for _, interp := range []Interp{Nearest, Bilinear} {
		got, err := Warp(src, shift, 5, 4, interp)
		if err != nil {
			t.Fatal(err)
		}
		for y := range 4 {
			for x := range 5 {
				want := float32(0) // uncovered
				if x >= 2 && y >= 1 {
					want = src.At(x-2, y-1)
				}
				if got.At(x, y) != want {
					t.Errorf("interp %d: (%d,%d) = %v, want %v", interp, x, y, got.At(x, y), want)
				}
			}
		}
	}
}

func TestWarpBilinearHalfPixel(t *testing.T) {
	src, _ := imgx.GrayFromSlice(2, 2, []float32{0, 100, 100, 200})
	// Output pixel (0, 0) reads source position (0.5, 0.5): the mean of all four.
	half := Homography{1, 0, -0.5, 0, 1, -0.5, 0, 0, 1}
	got, err := Warp(src, half, 1, 1, Bilinear)
	if err != nil {
		t.Fatal(err)
	}
	if got.At(0, 0) != 100 {
		t.Errorf("got %v, want 100", got.At(0, 0))
	}
	near, err := Warp(src, half, 1, 1, Nearest)
	if err != nil {
		t.Fatal(err)
	}
	if v := near.At(0, 0); v != 200 { // round(0.5) = 1 → pixel (1, 1)
		t.Errorf("nearest got %v, want 200", v)
	}
}

func TestWarpScaleUpKeepsMaskBinary(t *testing.T) {
	mask, _ := imgx.GrayFromSlice(2, 2, []float32{255, 0, 0, 255})
	up := Homography{3, 0, 1, 0, 3, 1, 0, 0, 1}
	got, err := Warp(mask, up, 6, 6, Nearest)
	if err != nil {
		t.Fatal(err)
	}
	var on int
	for _, v := range got.Pix {
		if v != 0 && v != 255 {
			t.Fatalf("value %v in a nearest-warped mask", v)
		}
		if v == 255 {
			on++
		}
	}
	if on != 18 { // two 3×3 blocks
		t.Errorf("%d foreground pixels, want 18", on)
	}
}

func TestWarperReuseAndErrors(t *testing.T) {
	wp, err := NewWarper(Identity, 4, 3, 4, 3, Bilinear)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := wp.OutSize(); w != 4 || h != 3 {
		t.Errorf("OutSize = %dx%d", w, h)
	}
	src := ramp(4, 3)
	for range 2 {
		got, err := wp.Apply(src)
		if err != nil {
			t.Fatal(err)
		}
		for i := range src.Pix {
			if got.Pix[i] != src.Pix[i] {
				t.Fatalf("identity warp changed pixel %d", i)
			}
		}
	}
	if _, err := wp.Apply(ramp(3, 3)); !errors.Is(err, imgx.ErrSizeMismatch) {
		t.Errorf("err = %v, want ErrSizeMismatch", err)
	}
	if _, err := NewWarper(Identity, 0, 3, 4, 3, Nearest); err == nil {
		t.Error("zero source width: want error")
	}
	if _, err := NewWarper(Identity, 4, 3, 4, 3, Interp(7)); err == nil {
		t.Error("unknown interpolation: want error")
	}
	if _, err := NewWarper(Homography{}, 4, 3, 4, 3, Nearest); err == nil {
		t.Error("singular homography: want error")
	}
}

func TestHomographyFarFromOrigin(t *testing.T) {
	// Regression: degeneracy was judged on raw coordinates, so valid
	// configurations a few thousand pixels from the origin were rejected.
	far := [4]Point{{8000, 4000}, {8032, 4000}, {8032, 4032}, {8000, 4032}}
	h, err := HomographyFromPoints(far, far)
	if err != nil {
		t.Fatalf("identical far squares: %v", err)
	}
	for i := range h {
		if math.Abs(h[i]-Identity[i]) > 1e-9 {
			t.Fatalf("h = %v, want identity", h)
		}
	}

	shifted := [4]Point{{3000, 3000}, {3001, 3000}, {3001, 3001}, {3000, 3001}}
	h, err = HomographyFromPoints(unitSquare, shifted)
	if err != nil {
		t.Fatalf("unit square shifted by 3000: %v", err)
	}
	if q := mustApply(t, h, Point{0.5, 0.5}); !nearPt(q, Point{3000.5, 3000.5}) {
		t.Errorf("centre maps to %v, want (3000.5, 3000.5)", q)
	}
}

func TestHomographyRejectsNonFiniteAndCoincidentPoints(t *testing.T) {
	nan := [4]Point{{math.NaN(), 0}, {1, 0}, {1, 1}, {0, 1}}
	inf := [4]Point{{0, 0}, {math.Inf(1), 0}, {1, 1}, {0, 1}}
	same := [4]Point{{2, 2}, {2, 2}, {2, 2}, {2, 2}}
	for name, pts := range map[string][4]Point{"NaN": nan, "Inf": inf, "coincident": same} {
		if _, err := HomographyFromPoints(pts, unitSquare); !errors.Is(err, ErrDegenerate) {
			t.Errorf("%s source: err = %v, want ErrDegenerate", name, err)
		}
		if _, err := HomographyFromPoints(unitSquare, pts); !errors.Is(err, ErrDegenerate) {
			t.Errorf("%s target: err = %v, want ErrDegenerate", name, err)
		}
	}
}

func TestWarpNearestNegativeHalfPixel(t *testing.T) {
	// Regression: output (0, 0) reads source x = −0.5, which math.Round
	// turns into −1, indexing Pix[−1].
	src, _ := imgx.GrayFromSlice(2, 2, []float32{10, 20, 30, 40})
	h := Homography{1, 0, 0.5, 0, 1, 0, 0, 0, 1}
	got, err := Warp(src, h, 2, 2, Nearest)
	if err != nil {
		t.Fatal(err)
	}
	if got.At(0, 0) != 10 || got.At(0, 1) != 30 {
		t.Errorf("left column = %v, %v, want the source's left column 10, 30", got.At(0, 0), got.At(0, 1))
	}
}

func TestNewWarperRejectsNonFiniteHomography(t *testing.T) {
	for _, h := range []Homography{
		{math.NaN(), 0, 0, 0, 1, 0, 0, 0, 1},
		{1, 0, math.Inf(1), 0, 1, 0, 0, 0, 1},
	} {
		if _, err := NewWarper(h, 4, 4, 4, 4, Nearest); !errors.Is(err, ErrDegenerate) {
			t.Errorf("%v: err = %v, want ErrDegenerate", h, err)
		}
	}
}
