package stats

import (
	"fmt"
	"math"
)

// DirectionHistogram counts motion vectors by direction, the last stage of
// p6: over a whole sequence it shows where people or cars mostly go.
//
// Angles are measured as seen on screen: 0° points right, 90° up, 180° left,
// 270° down. Image y grows downwards, so a vector (dx, dy) has the angle
// atan2(−dy, dx). Bin i of n is centred on i·360°/n and covers
// [i·360°/n − 180°/n, i·360°/n + 180°/n); with 8 bins they are the compass
// directions right, up-right, up, … down-right.
//
// It is not safe for concurrent use.
type DirectionHistogram struct {
	// Bins holds the count (or the summed length, see Weighted) per direction.
	Bins []float64
	// MinMagnitude ignores vectors shorter than this many pixels, whose
	// direction is mostly rounding noise.
	MinMagnitude float64
	// Weighted adds the length of every vector instead of 1, so fast motion
	// counts more than slow motion.
	Weighted bool
}

// NewDirectionHistogram returns an empty histogram with the given number of bins (≥ 2).
func NewDirectionHistogram(bins int, minMagnitude float64, weighted bool) (*DirectionHistogram, error) {
	if bins < 2 {
		return nil, fmt.Errorf("stats: direction histogram needs at least 2 bins, got %d", bins)
	}
	if !(minMagnitude >= 0) { // also rejects NaN
		return nil, fmt.Errorf("stats: minimum magnitude %v must not be negative", minMagnitude)
	}
	return &DirectionHistogram{Bins: make([]float64, bins), MinMagnitude: minMagnitude, Weighted: weighted}, nil
}

// Angle returns the screen angle of a vector in degrees, in [0, 360).
func Angle(dx, dy float64) float64 {
	a := math.Atan2(-dy, dx) * 180 / math.Pi
	if a < 0 {
		a += 360
	}
	return a
}

// Bin returns the index of the bin that holds the given angle in degrees.
func (h *DirectionHistogram) Bin(angle float64) int {
	n := len(h.Bins)
	width := 360 / float64(n)
	i := int(math.Floor((angle + width/2) / width))
	return ((i % n) + n) % n
}

// BinCenter returns the angle in degrees at the centre of bin i.
func (h *DirectionHistogram) BinCenter(i int) float64 {
	return float64(i) * 360 / float64(len(h.Bins))
}

// Add counts one motion vector. It reports whether the vector was counted
// (false when it is shorter than MinMagnitude or has zero length).
func (h *DirectionHistogram) Add(dx, dy float64) bool {
	mag := math.Hypot(dx, dy)
	if mag == 0 || mag < h.MinMagnitude {
		return false
	}
	w := 1.0
	if h.Weighted {
		w = mag
	}
	h.Bins[h.Bin(Angle(dx, dy))] += w
	return true
}

// Total returns the sum of all bins.
func (h *DirectionHistogram) Total() float64 {
	var s float64
	for _, v := range h.Bins {
		s += v
	}
	return s
}

// Normalized returns the bins divided by their total, so they sum to 1.
// An empty histogram gives all zeros.
func (h *DirectionHistogram) Normalized() []float64 {
	out := make([]float64, len(h.Bins))
	total := h.Total()
	if total == 0 {
		return out
	}
	for i, v := range h.Bins {
		out[i] = v / total
	}
	return out
}

// Dominant returns the centre angle of the fullest bin. ok is false for an
// empty histogram. On a tie the lower bin wins.
func (h *DirectionHistogram) Dominant() (angle float64, ok bool) {
	best := -1
	for i, v := range h.Bins {
		if v > 0 && (best < 0 || v > h.Bins[best]) {
			best = i
		}
	}
	if best < 0 {
		return 0, false
	}
	return h.BinCenter(best), true
}

// Reset empties the histogram.
func (h *DirectionHistogram) Reset() {
	clear(h.Bins)
}
