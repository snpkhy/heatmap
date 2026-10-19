package imgx

import "fmt"

// Gray is a single-channel image with float32 intensities, nominally in [0, 255].
// Pixels are stored row by row: pixel (x, y) is Pix[y*W+x].
type Gray struct {
	W, H int
	Pix  []float32
}

// NewGray returns a black w×h image. It panics if w or h is not positive.
func NewGray(w, h int) *Gray {
	mustPositive(w, h)
	return &Gray{W: w, H: h, Pix: make([]float32, w*h)}
}

// GrayFromSlice wraps the given pixels (row by row) into a w×h image.
// It returns an error if len(pix) != w*h.
func GrayFromSlice(w, h int, pix []float32) (*Gray, error) {
	if w <= 0 || h <= 0 || len(pix) != w*h {
		return nil, fmt.Errorf("imgx: %d pixels do not form a %dx%d image", len(pix), w, h)
	}
	return &Gray{W: w, H: h, Pix: pix}, nil
}

// At returns the pixel at (x, y). The coordinates must be inside the image.
func (g *Gray) At(x, y int) float32 {
	return g.Pix[y*g.W+x]
}

// Set sets the pixel at (x, y). The coordinates must be inside the image.
func (g *Gray) Set(x, y int, v float32) {
	g.Pix[y*g.W+x] = v
}

// AtClamped returns the pixel at (x, y), replicating the nearest border pixel
// when the coordinates fall outside the image. Filters use it to handle borders.
func (g *Gray) AtClamped(x, y int) float32 {
	return g.Pix[clampInt(y, 0, g.H-1)*g.W+clampInt(x, 0, g.W-1)]
}

// InBounds reports whether (x, y) lies inside the image.
func (g *Gray) InBounds(x, y int) bool {
	return x >= 0 && x < g.W && y >= 0 && y < g.H
}

// Clone returns a deep copy of the image.
func (g *Gray) Clone() *Gray {
	pix := make([]float32, len(g.Pix))
	copy(pix, g.Pix)
	return &Gray{W: g.W, H: g.H, Pix: pix}
}

// Clamp limits every pixel to [0, 255] in place and returns the image.
func (g *Gray) Clamp() *Gray {
	for i, v := range g.Pix {
		g.Pix[i] = Clamp255(v)
	}
	return g
}

// SameSize reports whether two images have equal width and height.
func SameSize(a, b *Gray) bool {
	return a.W == b.W && a.H == b.H
}

// CheckSameSize returns ErrSizeMismatch (wrapped with both sizes) when a and b differ in size.
func CheckSameSize(a, b *Gray) error {
	if !SameSize(a, b) {
		return fmt.Errorf("%w: %dx%d vs %dx%d", ErrSizeMismatch, a.W, a.H, b.W, b.H)
	}
	return nil
}
