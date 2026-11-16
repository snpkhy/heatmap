package texture

import (
	"fmt"

	"heatmap/internal/imgx"
)

// lbpBins is the number of distinct 8-neighbour LBP codes.
const lbpBins = 256

// lbpOffsets lists the 8 neighbours clockwise from the top-left one.
// Neighbour i contributes bit i of the code.
var lbpOffsets = [8][2]int{
	{-1, -1}, {0, -1}, {1, -1},
	{1, 0}, {1, 1}, {0, 1},
	{-1, 1}, {-1, 0},
}

// LBP returns the local binary pattern code (0–255) of every pixel.
// Bit i is set when neighbour i is brighter than the centre by more than
// the fraction margin of the centre's value:
//
//	bit_i = 1  if  neighbour_i ≥ centre · (1 + margin)
//
// The code depends only on the ratio between a pixel and its neighbours.
// A shadow multiplies both by the same factor, so it keeps the code the same.
// A margin of a few percent (e.g. 0.05) stops noise from flipping bits on
// flat areas. Pixels outside the image replicate the border.
func LBP(src *imgx.Gray, margin float32) *imgx.Gray {
	dst := imgx.NewGray(src.W, src.H)
	for y := 0; y < src.H; y++ {
		for x := 0; x < src.W; x++ {
			limit := src.At(x, y) * (1 + margin)
			var code uint8
			for i, o := range lbpOffsets {
				if src.AtClamped(x+o[0], y+o[1]) >= limit {
					code |= 1 << i
				}
			}
			dst.Set(x, y, float32(code))
		}
	}
	return dst
}

// BlockLBP measures how much the texture of every block×block cell differs
// from the background. Each block is described by the histogram of its LBP
// codes; the background model keeps a running average of these histograms.
//
// BlockLBP is stateful: feed it the frames of one sequence in order.
type BlockLBP struct {
	// Block is the side of a square block in pixels.
	Block int
	// Margin is the relative LBP noise margin, e.g. 0.05 for 5 %, see LBP.
	Margin float32
	// Alpha is the background learning rate in (0, 1]:
	// B ← (1 − Alpha)·B + Alpha·H. Small values adapt slowly.
	Alpha float32

	w, h       int
	background []float32 // one normalised histogram per block
}

// NewBlockLBP returns a detector with the given block size, LBP margin
// and background learning rate.
func NewBlockLBP(block int, margin, alpha float32) (*BlockLBP, error) {
	if block < 2 {
		return nil, fmt.Errorf("texture: block lbp block size must be at least 2, got %d", block)
	}
	if margin < 0 {
		return nil, fmt.Errorf("texture: block lbp margin must not be negative, got %v", margin)
	}
	if alpha <= 0 || alpha > 1 {
		return nil, fmt.Errorf("texture: block lbp alpha must be in (0, 1], got %v", alpha)
	}
	return &BlockLBP{Block: block, Margin: margin, Alpha: alpha}, nil
}

// Apply processes the next frame and returns its texture change map.
// Every pixel of a block holds the distance between the block's histogram H
// and its background histogram B, scaled to [0, 255]:
//
//	d = 255 · (1 − Σ min(H_i, B_i))
//
// 0 means the same texture, 255 a completely different one. The first frame
// only initialises the background, so its map is all zeros.
func (b *BlockLBP) Apply(frame *imgx.Gray) (*imgx.Gray, error) {
	hist := b.histograms(LBP(frame, b.Margin))
	dst := imgx.NewGray(frame.W, frame.H)

	if b.background == nil {
		b.w, b.h = frame.W, frame.H
		b.background = hist
		return dst, nil
	}
	if frame.W != b.w || frame.H != b.h {
		return nil, fmt.Errorf("texture: block lbp: %w: %dx%d vs %dx%d",
			imgx.ErrSizeMismatch, frame.W, frame.H, b.w, b.h)
	}

	cols := (frame.W + b.Block - 1) / b.Block
	for i := 0; i < len(hist); i += lbpBins {
		h, bg := hist[i:i+lbpBins], b.background[i:i+lbpBins]
		var common float32
		for j, hv := range h {
			common += min(hv, bg[j])
			bg[j] = (1-b.Alpha)*bg[j] + b.Alpha*hv
		}
		n := i / lbpBins
		b.fill(dst, n%cols, n/cols, imgx.Clamp255(255*(1-common)))
	}
	return dst, nil
}

// Reset forgets the background, e.g. before a new sequence.
func (b *BlockLBP) Reset() {
	b.background = nil
}

// histograms returns the LBP code histogram of every block, row by row.
// Each histogram sums to 1, so blocks cut off by the image border compare
// fairly with full ones.
func (b *BlockLBP) histograms(codes *imgx.Gray) []float32 {
	cols := (codes.W + b.Block - 1) / b.Block
	rows := (codes.H + b.Block - 1) / b.Block
	hist := make([]float32, cols*rows*lbpBins)

	for y := 0; y < codes.H; y++ {
		for x := 0; x < codes.W; x++ {
			n := (y/b.Block)*cols + x/b.Block
			hist[n*lbpBins+int(codes.At(x, y))]++
		}
	}
	for n := 0; n < cols*rows; n++ {
		bw := min(b.Block, codes.W-(n%cols)*b.Block)
		bh := min(b.Block, codes.H-(n/cols)*b.Block)
		area := float32(bw * bh)
		for j := n * lbpBins; j < (n+1)*lbpBins; j++ {
			hist[j] /= area
		}
	}
	return hist
}

// fill sets every pixel of block (bx, by) to v.
func (b *BlockLBP) fill(dst *imgx.Gray, bx, by int, v float32) {
	for y := by * b.Block; y < min((by+1)*b.Block, dst.H); y++ {
		for x := bx * b.Block; x < min((bx+1)*b.Block, dst.W); x++ {
			dst.Set(x, y, v)
		}
	}
}
