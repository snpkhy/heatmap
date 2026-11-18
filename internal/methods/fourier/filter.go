package fourier

import (
	"fmt"
	"math"

	"heatmap/internal/imgx"
)

// Shape is the transfer function profile of a frequency-domain filter.
type Shape int

const (
	// Gaussian has H(D) = exp(−D² / 2D₀²). Smooth, no ringing.
	Gaussian Shape = iota
	// Butterworth has H(D) = 1 / (1 + (D/D₀)^(2n)). Sharper cut-off than
	// Gaussian; higher orders ring more.
	Butterworth
	// Ideal has H(D) = 1 for D ≤ D₀, 0 otherwise. The sharpest cut-off, but
	// rings strongly (Gibbs phenomenon); kept mostly for comparison.
	Ideal
)

// String returns the name used in configs.
func (s Shape) String() string {
	switch s {
	case Gaussian:
		return "gaussian"
	case Butterworth:
		return "butterworth"
	case Ideal:
		return "ideal"
	default:
		return fmt.Sprintf("Shape(%d)", int(s))
	}
}

// ParseShape converts a config name ("gaussian", "butterworth", "ideal") into a Shape.
func ParseShape(name string) (Shape, error) {
	for _, s := range []Shape{Gaussian, Butterworth, Ideal} {
		if s.String() == name {
			return s, nil
		}
	}
	return 0, fmt.Errorf("fourier: unknown filter shape %q", name)
}

// Filter is a frequency-domain filter: the image is transformed with the
// FFT, every frequency is multiplied by the transfer function H, and the
// result is transformed back.
//
// D is the distance of a frequency from the origin in cycles per pixel
// (0 is the mean brightness, 0.5 the finest pattern a pixel grid can hold).
// A low-pass filter keeps D ≲ Cutoff, a high-pass filter uses 1 − H and
// keeps the rest.
//
// In p5 a low-pass filter removes sensor noise and fine flicker before frame
// differencing: noise spreads evenly over all frequencies, while the
// silhouettes of people and cars sit mostly in the low ones.
type Filter struct {
	// Shape is the transfer function profile.
	Shape Shape
	// Cutoff is D₀ in cycles per pixel, in (0, 0.5]. For a Gaussian low-pass
	// it corresponds to a spatial Gaussian blur with σ = 1/(2π·Cutoff) pixels
	// (0.1 ≈ 1.6 px, 0.05 ≈ 3.2 px).
	Cutoff float64
	// Order is the Butterworth order n ≥ 1; ignored by other shapes.
	Order int
	// HighPass inverts the filter (1 − H). The result is then centred on 0
	// and may be negative; it is not clamped.
	HighPass bool
}

func (f Filter) validate() error {
	if f.Cutoff <= 0 || f.Cutoff > 0.5 {
		return fmt.Errorf("fourier: cutoff %v must be in (0, 0.5] cycles/pixel", f.Cutoff)
	}
	switch f.Shape {
	case Gaussian, Ideal:
	case Butterworth:
		if f.Order < 1 {
			return fmt.Errorf("fourier: Butterworth order %d must be at least 1", f.Order)
		}
	default:
		return fmt.Errorf("fourier: unknown filter shape %d", f.Shape)
	}
	return nil
}

// Gain returns the transfer function H at distance d (cycles per pixel) from
// the zero frequency.
func (f Filter) Gain(d float64) float64 {
	var h float64
	switch f.Shape {
	case Gaussian:
		h = math.Exp(-d * d / (2 * f.Cutoff * f.Cutoff))
	case Butterworth:
		h = 1 / (1 + math.Pow(d/f.Cutoff, float64(2*f.Order)))
	case Ideal:
		if d <= f.Cutoff {
			h = 1
		}
	}
	if f.HighPass {
		return 1 - h
	}
	return h
}

