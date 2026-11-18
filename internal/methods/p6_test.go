package methods_test

import (
	"math/rand/v2"
	"testing"

	"heatmap/internal/imgx"
	"heatmap/internal/methods/keypoints"
	"heatmap/internal/methods/stats"
)

// p6Scene renders frame t: a static random texture with light sensor noise,
// and a 16×16 textured object that moves 3 px right and 3 px up per frame.
func p6Scene(bg, obj *imgx.Gray, rng *rand.Rand, t int) (*imgx.Gray, [4]int) {
	g := bg.Clone()
	x0, y0 := 12+3*t, 44-3*t
	for y := range obj.H {
		for x := range obj.W {
			g.Set(x0+x, y0+y, obj.At(x, y))
		}
	}
	for i := range g.Pix {
		g.Pix[i] += float32(2 * rng.NormFloat64())
	}
	return g, [4]int{x0, y0, x0 + obj.W, y0 + obj.H}
}

func randomTexture(rng *rand.Rand, w, h int, lo, span int) *imgx.Gray {
	g := imgx.NewGray(w, h)
	for i := range g.Pix {
		g.Pix[i] = float32(lo + rng.IntN(span))
	}
	return g
}

// TestP6DominantDirection runs pipeline p6 (Harris corners → block matching
// → motion direction histogram) on a textured scene with one object moving
// up and to the right. Background corners must not count as motion, the
// histogram must point at 45°, and the motion mask must sit on the object.
func TestP6DominantDirection(t *testing.T) {
	const w, h = 80, 64
	rng := rand.New(rand.NewPCG(6, 6))
	bg := randomTexture(rng, w, h, 60, 60)    // low-contrast background
	obj := randomTexture(rng, 16, 16, 0, 256) // high-contrast object

	tracker, err := keypoints.NewTracker(keypoints.DefaultTrackerConfig())
	if err != nil {
		t.Fatal(err)
	}
	hist, err := stats.NewDirectionHistogram(8, 1, false)
	if err != nil {
		t.Fatal(err)
	}

	var onObject, offObject int
	for frame := range 8 {
		g, box := p6Scene(bg, obj, rng, frame)
		vecs, mask, err := tracker.Apply(g)
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range vecs {
			hist.Add(float64(v.DX), float64(v.DY))
		}
		if frame == 0 {
			continue
		}
		if len(vecs) == 0 {
			t.Fatalf("frame %d: no moving corners", frame)
		}
		for y := range h {
			for x := range w {
				if mask.At(x, y) < 128 {
					continue
				}
				// Allow the disc radius around the object's box.
				if x >= box[0]-4 && x < box[2]+4 && y >= box[1]-4 && y < box[3]+4 {
					onObject++
				} else {
					offObject++
				}
			}
		}
	}

	t.Logf("bins %v, mask on/off object %d/%d", hist.Bins, onObject, offObject)
	angle, ok := hist.Dominant()
	if !ok || angle != 45 {
		t.Fatalf("dominant direction %v (ok=%v), want 45°; bins %v", angle, ok, hist.Bins)
	}
	if share := hist.Normalized()[1]; share < 0.8 {
		t.Errorf("only %.0f%% of the vectors point up-right; bins %v", 100*share, hist.Bins)
	}
	if onObject == 0 || offObject > onObject/20 {
		t.Errorf("mask: %d pixels on the object, %d elsewhere", onObject, offObject)
	}
}
