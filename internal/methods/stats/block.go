package stats

import (
	"fmt"
	"math"

	"heatmap/internal/imgx"
)

// BlockGrid holds per-block statistics of an image split into square blocks.
// Block (bx, by) is at index by*Cols+bx and covers pixels
// [bx·Block, (bx+1)·Block) × [by·Block, (by+1)·Block), cut at the image border.
type BlockGrid struct {
	Block      int // block side in pixels
	Cols, Rows int
	Mean, Var  []float32
}

// BlockStats splits the image into block×block squares and computes the mean
// and the (population) variance of each:
//
//	μ = Σx / n,   σ² = Σx² / n − μ²
//
// Blocks on the right and bottom borders may be smaller; they use their own n.
func BlockStats(g *imgx.Gray, block int) (*BlockGrid, error) {
	if block <= 0 {
		return nil, fmt.Errorf("stats: block size %d must be positive", block)
	}
	grid := &BlockGrid{
		Block: block,
		Cols:  (g.W + block - 1) / block,
		Rows:  (g.H + block - 1) / block,
	}
	n := grid.Cols * grid.Rows
	grid.Mean = make([]float32, n)
	grid.Var = make([]float32, n)

	sum := make([]float64, n)
	sumSq := make([]float64, n)
	count := make([]int, n)
	for y := range g.H {
		row := (y / block) * grid.Cols
		for x := range g.W {
			b := row + x/block
			v := float64(g.Pix[y*g.W+x])
			sum[b] += v
			sumSq[b] += v * v
			count[b]++
		}
	}
	for b := range n {
		mean := sum[b] / float64(count[b])
		grid.Mean[b] = float32(mean)
		grid.Var[b] = float32(max(sumSq[b]/float64(count[b])-mean*mean, 0))
	}
	return grid, nil
}

// BlockChangeConfig holds the parameters of BlockChange.
// Start from DefaultBlockChangeConfig and change what you need.
type BlockChangeConfig struct {
	// Block is the block side in pixels. Larger blocks average out more
	// noise but give a coarser mask.
	Block int
	// MeanThresh is the smallest change of the block mean that can mark a
	// block as changed: the floor of the adaptive threshold (see NoiseFactor).
	MeanThresh float32
	// StdThresh is the same floor for the block standard deviation
	// (√variance). The variance is compared through its square root so that
	// both thresholds are in intensity units.
	StdThresh float32
	// NoiseFactor makes the thresholds adapt to each block: the detector
	// tracks how much the statistics of a block normally fluctuate from
	// frame to frame, and the threshold is NoiseFactor times that level
	// (but never below MeanThresh / StdThresh). 0 switches adaptation off,
	// leaving the two fixed thresholds.
	NoiseFactor float32
	// NoiseStep is how fast the fluctuation level is tracked, in intensity
	// units per frame. It should be several times smaller than Step, so
	// that an object crossing a block is not mistaken for noise.
	NoiseStep float32
	// Step is how far the reference statistics move towards the current
	// ones in each frame, in intensity units. It sets how fast the detector
	// follows slow scene changes and absorbs objects that stopped: a change
	// of size d stops being reported after about (d − threshold)/Step frames.
	Step float32
	// Warmup is the number of first frames during which the reference is a
	// plain cumulative average (rate 1/n), a quick first estimate. Masks
	// produced during warm-up are not reliable.
	Warmup int
	// Persist is how many frames in a row a block must be changed before it
	// is reported as moving. 1 reports immediately. Higher values reject
	// things that hit a block for a single frame (snowflakes, raindrops,
	// sensor glitches) at the cost of reacting Persist−1 frames later.
	Persist int
}

// DefaultBlockChangeConfig returns parameters for a Prewitt gradient magnitude
// image (see edges.Prewitt), chosen on the CDnet 2014 badWeather sequences
// blizzard and skating.
func DefaultBlockChangeConfig() BlockChangeConfig {
	return BlockChangeConfig{
		Block:       8,
		MeanThresh:  1,
		StdThresh:   1.5,
		NoiseFactor: 3,
		NoiseStep:   0.02,
		Step:        0.05,
		Warmup:      10,
		Persist:     2,
	}
}

func (c BlockChangeConfig) validate() error {
	switch {
	case c.Block <= 0:
		return fmt.Errorf("stats: Block %d must be positive", c.Block)
	case c.MeanThresh <= 0 || c.StdThresh <= 0:
		return fmt.Errorf("stats: thresholds (%v, %v) must be positive", c.MeanThresh, c.StdThresh)
	case c.Step <= 0:
		return fmt.Errorf("stats: Step %v must be positive", c.Step)
	case c.NoiseFactor < 0 || (c.NoiseFactor > 0 && c.NoiseStep <= 0):
		return fmt.Errorf("stats: NoiseFactor %v must not be negative and needs a positive NoiseStep (%v)",
			c.NoiseFactor, c.NoiseStep)
	case c.Warmup < 0:
		return fmt.Errorf("stats: Warmup %d must not be negative", c.Warmup)
	case c.Persist < 1:
		return fmt.Errorf("stats: Persist %d must be at least 1", c.Persist)
	}
	return nil
}