// Apply filters one image and returns a new one of the same size.
//
// The FFT treats the image as periodic, so the right border would be blurred
// into the left one. To avoid this the image is first padded to power-of-two
// sides with a margin of mirrored pixels (…c b a | a b c … x y z | z y x…);
// the padding is cut off after the inverse transform.
func (f Filter) Apply(src *imgx.Gray) (*imgx.Gray, error) {
	if err := f.validate(); err != nil {
		return nil, err
	}
	m := padMargin(f.Cutoff)
	pw, ph := NextPow2(src.W+2*m), NextPow2(src.H+2*m)

	data := make([]complex128, pw*ph)
	for y := range ph {
		sy := reflect(y-m, src.H)
		for x := range pw {
			data[y*pw+x] = complex(float64(src.Pix[sy*src.W+reflect(x-m, src.W)]), 0)
		}
	}
	if err := FFT2D(data, pw, ph); err != nil {
		return nil, err
	}
	freqU := axisFreqs(pw)
	freqV := axisFreqs(ph)
	for v := range ph {
		for u := range pw {
			d := math.Hypot(freqU[u], freqV[v])
			data[v*pw+u] *= complex(f.Gain(d), 0)
		}
	}
	if err := IFFT2D(data, pw, ph); err != nil {
		return nil, err
	}

	dst := imgx.NewGray(src.W, src.H)
	for y := range src.H {
		for x := range src.W {
			dst.Pix[y*src.W+x] = float32(real(data[(y+m)*pw+x+m]))
		}
	}
	return dst, nil
}

// LogSpectrum returns the centred log-magnitude spectrum log(1 + |F|) of an
// image, scaled to [0, 255] — the usual way to look at a Fourier transform.
// The zero frequency is in the middle. The image is zero-padded to power-of-two
// sides, so the result may be larger than src.
func LogSpectrum(src *imgx.Gray) *imgx.Gray {
	pw, ph := NextPow2(src.W), NextPow2(src.H)
	data := make([]complex128, pw*ph)
	for y := range src.H {
		for x := range src.W {
			data[y*pw+x] = complex(float64(src.Pix[y*src.W+x]), 0)
		}
	}
	_ = FFT2D(data, pw, ph) // sizes are powers of two by construction

	dst := imgx.NewGray(pw, ph)
	var peak float64
	logs := make([]float64, len(data))
	for i, c := range data {
		logs[i] = math.Log1p(math.Hypot(real(c), imag(c)))
		peak = max(peak, logs[i])
	}
	if peak == 0 {
		return dst
	}
	for v := range ph {
		for u := range pw {
			// Shift by half a period so the zero frequency lands in the centre.
			x, y := (u+pw/2)%pw, (v+ph/2)%ph
			dst.Pix[y*pw+x] = float32(255 * logs[v*pw+u] / peak)
		}
	}
	return dst
}

// axisFreqs returns the signed frequency of every index along an axis of
// length n, in cycles per pixel: 0, 1/n, …, (n/2−1)/n, −1/2, …, −1/n.
func axisFreqs(n int) []float64 {
	f := make([]float64, n)
	for i := range n {
		k := i
		if i >= n/2 {
			k = i - n
		}
		f[i] = float64(k) / float64(n)
	}
	return f
}

// padMargin returns the mirrored margin in pixels: three spatial σ of the
// equivalent Gaussian blur, so that wrap-around stays inside the padding,
// but at least 8 and at most 64 pixels.
func padMargin(cutoff float64) int {
	sigma := 1 / (2 * math.Pi * cutoff)
	return min(max(int(math.Ceil(3*sigma)), 8), 64)
}

// reflect maps any index onto [0, n) by mirroring at the borders with the
// edge pixel repeated (−1 → 0, n → n−1), period 2n.
func reflect(i, n int) int {
	period := 2 * n
	i %= period
	if i < 0 {
		i += period
	}
	if i >= n {
		i = period - 1 - i
	}
	return i
}
