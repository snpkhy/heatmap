package color

import (
	"math"

	"heatmap/internal/imgx"
)

// HSV is a color image in the hue–saturation–value space, stored as separate
// planes. Pixel (x, y) of a plane is at index y*W+x.
//
//	Hue is an angle in degrees, in [0, 360)
//	Sat is in [0, 1]
//	Val is in [0, 255], the same scale as the RGB input
type HSV struct {
	W, H          int
	Hue, Sat, Val []float32
}

// NewHSV returns a black w×h HSV image.
func NewHSV(w, h int) *HSV {
	n := w * h
	return &HSV{W: w, H: h, Hue: make([]float32, n), Sat: make([]float32, n), Val: make([]float32, n)}
}

// Value returns the V plane as a Gray image that shares memory with the HSV image.
func (p *HSV) Value() *imgx.Gray {
	return &imgx.Gray{W: p.W, H: p.H, Pix: p.Val}
}

// RGBToHSV converts an RGB image to HSV pixel by pixel (see PixelToHSV).
func RGBToHSV(c *imgx.RGB) *HSV {
	out := NewHSV(c.W, c.H)
	for i := range c.R {
		out.Hue[i], out.Sat[i], out.Val[i] = PixelToHSV(c.R[i], c.G[i], c.B[i])
	}
	return out
}

// HSVToRGB converts an HSV image back to RGB pixel by pixel (see PixelToRGB).
func HSVToRGB(p *HSV) *imgx.RGB {
	out := imgx.NewRGB(p.W, p.H)
	for i := range p.Hue {
		out.R[i], out.G[i], out.B[i] = PixelToRGB(p.Hue[i], p.Sat[i], p.Val[i])
	}
	return out
}

// PixelToHSV converts one RGB pixel (channels in [0, 255]) to HSV:
//
//	V = max(R, G, B)
//	S = (max − min) / max            (0 for black)
//	H = 60° · (G−B)/(max−min)        if max is R (wrapped into [0, 360))
//	    60° · ((B−R)/(max−min) + 2)  if max is G
//	    60° · ((R−G)/(max−min) + 4)  if max is B
//
// Gray pixels (max == min) have no defined hue; H is reported as 0.
func PixelToHSV(r, g, b float32) (h, s, v float32) {
	mx := max(r, g, b)
	mn := min(r, g, b)
	d := mx - mn
	if mx <= 0 || d == 0 {
		return 0, 0, mx
	}
	switch mx {
	case r:
		h = 60 * (g - b) / d
	case g:
		h = 60 * ((b-r)/d + 2)
	default:
		h = 60 * ((r-g)/d + 4)
	}
	if h < 0 {
		h += 360
	}
	return h, d / mx, mx
}

// PixelToRGB converts one HSV pixel (H in degrees, S in [0, 1], V in [0, 255])
// to RGB. With C = V·S, X = C·(1 − |H/60 mod 2 − 1|) and m = V − C, the result
// is (C, X, 0) permuted according to the 60° sector of H, plus m on every channel.
func PixelToRGB(h, s, v float32) (r, g, b float32) {
	hp := math.Mod(float64(h), 360) / 60
	if hp < 0 {
		hp += 6
	}
	c := v * s
	x := c * float32(1-math.Abs(math.Mod(hp, 2)-1))
	m := v - c
	switch int(hp) {
	case 0:
		r, g, b = c, x, 0
	case 1:
		r, g, b = x, c, 0
	case 2:
		r, g, b = 0, c, x
	case 3:
		r, g, b = 0, x, c
	case 4:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	return r + m, g + m, b + m
}
