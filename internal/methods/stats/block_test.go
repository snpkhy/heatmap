package stats

import (
	"errors"
	"math"
	"math/rand/v2"
	"testing"

	"heatmap/internal/imgx"
)

func near(a, b float32) bool { return math.Abs(float64(a-b)) <= 1e-3 }

func flat(w, h int, v float32) *imgx.Gray {
	g := imgx.NewGray(w, h)
	for i := range g.Pix {
		g.Pix[i] = v
	}
	return g
}

func TestBlockStatsHandComputed(t *testing.T) {
	// 4×2 image, 2×2 blocks:
	//   1 3 | 10 10
	//   5 7 | 10 10
	// left:  μ = 4,  σ² = (1+9+25+49)/4 − 16 = 5
	// right: μ = 10, σ² = 0
	g, err := imgx.GrayFromSlice(4, 2, []float32{1, 3, 10, 10, 5, 7, 10, 10})
	if err != nil {
		t.Fatal(err)
	}
	grid, err := BlockStats(g, 2)
	if err != nil {
		t.Fatal(err)
	}
	if grid.Cols != 2 || grid.Rows != 1 {
		t.Fatalf("grid %dx%d, want 2x1", grid.Cols, grid.Rows)
	}
	if !near(grid.Mean[0], 4) || !near(grid.Var[0], 5) || !near(grid.Mean[1], 10) || !near(grid.Var[1], 0) {
		t.Errorf("mean %v var %v, want [4 10] [5 0]", grid.Mean, grid.Var)
	}
}

func TestBlockStatsPartialBlocks(t *testing.T) {
	// 5×3 with 2×2 blocks → 3×2 grid; the last column is 1 pixel wide and
	// the last row 1 pixel high, each using its own pixel count.
	g := imgx.NewGray(5, 3)
	for i := range g.Pix {
		g.Pix[i] = float32(i)
	}
	grid, err := BlockStats(g, 2)
	if err != nil {
		t.Fatal(err)
	}
	if grid.Cols != 3 || grid.Rows != 2 {
		t.Fatalf("grid %dx%d, want 3x2", grid.Cols, grid.Rows)
	}
	// Block (2,0) holds pixels 4 and 9: μ = 6.5, σ² = 6.25.
	if !near(grid.Mean[2], 6.5) || !near(grid.Var[2], 6.25) {
		t.Errorf("block (2,0): mean %v var %v, want 6.5 6.25", grid.Mean[2], grid.Var[2])
	}
	// Block (2,1) is the single pixel 14.
	if !near(grid.Mean[5], 14) || !near(grid.Var[5], 0) {
		t.Errorf("block (2,1): mean %v var %v, want 14 0", grid.Mean[5], grid.Var[5])
	}
}

func TestBlockStatsBadBlock(t *testing.T) {
	if _, err := BlockStats(flat(4, 4, 0), 0); err == nil {
		t.Error("block size 0 was accepted")
	}
}

func TestBlockChangeConfigValidation(t *testing.T) {
	bad := []func(*BlockChangeConfig){
		func(c *BlockChangeConfig) { c.Block = 0 },
		func(c *BlockChangeConfig) { c.MeanThresh = 0 },
		func(c *BlockChangeConfig) { c.StdThresh = -1 },
		func(c *BlockChangeConfig) { c.Step = 0 },
		func(c *BlockChangeConfig) { c.NoiseFactor = -1 },
		func(c *BlockChangeConfig) { c.NoiseFactor, c.NoiseStep = 3, 0 },
		func(c *BlockChangeConfig) { c.Warmup = -1 },
		func(c *BlockChangeConfig) { c.Persist = 0 },
	}
	for i, mutate := range bad {
		cfg := DefaultBlockChangeConfig()
		mutate(&cfg)
		if _, err := NewBlockChange(cfg); err == nil {
			t.Errorf("bad config #%d was accepted: %+v", i, cfg)
		}
	}
}

// immediate returns a configuration with fixed thresholds (mean 4, std 5),
// Step 0.2, no warm-up and no persistence delay, so a test can check the
// basic rule one frame at a time.
func immediate() BlockChangeConfig {
	cfg := DefaultBlockChangeConfig()
	cfg.MeanThresh, cfg.StdThresh, cfg.NoiseFactor = 4, 5, 0
	cfg.Step, cfg.Warmup, cfg.Persist = 0.2, 0, 1
	return cfg
}

