package keypoints

import (
	"fmt"
	"math"

	"heatmap/internal/imgx"
)

// Match is the displacement of one point between two frames: the point was at
// (X, Y) in the previous frame and is at (X+DX, Y+DY) in the current one.
type Match struct {
	X, Y   int
	DX, DY int
	// Cost is the mean absolute difference per pixel between the block in
	// the previous frame and the best block in the current one. Low values
	// mean a confident match.
	Cost float32
}

// Magnitude returns the length of the displacement in pixels.
func (m Match) Magnitude() float64 {
	return math.Hypot(float64(m.DX), float64(m.DY))
}

// MatchBlocks finds where every point moved between prev and cur by block
// matching. The block×block window centred on the point in prev is compared
// with every window in cur whose centre is at most search pixels away in x
// and y (exhaustive search), using the sum of absolute differences
//
//	SAD(dx, dy) = Σ |prev(x+i, y+j) − cur(x+dx+i, y+dy+j)|
//
// The displacement with the smallest SAD wins; on a tie the shorter one is
// preferred, so a static or featureless block reports no motion. Pixels
// outside the image replicate the border. block must be odd and ≥ 3.
//
// Block matching works best on corners: on a flat area every displacement
// matches equally well, and along an edge only the motion across the edge is
// defined (the aperture problem). That is why p6 matches Harris corners.
func MatchBlocks(prev, cur *imgx.Gray, points []Corner, block, search int) ([]Match, error) {
	if block < 3 || block%2 == 0 {
		return nil, fmt.Errorf("keypoints: block size %d must be odd and at least 3", block)
	}
	if search < 0 {
		return nil, fmt.Errorf("keypoints: search radius %d must not be negative", search)
	}
	if err := imgx.CheckSameSize(prev, cur); err != nil {
		return nil, fmt.Errorf("keypoints: block matching: %w", err)
	}
	r := block / 2
	ref := make([]float32, block*block)
	out := make([]Match, 0, len(points))
	for _, p := range points {
		for j := range block {
			for i := range block {
				ref[j*block+i] = prev.AtClamped(p.X+i-r, p.Y+j-r)
			}
		}
		best := Match{X: p.X, Y: p.Y, Cost: float32(math.Inf(1))}
		bestLen := math.MaxInt
		for dy := -search; dy <= search; dy++ {
			for dx := -search; dx <= search; dx++ {
				sad := blockSAD(cur, ref, p.X+dx-r, p.Y+dy-r, block, best.Cost)
				l := dx*dx + dy*dy
				if sad < best.Cost || (sad == best.Cost && l < bestLen) {
					best.DX, best.DY, best.Cost, bestLen = dx, dy, sad, l
				}
			}
		}
		best.Cost /= float32(block * block)
		out = append(out, best)
	}
	return out, nil
}

// blockSAD sums |ref − cur| over the block with top-left corner (x0, y0). It
// stops early once the sum exceeds limit, since that candidate cannot win.
func blockSAD(cur *imgx.Gray, ref []float32, x0, y0, block int, limit float32) float32 {
	inside := x0 >= 0 && y0 >= 0 && x0+block <= cur.W && y0+block <= cur.H
	var sad float32
	for j := range block {
		for i := range block {
			var v float32
			if inside {
				v = cur.Pix[(y0+j)*cur.W+x0+i]
			} else {
				v = cur.AtClamped(x0+i, y0+j)
			}
			d := ref[j*block+i] - v
			if d < 0 {
				d = -d
			}
			sad += d
		}
		if sad > limit {
			return sad
		}
	}
	return sad
}
