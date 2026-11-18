package geometry

import (
	"fmt"
	"math"

	"heatmap/internal/imgx"
)

// Interp is the way a pixel value is read at a non-integer source position.
type Interp int

const (
	// Nearest takes the closest pixel. Use it for binary masks: it keeps
	// them binary.
	Nearest Interp = iota
	// Bilinear blends the four surrounding pixels by distance. Smoother for
	// photos and heatmaps.
	Bilinear
)

// Warper applies one perspective transform to many images of the same size,
// e.g. the motion mask of every frame of a sequence. The source position of
// every output pixel is computed once in NewWarper.
//
// Warping uses inverse mapping: for every output pixel the inverse
// homography gives the source position to read from. Forward mapping (pushing
// source pixels to the output) would leave holes where the output is
// stretched.
type Warper struct {
	srcW, srcH int
	outW, outH int
	interp     Interp
	// For each output pixel: whether its source position lies inside the
	// source image, the source pixel to read (Nearest) or the top-left of the
	// four pixels to blend (Bilinear, may be −1), and the bilinear weights.
	inside []bool
	sx, sy []int32
	fx, fy []float32
}

// NewWarper prepares the warp of srcW×srcH images by h (which maps source
// coordinates to output coordinates) into outW×outH images. Output pixels
// whose source position falls outside the source image become 0.
func NewWarper(h Homography, srcW, srcH, outW, outH int, interp Interp) (*Warper, error) {
	if srcW <= 0 || srcH <= 0 || outW <= 0 || outH <= 0 {
		return nil, fmt.Errorf("geometry: invalid warp sizes %dx%d → %dx%d", srcW, srcH, outW, outH)
	}
	if interp != Nearest && interp != Bilinear {
		return nil, fmt.Errorf("geometry: unknown interpolation %d", interp)
	}
	inv, err := h.Inverse()
	if err != nil {
		return nil, err
	}
	n := outW * outH
	wp := &Warper{
		srcW: srcW, srcH: srcH, outW: outW, outH: outH, interp: interp,
		inside: make([]bool, n),
		sx:     make([]int32, n), sy: make([]int32, n),
		fx: make([]float32, n), fy: make([]float32, n),
	}
	for y := range outH {
		for x := range outW {
			i := y*outW + x
			p, ok := inv.Apply(Point{X: float64(x), Y: float64(y)})
			// A pixel covers ±0.5 around its centre.
			if !ok || p.X < -0.5 || p.Y < -0.5 || p.X >= float64(srcW)-0.5 || p.Y >= float64(srcH)-0.5 {
				continue
			}
			wp.inside[i] = true
			if interp == Nearest {
				wp.sx[i] = int32(min(int(math.Round(p.X)), srcW-1))
				wp.sy[i] = int32(min(int(math.Round(p.Y)), srcH-1))
				continue
			}
			x0, y0 := math.Floor(p.X), math.Floor(p.Y)
			wp.sx[i], wp.sy[i] = int32(x0), int32(y0)
			wp.fx[i] = float32(p.X - x0)
			wp.fy[i] = float32(p.Y - y0)
		}
	}
	return wp, nil
}

// OutSize returns the width and height of the warped images.
func (wp *Warper) OutSize() (w, h int) {
	return wp.outW, wp.outH
}

// Apply warps one image. Its size must match the source size given to NewWarper.
func (wp *Warper) Apply(src *imgx.Gray) (*imgx.Gray, error) {
	if src.W != wp.srcW || src.H != wp.srcH {
		return nil, fmt.Errorf("%w: image %dx%d, warper expects %dx%d",
			imgx.ErrSizeMismatch, src.W, src.H, wp.srcW, wp.srcH)
	}
	dst := imgx.NewGray(wp.outW, wp.outH)
	for i, in := range wp.inside {
		switch {
		case !in:
		case wp.interp == Nearest:
			dst.Pix[i] = src.Pix[int(wp.sy[i])*src.W+int(wp.sx[i])]
		default:
			dst.Pix[i] = bilinear(src, int(wp.sx[i]), int(wp.sy[i]), wp.fx[i], wp.fy[i])
		}
	}
	return dst, nil
}

// bilinear blends pixel (x0, y0) with its right, lower and lower-right
// neighbours by the weights fx, fy ∈ [0, 1). Near the border x0 or y0 may be
// −1 or a neighbour may be outside; such pixels replicate the border.
func bilinear(src *imgx.Gray, x0, y0 int, fx, fy float32) float32 {
	a := src.AtClamped(x0, y0)
	b := src.AtClamped(x0+1, y0)
	c := src.AtClamped(x0, y0+1)
	d := src.AtClamped(x0+1, y0+1)
	top := a + fx*(b-a)
	bot := c + fx*(d-c)
	return top + fy*(bot-top)
}

// Warp is a one-off perspective warp: NewWarper followed by Apply.
func Warp(src *imgx.Gray, h Homography, outW, outH int, interp Interp) (*imgx.Gray, error) {
	wp, err := NewWarper(h, src.W, src.H, outW, outH, interp)
	if err != nil {
		return nil, err
	}
	return wp.Apply(src)
}
