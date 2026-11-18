package methods_test

import (
	"math"
	"math/rand/v2"
	"testing"

	"heatmap/internal/imgx"
	"heatmap/internal/methods/fourier"
	"heatmap/internal/methods/geometry"
	"heatmap/internal/methods/segmentation"
)

// p5Scene renders frame t of a noisy static scene with a bright 12×12 object
// moving 4 px per frame to the right along y ∈ [30, 42).
func p5Scene(rng *rand.Rand, t int) *imgx.Gray {
	const w, h = 96, 64
	g := imgx.NewGray(w, h)
	for i := range g.Pix {
		g.Pix[i] = float32(100 + 12*rng.NormFloat64())
	}
	x0 := 20 + 4*t
	for y := 30; y < 42; y++ {
		for x := x0; x < x0+12; x++ {
			g.Set(x, y, float32(200+12*rng.NormFloat64()))
		}
	}
	return g
}

func centroid(mask *imgx.Gray) (cx, cy float64, n int) {
	for y := range mask.H {
		for x := range mask.W {
			if mask.At(x, y) >= 128 {
				cx += float64(x)
				cy += float64(y)
				n++
			}
		}
	}
	if n > 0 {
		cx /= float64(n)
		cy /= float64(n)
	}
	return cx, cy, n
}

// foregroundOutside counts foreground pixels outside rows [y0, y1).
func foregroundOutside(mask *imgx.Gray, y0, y1 int) int {
	n := 0
	for y := range mask.H {
		if y >= y0 && y < y1 {
			continue
		}
		for x := range mask.W {
			if mask.At(x, y) >= 128 {
				n++
			}
		}
	}
	return n
}

// TestP5NoisySceneToFloorPlan runs pipeline p5 (FFT low-pass → frame
// differencing → perspective warp) on a noisy synthetic sequence. The low-pass
// stage must remove the noise that plain frame differencing reports as
// motion, the moving object must be detected, and the warp must put it where
// the homography says on the floor plan.
func TestP5NoisySceneToFloorPlan(t *testing.T) {
	lowpass := fourier.Filter{Shape: fourier.Gaussian, Cutoff: 0.08}
	diff, err := segmentation.NewFrameDiff(30)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := segmentation.NewFrameDiff(30) // same detector without the filter
	if err != nil {
		t.Fatal(err)
	}

	// The camera looks down at the floor at an angle: the floor rectangle
	// appears as a trapezoid in the image and becomes a 60×120 plan.
	camera := [4]geometry.Point{{X: 30, Y: 0}, {X: 66, Y: 0}, {X: 95, Y: 63}, {X: 0, Y: 63}}
	plan := [4]geometry.Point{{X: 0, Y: 0}, {X: 59, Y: 0}, {X: 59, Y: 119}, {X: 0, Y: 119}}
	h, err := geometry.HomographyFromPoints(camera, plan)
	if err != nil {
		t.Fatal(err)
	}
	warper, err := geometry.NewWarper(h, 96, 64, 60, 120, geometry.Nearest)
	if err != nil {
		t.Fatal(err)
	}

	rng := rand.New(rand.NewPCG(5, 5))
	for frame := range 6 {
		g := p5Scene(rng, frame)
		smooth, err := lowpass.Apply(g)
		if err != nil {
			t.Fatal(err)
		}
		mask, err := diff.Apply(smooth)
		if err != nil {
			t.Fatal(err)
		}
		rawMask, err := raw.Apply(g)
		if err != nil {
			t.Fatal(err)
		}
		floor, err := warper.Apply(mask)
		if err != nil {
			t.Fatal(err)
		}
		if frame == 0 {
			continue // no previous frame yet
		}

		// Noise: rows away from the object (with a margin for the blur).
		if fp := foregroundOutside(mask, 24, 48); fp > 0 {
			t.Errorf("frame %d: %d noise pixels after low-pass, want 0", frame, fp)
		}
		if fp := foregroundOutside(rawMask, 24, 48); fp < 100 {
			t.Errorf("frame %d: only %d noise pixels without the filter — test scene is too easy", frame, fp)
		}

		cx, cy, n := centroid(mask)
		if n < 40 {
			t.Fatalf("frame %d: %d foreground pixels, the moving object is lost", frame, n)
		}
		// Frame differencing sees where the object was and where it is now.
		if wantX := 20 + 4*float64(frame) + 4; math.Abs(cx-wantX) > 3 || math.Abs(cy-35.5) > 2 {
			t.Errorf("frame %d: object centroid (%.1f, %.1f), want ≈ (%.1f, 35.5)", frame, cx, cy, wantX)
		}

		want, _ := h.Apply(geometry.Point{X: cx, Y: cy})
		fx, fy, fn := centroid(floor)
		if fn == 0 {
			t.Fatalf("frame %d: nothing on the floor plan", frame)
		}
		if math.Abs(fx-want.X) > 3 || math.Abs(fy-want.Y) > 3 {
			t.Errorf("frame %d: floor-plan centroid (%.1f, %.1f), homography predicts (%.1f, %.1f)",
				frame, fx, fy, want.X, want.Y)
		}
	}
}
