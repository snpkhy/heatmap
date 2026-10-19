package imgx

// RGB is a three-channel color image stored as separate float32 planes,
// nominally in [0, 255]. Pixel (x, y) of a plane is at index y*W+x.
// Separate planes make per-channel processing as simple as with Gray.
type RGB struct {
	W, H    int
	R, G, B []float32
}

// NewRGB returns a black w×h color image. It panics if w or h is not positive.
func NewRGB(w, h int) *RGB {
	mustPositive(w, h)
	n := w * h
	return &RGB{W: w, H: h, R: make([]float32, n), G: make([]float32, n), B: make([]float32, n)}
}

// At returns the (r, g, b) values at (x, y). The coordinates must be inside the image.
func (c *RGB) At(x, y int) (r, g, b float32) {
	i := y*c.W + x
	return c.R[i], c.G[i], c.B[i]
}

// Set sets the (r, g, b) values at (x, y). The coordinates must be inside the image.
func (c *RGB) Set(x, y int, r, g, b float32) {
	i := y*c.W + x
	c.R[i], c.G[i], c.B[i] = r, g, b
}

// Clone returns a deep copy of the image.
func (c *RGB) Clone() *RGB {
	out := NewRGB(c.W, c.H)
	copy(out.R, c.R)
	copy(out.G, c.G)
	copy(out.B, c.B)
	return out
}

// Channel returns one plane as a Gray image that shares memory with c.
// idx is 0 for R, 1 for G and 2 for B.
func (c *RGB) Channel(idx int) *Gray {
	planes := [3][]float32{c.R, c.G, c.B}
	return &Gray{W: c.W, H: c.H, Pix: planes[idx]}
}

// IsGrayscale reports whether all three channels are equal at every pixel,
// e.g. for a monochrome image saved in a color format.
func (c *RGB) IsGrayscale() bool {
	for i := range c.R {
		if c.R[i] != c.G[i] || c.G[i] != c.B[i] {
			return false
		}
	}
	return true
}

// Luma converts the image to grayscale using the ITU-R BT.601 weights:
//
//	Y = 0.299·R + 0.587·G + 0.114·B
func (c *RGB) Luma() *Gray {
	out := NewGray(c.W, c.H)
	for i := range out.Pix {
		out.Pix[i] = 0.299*c.R[i] + 0.587*c.G[i] + 0.114*c.B[i]
	}
	return out
}

// GrayToRGB copies a grayscale image into all three channels of a new color image.
func GrayToRGB(g *Gray) *RGB {
	out := NewRGB(g.W, g.H)
	copy(out.R, g.Pix)
	copy(out.G, g.Pix)
	copy(out.B, g.Pix)
	return out
}
