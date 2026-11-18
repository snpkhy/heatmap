package keypoints

import (
	"errors"
	"math/rand/v2"
	"testing"

	"heatmap/internal/imgx"
)

// square returns a w×h black image with a white side×side square whose
// top-left pixel is (x0, y0).
func square(w, h, x0, y0, side int) *imgx.Gray {
	g := imgx.NewGray(w, h)
	for y := y0; y < y0+side; y++ {
		for x := x0; x < x0+side; x++ {
			g.Set(x, y, 255)
		}
	}
	return g
}

// texture returns a seeded random image with values in [40, 215].
func texture(w, h int, seed uint64) *imgx.Gray {
	rng := rand.New(rand.NewPCG(seed, seed+1))
	g := imgx.NewGray(w, h)
	for i := range g.Pix {
		g.Pix[i] = float32(40 + rng.IntN(176))
	}
	return g
}

// shift returns src moved by (dx, dy), filling uncovered pixels with fill.
func shift(src *imgx.Gray, dx, dy int, fill float32) *imgx.Gray {
	dst := imgx.NewGray(src.W, src.H)
	for y := range src.H {
		for x := range src.W {
			sx, sy := x-dx, y-dy
			if src.InBounds(sx, sy) {
				dst.Set(x, y, src.At(sx, sy))
			} else {
				dst.Set(x, y, fill)
			}
		}
	}
	return dst
}

func TestHarrisResponseSigns(t *testing.T) {
	g := square(32, 32, 10, 10, 12)
	r := HarrisResponse(g, 0.04, 1)
	if v := r.At(10, 10); v <= 0 {
		t.Errorf("corner response %v, want positive", v)
	}
	if v := r.At(16, 10); v >= 0 {
		t.Errorf("edge response %v, want negative", v)
	}
	if v := r.At(16, 16); v != 0 {
		t.Errorf("flat interior response %v, want 0", v)
	}
	if v := r.At(2, 2); v != 0 {
		t.Errorf("flat background response %v, want 0", v)
	}
}

