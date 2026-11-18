package geometry

import (
	"errors"
	"fmt"
	"math"
)

// Point is a position in pixel coordinates: x to the right, y down,
// pixel (x, y) has its centre at (x, y).
type Point struct {
	X, Y float64
}

// ErrDegenerate is returned when points do not define a homography, e.g.
// when three of the four points lie on one line.
var ErrDegenerate = errors.New("geometry: degenerate point configuration")

// Homography is a 3×3 projective transform stored row by row:
//
//	⎡u·w⎤   ⎡h0 h1 h2⎤ ⎡x⎤
//	⎢v·w⎥ = ⎢h3 h4 h5⎥ ⎢y⎥
//	⎣ w ⎦   ⎣h6 h7 h8⎦ ⎣1⎦
//
// It maps any quadrilateral onto any other one and models how a flat floor
// seen by a camera at an angle maps onto a top-down floor plan.
type Homography [9]float64

// Identity is the homography that leaves every point in place.
var Identity = Homography{1, 0, 0, 0, 1, 0, 0, 0, 1}

// HomographyFromPoints finds the homography that maps src[i] to dst[i] for
// the four point pairs (direct linear transform). With h8 fixed to 1 every
// pair gives two linear equations in the eight remaining unknowns,
//
//	h0·x + h1·y + h2 − h6·x·u − h7·y·u = u
//	h3·x + h4·y + h5 − h6·x·v − h7·y·v = v
//
// and the 8×8 system is solved by Gaussian elimination with partial
// pivoting. It returns ErrDegenerate if three points of either quadrilateral
// are collinear or a coordinate is NaN or infinite.
//
// Both point sets are first normalised (Hartley): moved so their centroid is
// at the origin and scaled so their mean distance from it is √2. Without
// this, products like x·u reach 10⁷ for points a few thousand pixels from the
// origin, the system becomes badly conditioned, and whether a configuration
// counts as degenerate would depend on where the points are rather than on
// their shape. The result is denormalised as H = T_dst⁻¹ · Ĥ · T_src.
//
// In p5 src are four floor points marked in the camera image (e.g. the
// corners of a parking lot) and dst the same points on the floor plan.
func HomographyFromPoints(src, dst [4]Point) (Homography, error) {
	tSrc, err := normalizer(src)
	if err != nil {
		return Homography{}, err
	}
	tDst, err := normalizer(dst)
	if err != nil {
		return Homography{}, err
	}
	var a [8][9]float64 // augmented matrix [A | b]
	for i := range 4 {
		p, _ := tSrc.Apply(src[i])
		q, _ := tDst.Apply(dst[i])
		x, y, u, v := p.X, p.Y, q.X, q.Y
		a[2*i] = [9]float64{x, y, 1, 0, 0, 0, -x * u, -y * u, u}
		a[2*i+1] = [9]float64{0, 0, 0, x, y, 1, -x * v, -y * v, v}
	}
	sol, err := solve8(a)
	if err != nil {
		return Homography{}, err
	}
	var hn Homography
	copy(hn[:8], sol[:])
	hn[8] = 1
	// Collinear target points still give a solvable system, but the result
	// squashes the plane onto a line: det H = 0. In normalised coordinates the
	// entries of a valid Ĥ are of order 1, so a fixed relative tolerance works.
	var norm float64
	for _, v := range hn {
		norm = max(norm, math.Abs(v))
	}
	if !(math.Abs(hn.det()) > 1e-9*norm*norm*norm) {
		return Homography{}, ErrDegenerate
	}

	tDstInv, err := tDst.Inverse()
	if err != nil {
		return Homography{}, err
	}
	h := tDstInv.Mul(hn).Mul(tSrc)
	scale := h[8]
	if math.Abs(scale) < 1e-12 {
		// The origin maps to infinity; any non-zero scale is valid.
		scale = norm
	}
	for i := range h {
		h[i] /= scale
	}
	return h, nil
}