func newDetector(t *testing.T, cfg BlockChangeConfig) *BlockChange {
	t.Helper()
	d, err := NewBlockChange(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func apply(t *testing.T, d *BlockChange, g *imgx.Gray) *imgx.Gray {
	t.Helper()
	mask, err := d.Apply(g)
	if err != nil {
		t.Fatal(err)
	}
	return mask
}

// blockOn reports whether the whole block (bx, by) of the mask is 255 (true)
// or 0 (false); a mixed block fails the test.
func blockOn(t *testing.T, mask *imgx.Gray, bx, by, block int) bool {
	t.Helper()
	first := mask.At(bx*block, by*block)
	for y := by * block; y < min((by+1)*block, mask.H); y++ {
		for x := bx * block; x < min((bx+1)*block, mask.W); x++ {
			if mask.At(x, y) != first {
				t.Fatalf("block (%d,%d) is not uniform", bx, by)
			}
		}
	}
	return first == 255
}

func TestBlockChangeMeanAndVariance(t *testing.T) {
	cfg := immediate()
	cfg.Block, cfg.MeanThresh, cfg.StdThresh = 4, 4, 5
	d := newDetector(t, cfg)

	base := flat(12, 4, 50) // three 4×4 blocks
	if mask := apply(t, d, base); mask.Pix[0] != 0 {
		t.Fatal("first frame is not empty")
	}

	frame := base.Clone()
	// Block 0: every pixel +10 → the mean changes by 10, the spread stays 0.
	for y := range 4 {
		for x := range 4 {
			frame.Set(x, y, 60)
		}
	}
	// Block 1: half the pixels +20, half −20 → the mean stays 50, σ becomes 20.
	for y := range 4 {
		for x := 4; x < 8; x++ {
			if (x+y)%2 == 0 {
				frame.Set(x, y, 70)
			} else {
				frame.Set(x, y, 30)
			}
		}
	}
	// Block 2: every pixel +2 → below both thresholds.
	for y := range 4 {
		for x := 8; x < 12; x++ {
			frame.Set(x, y, 52)
		}
	}

	mask := apply(t, d, frame)
	if !blockOn(t, mask, 0, 0, 4) {
		t.Error("a change of the block mean was not detected")
	}
	if !blockOn(t, mask, 1, 0, 4) {
		t.Error("a change of the block variance was not detected")
	}
	if blockOn(t, mask, 2, 0, 4) {
		t.Error("a change below the thresholds was reported")
	}
}

func TestBlockChangeFollowsSlowDrift(t *testing.T) {
	// Brightness creeps up by 0.5 per frame — 50 levels in total, far above
	// MeanThresh, yet never a detection because the reference keeps up.
	cfg := immediate()
	cfg.Step = 0.5
	d := newDetector(t, cfg)
	for i := range 100 {
		mask := apply(t, d, flat(16, 16, 50+0.5*float32(i)))
		for _, v := range mask.Pix {
			if v != 0 {
				t.Fatalf("slow drift reported as motion at frame %d", i)
			}
		}
	}
}

func TestBlockChangeObjectLeaves(t *testing.T) {
	cfg := immediate()
	cfg.Block = 4
	d := newDetector(t, cfg)
	empty := flat(8, 4, 50)
	apply(t, d, empty)

	with := empty.Clone()
	for y := range 4 {
		for x := range 4 {
			with.Set(x, y, 150)
		}
	}
	if mask := apply(t, d, with); !blockOn(t, mask, 0, 0, 4) || blockOn(t, mask, 1, 0, 4) {
		t.Fatal("object block not detected, or its neighbour was")
	}
	// One frame with the object moves the reference by a single Step, so
	// once the object is gone the block is immediately still again.
	if mask := apply(t, d, empty); blockOn(t, mask, 0, 0, 4) {
		t.Error("block still reported after the object left")
	}
}

func TestBlockChangePersist(t *testing.T) {
	cfg := immediate()
	cfg.Block, cfg.Persist = 4, 3
	d := newDetector(t, cfg)
	empty := flat(8, 4, 50)
	with := empty.Clone()
	for y := range 4 {
		for x := range 4 {
			with.Set(x, y, 150)
		}
	}
	apply(t, d, empty)

	// A one-frame flash (a snowflake) is never reported.
	if blockOn(t, apply(t, d, with), 0, 0, 4) {
		t.Error("reported after 1 changed frame, Persist is 3")
	}
	if blockOn(t, apply(t, d, empty), 0, 0, 4) {
		t.Error("reported on a still frame")
	}
	// A change that stays is reported on its third frame, and the counter
	// starts again after the still frame above.
	for frame := 1; frame <= 4; frame++ {
		got := blockOn(t, apply(t, d, with), 0, 0, 4)
		if want := frame >= 3; got != want {
			t.Errorf("changed frame %d: reported = %v, want %v", frame, got, want)
		}
	}
}

func TestBlockChangeWarmup(t *testing.T) {
	// The first frame is corrupted (as by a snowflake). With warm-up the
	// reference averages it away; without it the block stays "changed".
	bad := flat(4, 4, 80)
	good := flat(4, 4, 50)
	for _, warmup := range []int{0, 10} {
		cfg := immediate()
		cfg.Block, cfg.Warmup = 4, warmup
		d := newDetector(t, cfg)
		apply(t, d, bad)
		var last *imgx.Gray
		for range 12 {
			last = apply(t, d, good)
		}
		if got, want := blockOn(t, last, 0, 0, 4), warmup == 0; got != want {
			t.Errorf("Warmup %d: block reported = %v, want %v", warmup, got, want)
		}
	}
}

func TestBlockChangeReferenceIsRobust(t *testing.T) {
	// A block is hit by a strong outlier (a snowflake) in 2 frames out of 5.
	// A running average would settle near 50 + 0.4·100 = 90 and then call
	// every normal frame "changed". The median-like reference must stay
	// close to the quiet level.
	cfg := immediate()
	cfg.Block = 4
	d := newDetector(t, cfg)
	quiet := flat(4, 4, 50)
	hit := flat(4, 4, 150)
	for f := range 300 {
		if f%5 == 1 || f%5 == 3 {
			apply(t, d, hit)
		} else {
			apply(t, d, quiet)
		}
	}
	if ref := d.refMean[0]; ref < 49 || ref > 52 {
		t.Errorf("reference mean = %v, want about 50", ref)
	}
	if blockOn(t, apply(t, d, quiet), 0, 0, 4) {
		t.Error("a normal frame is reported as changed")
	}
}

func TestBlockChangeAbsorbsStoppedObject(t *testing.T) {
	// The block mean jumps by 24 and stays. With Step 0.2 and MeanThresh 4
	// it is reported for (24 − 4)/0.2 = 100 frames and then becomes background.
	cfg := immediate()
	cfg.Block = 4
	d := newDetector(t, cfg)
	apply(t, d, flat(4, 4, 50))
	parked := flat(4, 4, 74)
	for f := 1; f <= 120; f++ {
		got := blockOn(t, apply(t, d, parked), 0, 0, 4)
		switch {
		case f <= 95 && !got:
			t.Fatalf("frame %d: a new object is not reported", f)
		case f >= 105 && got:
			t.Fatalf("frame %d: a stopped object is still reported", f)
		}
	}
}

func TestBlockChangeAdaptiveThreshold(t *testing.T) {
	// Block 0 is restless: its mean wanders randomly within 50 ± 5, so it is
	// typically 2.5 away from its median. Block 1 is calm. After the detector
	// has learned that, a rise of 4 is reported in the calm block but not in
	// the restless one, where it is within three times the usual fluctuation.
	cfg := immediate()
	cfg.Block, cfg.MeanThresh, cfg.StdThresh = 4, 1, 1.5
	cfg.NoiseFactor, cfg.NoiseStep, cfg.Step = 3, 0.02, 0.05
	d := newDetector(t, cfg)
	scene := func(wobble, rise float32) *imgx.Gray {
		g := flat(8, 4, 50+rise)
		for y := range 4 {
			for x := range 4 {
				g.Set(x, y, 50+rise+wobble)
			}
		}
		return g
	}
	rng := rand.New(rand.NewPCG(3, 3))
	var last *imgx.Gray
	for range 600 {
		last = apply(t, d, scene(float32(rng.Float64()*10-5), 0))
	}
	if blockOn(t, last, 0, 0, 4) || blockOn(t, last, 1, 0, 4) {
		t.Fatal("a block is reported while nothing unusual happens")
	}
	if dev := d.devMean[0]; dev < 2 || dev > 3 {
		t.Errorf("fluctuation level of the restless block = %v, want about 2.5", dev)
	}
	if dev := d.devMean[1]; dev > 0.1 {
		t.Errorf("fluctuation level of the calm block = %v, want about 0", dev)
	}

	mask := apply(t, d, scene(0, 4))
	if blockOn(t, mask, 0, 0, 4) {
		t.Error("the restless block reported a change within its usual fluctuation")
	}
	if !blockOn(t, mask, 1, 0, 4) {
		t.Error("the calm block did not report the same change")
	}
}

func TestBlockChangeSizeMismatchAndReset(t *testing.T) {
	d := newDetector(t, DefaultBlockChangeConfig())
	apply(t, d, flat(16, 16, 0))
	if _, err := d.Apply(flat(8, 8, 0)); !errors.Is(err, imgx.ErrSizeMismatch) {
		t.Errorf("frame of another size: err = %v", err)
	}
	d.Reset()
	if _, err := d.Apply(flat(8, 8, 0)); err != nil {
		t.Errorf("after Reset: %v", err)
	}
}