// BlockChange detects motion from changes of block statistics over time.
// It keeps a reference mean and standard deviation for every block and calls
// a block changed when
//
//	|μ − μref| > MeanThresh   or   |σ − σref| > StdThresh
//
// Statistics of a whole block change little under pixel noise, but both the
// mean and the spread jump when an object's contour enters the block.
// A block is reported as moving once it has been changed for Persist frames
// in a row.
//
// The reference follows the scene with a fixed step per frame:
//
//	ref ← ref + Step·sign(current − ref)
//
// This recursion converges to the temporal median of the statistic, not to
// its mean. The step does not depend on how large the difference is, so
// short-lived outliers — a snowflake, a passing object — barely move the
// reference, however strong they are, while a wrong reference is always
// corrected at a steady pace.
//
// With NoiseFactor > 0 the same recursion also tracks the typical size of
// |current − ref| for each block, and the thresholds become NoiseFactor times
// that size. A block full of falling snow then needs a larger change to be
// reported than a block of calm, low-contrast road.
//
// The detector is stateful: feed it the frames of one sequence in order.
// It is not safe for concurrent use.
type BlockChange struct {
	cfg             BlockChangeConfig
	w, h            int
	refMean, refStd []float32
	devMean, devStd []float32 // typical |current − ref| of each block
	streak          []int     // consecutive frames each block has been changed
	cols, seen      int
}

// NewBlockChange returns an empty detector, or an error if the configuration is invalid.
func NewBlockChange(cfg BlockChangeConfig) (*BlockChange, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &BlockChange{cfg: cfg}, nil
}

// Reset forgets the reference statistics, so the detector can be reused for another sequence.
func (d *BlockChange) Reset() {
	d.refMean, d.refStd, d.devMean, d.devStd, d.streak, d.seen = nil, nil, nil, nil, nil, 0
}

// Apply compares the block statistics of one frame with the reference and
// returns a full-resolution mask: 255 over every moving block, 0 elsewhere.
// The first frame only initializes the reference and yields an empty mask.
func (d *BlockChange) Apply(g *imgx.Gray) (*imgx.Gray, error) {
	grid, err := BlockStats(g, d.cfg.Block)
	if err != nil {
		return nil, err
	}
	std := make([]float32, len(grid.Var))
	for b, v := range grid.Var {
		std[b] = float32(math.Sqrt(float64(v)))
	}

	mask := imgx.NewGray(g.W, g.H)
	if d.seen == 0 {
		d.w, d.h, d.cols = g.W, g.H, grid.Cols
		d.refMean, d.refStd = grid.Mean, std
		d.devMean = make([]float32, len(std))
		d.devStd = make([]float32, len(std))
		d.streak = make([]int, len(std))
		d.seen = 1
		return mask, nil
	}
	if g.W != d.w || g.H != d.h {
		return nil, fmt.Errorf("%w: frame %dx%d, reference %dx%d", imgx.ErrSizeMismatch, g.W, g.H, d.w, d.h)
	}

	warm := d.seen < d.cfg.Warmup
	for b := range grid.Mean {
		dMean := grid.Mean[b] - d.refMean[b]
		dStd := std[b] - d.refStd[b]
		changed := abs32(dMean) > max(d.cfg.MeanThresh, d.cfg.NoiseFactor*d.devMean[b]) ||
			abs32(dStd) > max(d.cfg.StdThresh, d.cfg.NoiseFactor*d.devStd[b])

		if changed {
			d.streak[b]++
		} else {
			d.streak[b] = 0
		}
		if d.streak[b] >= d.cfg.Persist {
			fillBlock(mask, b%d.cols, b/d.cols, d.cfg.Block)
		}

		if warm {
			rate := 1 / float32(d.seen+1)
			d.refMean[b] += rate * dMean
			d.refStd[b] += rate * dStd
			d.devMean[b] += rate * (abs32(dMean) - d.devMean[b])
			d.devStd[b] += rate * (abs32(dStd) - d.devStd[b])
			continue
		}
		d.refMean[b] += stepTowards(dMean, d.cfg.Step)
		d.refStd[b] += stepTowards(dStd, d.cfg.Step)
		if d.cfg.NoiseFactor > 0 {
			d.devMean[b] += stepTowards(abs32(dMean)-d.devMean[b], d.cfg.NoiseStep)
			d.devStd[b] += stepTowards(abs32(dStd)-d.devStd[b], d.cfg.NoiseStep)
		}
	}
	d.seen++
	return mask, nil
}

func fillBlock(mask *imgx.Gray, bx, by, block int) {
	x0, y0 := bx*block, by*block
	x1, y1 := min(x0+block, mask.W), min(y0+block, mask.H)
	for y := y0; y < y1; y++ {
		row := mask.Pix[y*mask.W+x0 : y*mask.W+x1]
		for i := range row {
			row[i] = 255
		}
	}
}

// stepTowards returns the move of a reference that is diff away from the
// current value: at most step, and never past the value itself.
func stepTowards(diff, step float32) float32 {
	return max(-step, min(step, diff))
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