func TestHarrisFindsSquareCorners(t *testing.T) {
	g := square(40, 40, 10, 12, 15) // corners at (10,12), (24,12), (10,26), (24,26)
	corners, err := Harris(g, DefaultHarrisConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(corners) != 4 {
		t.Fatalf("found %d corners %v, want 4", len(corners), corners)
	}
	want := [][2]int{{10, 12}, {24, 12}, {10, 26}, {24, 26}}
	for _, w := range want {
		found := false
		for _, c := range corners {
			if abs(c.X-w[0]) <= 1 && abs(c.Y-w[1]) <= 1 {
				found = true
			}
		}
		if !found {
			t.Errorf("no corner near %v in %v", w, corners)
		}
	}
	for i := 1; i < len(corners); i++ {
		if corners[i].Response > corners[i-1].Response {
			t.Error("corners must be sorted strongest first")
		}
	}
}

func TestHarrisIgnoresFlatAndEdges(t *testing.T) {
	flat := imgx.NewGray(20, 20)
	half := imgx.NewGray(20, 20)
	for y := range 20 {
		for x := 10; x < 20; x++ {
			half.Set(x, y, 200)
		}
	}
	noisy := imgx.NewGray(20, 20)
	rng := rand.New(rand.NewPCG(1, 1))
	for i := range noisy.Pix {
		noisy.Pix[i] = float32(100 + 3*rng.NormFloat64())
	}
	for name, g := range map[string]*imgx.Gray{"flat": flat, "edge": half, "weak noise": noisy} {
		corners, err := Harris(g, DefaultHarrisConfig())
		if err != nil {
			t.Fatal(err)
		}
		if len(corners) != 0 {
			t.Errorf("%s: found %d corners, want 0", name, len(corners))
		}
	}
}

func TestHarrisMinDistanceAndMaxCorners(t *testing.T) {
	g := texture(48, 48, 3)
	cfg := DefaultHarrisConfig()
	cfg.MinDistance = 6
	cfg.MaxCorners = 0
	corners, err := Harris(g, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(corners) < 10 {
		t.Fatalf("only %d corners on a random texture", len(corners))
	}
	for i := range corners {
		for j := i + 1; j < len(corners); j++ {
			dx, dy := corners[i].X-corners[j].X, corners[i].Y-corners[j].Y
			if dx*dx+dy*dy < 36 {
				t.Fatalf("corners %v and %v are closer than 6", corners[i], corners[j])
			}
		}
	}
	cfg.MaxCorners = 5
	few, err := Harris(g, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(few) != 5 {
		t.Fatalf("MaxCorners 5: got %d", len(few))
	}
	for i := range few {
		if few[i] != corners[i] {
			t.Errorf("MaxCorners must keep the strongest corners: %v vs %v", few[i], corners[i])
		}
	}
}

func TestHarrisConfigValidation(t *testing.T) {
	bad := []func(*HarrisConfig){
		func(c *HarrisConfig) { c.K = 0 },
		func(c *HarrisConfig) { c.K = 0.3 },
		func(c *HarrisConfig) { c.Sigma = 0 },
		func(c *HarrisConfig) { c.Quality = 0 },
		func(c *HarrisConfig) { c.Quality = 1.5 },
		func(c *HarrisConfig) { c.MinResponse = -1 },
		func(c *HarrisConfig) { c.MinDistance = -1 },
		func(c *HarrisConfig) { c.MaxCorners = -1 },
	}
	for i, f := range bad {
		cfg := DefaultHarrisConfig()
		f(&cfg)
		if _, err := Harris(imgx.NewGray(8, 8), cfg); err == nil {
			t.Errorf("case %d: want error", i)
		}
	}
}

func TestMatchBlocksRecoversShift(t *testing.T) {
	prev := texture(40, 40, 7)
	cur := shift(prev, 3, -2, 0)
	points := []Corner{{X: 15, Y: 20}, {X: 25, Y: 18}, {X: 20, Y: 25}}
	matches, err := MatchBlocks(prev, cur, points, 7, 5)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range matches {
		if m.DX != 3 || m.DY != -2 || m.Cost != 0 {
			t.Errorf("match %+v, want DX=3 DY=-2 cost 0", m)
		}
	}
	if got := matches[0].Magnitude(); got < 3.6 || got > 3.61 {
		t.Errorf("magnitude %v, want √13", got)
	}
}

func TestMatchBlocksStaticAndFlat(t *testing.T) {
	g := texture(30, 30, 9)
	matches, err := MatchBlocks(g, g, []Corner{{X: 15, Y: 15}, {X: 0, Y: 0}}, 5, 4)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range matches {
		if m.DX != 0 || m.DY != 0 {
			t.Errorf("static scene: %+v, want no motion", m)
		}
	}
	// On a flat image every displacement ties; the shortest one wins.
	flat := imgx.NewGray(20, 20)
	matches, err = MatchBlocks(flat, flat, []Corner{{X: 10, Y: 10}}, 5, 3)
	if err != nil {
		t.Fatal(err)
	}
	if matches[0].DX != 0 || matches[0].DY != 0 {
		t.Errorf("flat image: %+v, want no motion", matches[0])
	}
}

func TestMatchBlocksBeyondSearch(t *testing.T) {
	prev := texture(40, 40, 11)
	cur := shift(prev, 6, 0, 0)
	matches, err := MatchBlocks(prev, cur, []Corner{{X: 20, Y: 20}}, 7, 3)
	if err != nil {
		t.Fatal(err)
	}
	if matches[0].Cost < 10 {
		t.Errorf("motion beyond the search range must give a poor match, cost %v", matches[0].Cost)
	}
}

func TestMatchBlocksErrors(t *testing.T) {
	g := imgx.NewGray(10, 10)
	for _, b := range []int{1, 4} {
		if _, err := MatchBlocks(g, g, nil, b, 2); err == nil {
			t.Errorf("block %d: want error", b)
		}
	}
	if _, err := MatchBlocks(g, g, nil, 3, -1); err == nil {
		t.Error("negative search: want error")
	}
	if _, err := MatchBlocks(g, imgx.NewGray(9, 10), nil, 3, 2); !errors.Is(err, imgx.ErrSizeMismatch) {
		t.Errorf("err = %v, want ErrSizeMismatch", err)
	}
}

func TestTrackerFollowsMovingSquare(t *testing.T) {
	tr, err := NewTracker(DefaultTrackerConfig())
	if err != nil {
		t.Fatal(err)
	}
	vecs, mask, err := tr.Apply(square(64, 48, 10, 10, 14))
	if err != nil {
		t.Fatal(err)
	}
	if vecs != nil || mask.W != 64 || mask.H != 48 {
		t.Fatalf("first frame: %d vectors, mask %dx%d", len(vecs), mask.W, mask.H)
	}
	for f := 1; f <= 3; f++ {
		vecs, mask, err = tr.Apply(square(64, 48, 10+4*f, 10+2*f, 14))
		if err != nil {
			t.Fatal(err)
		}
		if len(vecs) != 4 {
			t.Fatalf("frame %d: %d moving corners, want 4", f, len(vecs))
		}
		for _, v := range vecs {
			if v.DX != 4 || v.DY != 2 {
				t.Errorf("frame %d: %+v, want DX=4 DY=2", f, v)
			}
			if mask.At(v.X+v.DX, v.Y+v.DY) != 255 {
				t.Errorf("frame %d: mask is empty at the corner's new position", f)
			}
		}
		if mask.At(60, 44) != 0 {
			t.Errorf("frame %d: mask set far from the object", f)
		}
	}
}

func TestTrackerStaticSceneAndReset(t *testing.T) {
	tr, err := NewTracker(DefaultTrackerConfig())
	if err != nil {
		t.Fatal(err)
	}
	g := texture(40, 30, 13)
	if _, _, err := tr.Apply(g); err != nil {
		t.Fatal(err)
	}
	vecs, _, err := tr.Apply(g)
	if err != nil {
		t.Fatal(err)
	}
	if len(vecs) != 0 {
		t.Errorf("static scene: %d moving corners, want 0", len(vecs))
	}
	if _, _, err := tr.Apply(imgx.NewGray(20, 30)); !errors.Is(err, imgx.ErrSizeMismatch) {
		t.Errorf("err = %v, want ErrSizeMismatch", err)
	}
	tr.Reset()
	if _, _, err := tr.Apply(imgx.NewGray(20, 30)); err != nil {
		t.Errorf("after Reset: %v", err)
	}
}

func TestTrackerConfigValidation(t *testing.T) {
	bad := []func(*TrackerConfig){
		func(c *TrackerConfig) { c.Block = 4 },
		func(c *TrackerConfig) { c.Search = 0 },
		func(c *TrackerConfig) { c.MaxCost = 0 },
		func(c *TrackerConfig) { c.MinMotion = -1 },
		func(c *TrackerConfig) { c.MaskRadius = -1 },
		func(c *TrackerConfig) { c.Harris.Sigma = 0 },
	}
	for i, f := range bad {
		cfg := DefaultTrackerConfig()
		f(&cfg)
		if _, err := NewTracker(cfg); err == nil {
			t.Errorf("case %d: want error", i)
		}
	}
}

func TestMotionMask(t *testing.T) {
	m := MotionMask(10, 10, []Match{{X: 1, Y: 1, DX: -1, DY: -1}}, 1)
	// Disc of radius 1 at (0, 0), clipped by the border: (0,0), (1,0), (0,1).
	var on int
	for _, v := range m.Pix {
		if v == 255 {
			on++
		}
	}
	if on != 3 || m.At(0, 0) != 255 || m.At(1, 1) != 0 {
		t.Errorf("%d pixels set, want 3 around (0,0)", on)
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
