package color

import (
	"heatmap/internal/imgx"
)

// YCbCr represents a color image in the ITU-R BT.601 YCbCr space,
// stored as separate float32 planes.
//
//	Y  (luma)            is in [0, 255]
//	Cb (blue-difference) is centered at 128, nominally in [0, 255]
//	Cr (red-difference)  is centered at 128, nominally in [0, 255]
//
// Separating luma from chroma allows night-vision and low-light pipelines (like p2)
// to perform contrast enhancement and motion segmentation solely on the Y channel,
// without being misled by noisy color variations in dark scenes.
type YCbCr struct {
	W, H int
	Y    []float32
	Cb   []float32
	Cr   []float32
}

// NewYCbCr returns a w×h YCbCr image initialized to neutral black (Y=0, Cb=128, Cr=128).
func NewYCbCr(w, h int) *YCbCr {
	n := w * h
	cb := make([]float32, n)
	cr := make([]float32, n)
	for i := 0; i < n; i++ {
		cb[i] = 128
		cr[i] = 128
	}
	return &YCbCr{W: w, H: h, Y: make([]float32, n), Cb: cb, Cr: cr}
}

// Luma returns the Y plane as an imgx.Gray image sharing memory with p.
func (p *YCbCr) Luma() *imgx.Gray {
	return &imgx.Gray{W: p.W, H: p.H, Pix: p.Y}
}

// RGBToYCbCr converts an RGB image to YCbCr using standard ITU-R BT.601 coefficients.
func RGBToYCbCr(c *imgx.RGB) *YCbCr {
	out := NewYCbCr(c.W, c.H)
	for i := range c.R {
		out.Y[i], out.Cb[i], out.Cr[i] = PixelToYCbCr(c.R[i], c.G[i], c.B[i])
	}
	return out
}

// YCbCrToRGB converts a YCbCr image back to RGB.
func YCbCrToRGB(p *YCbCr) *imgx.RGB {
	out := imgx.NewRGB(p.W, p.H)
	for i := range p.Y {
		out.R[i], out.G[i], out.B[i] = PixelToRGBFromYCbCr(p.Y[i], p.Cb[i], p.Cr[i])
	}
	return out
}

// PixelToYCbCr converts one RGB pixel (in [0, 255]) to YCbCr:
//
//	Y  =  0.299000·R + 0.587000·G + 0.114000·B
//	Cb = -0.168736·R - 0.331264·G + 0.500000·B + 128
//	Cr =  0.500000·R - 0.418688·G - 0.081312·B + 128
func PixelToYCbCr(r, g, b float32) (y, cb, cr float32) {
	y = 0.299*r + 0.587*g + 0.114*b
	cb = -0.168736*r - 0.331264*g + 0.5*b + 128
	cr = 0.5*r - 0.418688*g - 0.081312*b + 128
	return y, cb, cr
}

// PixelToRGBFromYCbCr converts one YCbCr pixel back to clamped [0, 255] RGB:
//
//	R = Y + 1.402000·(Cr - 128)
//	G = Y - 0.344136·(Cb - 128) - 0.714136·(Cr - 128)
//	B = Y + 1.772000·(Cb - 128)
func PixelToRGBFromYCbCr(y, cb, cr float32) (r, g, b float32) {
	cbr := cb - 128
	crr := cr - 128

	r = y + 1.402*crr
	g = y - 0.344136*cbr - 0.714136*crr
	b = y + 1.772*cbr

	return imgx.Clamp255(r), imgx.Clamp255(g), imgx.Clamp255(b)
}