// normalizer returns the similarity transform that moves the centroid of pts
// to the origin and scales their mean distance from it to √2.
func normalizer(pts [4]Point) (Homography, error) {
	var cx, cy float64
	for _, p := range pts {
		if math.IsNaN(p.X) || math.IsNaN(p.Y) || math.IsInf(p.X, 0) || math.IsInf(p.Y, 0) {
			return Homography{}, fmt.Errorf("%w: point %v is not finite", ErrDegenerate, p)
		}
		cx += p.X / 4
		cy += p.Y / 4
	}
	var mean float64
	for _, p := range pts {
		mean += math.Hypot(p.X-cx, p.Y-cy) / 4
	}
	if !(mean > 0) {
		return Homography{}, fmt.Errorf("%w: all points coincide", ErrDegenerate)
	}
	s := math.Sqrt2 / mean
	return Homography{s, 0, -s * cx, 0, s, -s * cy, 0, 0, 1}, nil
}

// finite reports whether every entry is a finite number.
func (h Homography) finite() bool {
	for _, v := range h {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return true
}

func (h Homography) det() float64 {
	return h[0]*(h[4]*h[8]-h[5]*h[7]) - h[1]*(h[3]*h[8]-h[5]*h[6]) + h[2]*(h[3]*h[7]-h[4]*h[6])
}

// solve8 solves an 8×8 linear system given as an augmented matrix.
func solve8(a [8][9]float64) ([8]float64, error) {
	var scale float64
	for _, row := range a {
		for _, v := range row[:8] {
			scale = max(scale, math.Abs(v))
		}
	}
	const n = 8
	for col := range n {
		// Partial pivoting: bring the row with the largest entry up.
		pivot := col
		for r := col + 1; r < n; r++ {
			if math.Abs(a[r][col]) > math.Abs(a[pivot][col]) {
				pivot = r
			}
		}
		if math.Abs(a[pivot][col]) <= 1e-10*scale {
			return [8]float64{}, ErrDegenerate
		}
		a[col], a[pivot] = a[pivot], a[col]
		for r := col + 1; r < n; r++ {
			f := a[r][col] / a[col][col]
			for c := col; c <= n; c++ {
				a[r][c] -= f * a[col][c]
			}
		}
	}
	var x [8]float64
	for r := n - 1; r >= 0; r-- {
		s := a[r][n]
		for c := r + 1; c < n; c++ {
			s -= a[r][c] * x[c]
		}
		x[r] = s / a[r][r]
	}
	return x, nil
}

// Apply maps a point. ok is false when the point is mapped to infinity
// (it lies on the horizon line of the transform).
func (h Homography) Apply(p Point) (q Point, ok bool) {
	w := h[6]*p.X + h[7]*p.Y + h[8]
	if math.Abs(w) < 1e-12 {
		return Point{}, false
	}
	return Point{
		X: (h[0]*p.X + h[1]*p.Y + h[2]) / w,
		Y: (h[3]*p.X + h[4]*p.Y + h[5]) / w,
	}, true
}

// Inverse returns the homography that undoes h, computed as the adjugate
// divided by the determinant and normalised so that h8 = 1 when possible.
func (h Homography) Inverse() (Homography, error) {
	inv := Homography{
		h[4]*h[8] - h[5]*h[7], h[2]*h[7] - h[1]*h[8], h[1]*h[5] - h[2]*h[4],
		h[5]*h[6] - h[3]*h[8], h[0]*h[8] - h[2]*h[6], h[2]*h[3] - h[0]*h[5],
		h[3]*h[7] - h[4]*h[6], h[1]*h[6] - h[0]*h[7], h[0]*h[4] - h[1]*h[3],
	}
	det := h[0]*inv[0] + h[1]*inv[3] + h[2]*inv[6]
	if !(math.Abs(det) >= 1e-12) || math.IsInf(det, 0) { // also rejects NaN
		return Homography{}, fmt.Errorf("%w: singular homography", ErrDegenerate)
	}
	norm := det
	if math.Abs(inv[8]) > 1e-12 {
		norm = inv[8]
	}
	for i := range inv {
		inv[i] /= norm
	}
	return inv, nil
}

// Mul returns the composition h·g: first g, then h.
func (h Homography) Mul(g Homography) Homography {
	var out Homography
	for r := range 3 {
		for c := range 3 {
			for k := range 3 {
				out[r*3+c] += h[r*3+k] * g[k*3+c]
			}
		}
	}
	return out
}
