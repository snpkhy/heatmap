package keypoints

import (
	"fmt"
	"math"
	"sort"

	"heatmap/internal/imgx"
)

// Corner is a detected corner: its pixel position and Harris response.
type Corner struct {
	X, Y     int
	Response float32
}

// HarrisConfig holds the parameters of Harris.
// Start from DefaultHarrisConfig and change what you need.
type HarrisConfig struct {
	// K is the Harris sensitivity constant in R = det M − K·(trace M)².
	// Usual values are 0.04–0.06; larger values reject more edges.
	K float32
	// Sigma is the standard deviation, in pixels, of the Gaussian window
	// that sums the gradient products around each pixel.
	Sigma float32
	// Quality keeps only corners with R ≥ Quality · max R over the image,
	// in (0, 1]. 0.01 is a common choice.
	Quality float32
	// MinResponse is an absolute floor on R. Without it a flat, noisy image
	// still yields "corners" (Quality is relative to the strongest one).
	MinResponse float32
	// MinDistance is the smallest distance in pixels between two returned
	// corners; weaker corners too close to a stronger one are dropped.
	MinDistance int
	// MaxCorners caps the number of returned corners (strongest first);
	// 0 means no limit.
	MaxCorners int
}

// DefaultHarrisConfig returns parameters suited to 320×240 surveillance frames
// with intensities in [0, 255].
func DefaultHarrisConfig() HarrisConfig {
	return HarrisConfig{
		K:           0.04,
		Sigma:       1,
		Quality:     0.01,
		MinResponse: 1e4,
		MinDistance: 5,
		MaxCorners:  500,
	}
}

func (c HarrisConfig) validate() error {
	switch {
	// Comparisons are written so that NaN fails them.
	case !(c.K > 0 && c.K < 0.25):
		return fmt.Errorf("keypoints: Harris K %v must be in (0, 0.25)", c.K)
	case !(c.Sigma > 0 && c.Sigma <= 50):
		return fmt.Errorf("keypoints: Harris Sigma %v must be in (0, 50]", c.Sigma)
	case !(c.Quality > 0 && c.Quality <= 1):
		return fmt.Errorf("keypoints: Harris Quality %v must be in (0, 1]", c.Quality)
	case !(c.MinResponse >= 0):
		return fmt.Errorf("keypoints: Harris MinResponse %v must not be negative", c.MinResponse)
	case c.MinDistance < 0 || c.MaxCorners < 0:
		return fmt.Errorf("keypoints: MinDistance %d and MaxCorners %d must not be negative",
			c.MinDistance, c.MaxCorners)
	}
	return nil
}

// Sobel kernels divided by 8, so a step of height s gives a gradient of s/2
// per pixel, close to the true derivative.
var (
	sobelX = [3][3]float32{{-1, 0, 1}, {-2, 0, 2}, {-1, 0, 1}}
	sobelY = [3][3]float32{{-1, -2, -1}, {0, 0, 0}, {1, 2, 1}}
)

// HarrisResponse returns the Harris corner response of every pixel.
//
// With image derivatives Ix, Iy (Sobel) and a Gaussian window w, the
// structure tensor at a pixel is
//
//	M = Σ w · ⎡Ix²   IxIy⎤
//	          ⎣IxIy  Iy² ⎦
//
// Its eigenvalues λ₁, λ₂ say how much the window changes when shifted in the
// two principal directions: both small on a flat area, one large on an edge,
// both large on a corner. Instead of computing them,
//
//	R = det M − k·(trace M)² = λ₁λ₂ − k(λ₁+λ₂)²
//
// is positive and large on corners, negative on edges and near 0 on flat areas.
func HarrisResponse(src *imgx.Gray, k, sigma float32) *imgx.Gray {
	n := len(src.Pix)
	ixx, iyy, ixy := make([]float32, n), make([]float32, n), make([]float32, n)
	for y := range src.H {
		for x := range src.W {
			var gx, gy float32
			for ky := range 3 {
				for kx := range 3 {
					v := src.AtClamped(x+kx-1, y+ky-1)
					gx += sobelX[ky][kx] * v
					gy += sobelY[ky][kx] * v
				}
			}
			gx, gy = gx/8, gy/8
			i := y*src.W + x
			ixx[i], iyy[i], ixy[i] = gx*gx, gy*gy, gx*gy
		}
	}
	kernel := gaussianKernel(sigma)
	sxx := gaussianBlur(ixx, src.W, src.H, kernel)
	syy := gaussianBlur(iyy, src.W, src.H, kernel)
	sxy := gaussianBlur(ixy, src.W, src.H, kernel)

	dst := imgx.NewGray(src.W, src.H)
	for i := range dst.Pix {
		det := sxx[i]*syy[i] - sxy[i]*sxy[i]
		tr := sxx[i] + syy[i]
		dst.Pix[i] = det - k*tr*tr
	}
	return dst
}

// Harris detects corners:
//
//  1. compute the response R of every pixel (HarrisResponse);
//  2. keep pixels that are local maxima of R in their 3×3 neighbourhood and
//     have R ≥ max(MinResponse, Quality · max R);
//  3. go through them from the strongest and drop every one closer than
//     MinDistance to a corner already kept (non-maximum suppression);
//  4. stop after MaxCorners corners.
//
// Pixels on the 1-pixel image border are never reported: their gradients
// are computed from replicated pixels.
func Harris(src *imgx.Gray, cfg HarrisConfig) ([]Corner, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	r := HarrisResponse(src, cfg.K, cfg.Sigma)
	var peak float32
	for _, v := range r.Pix {
		peak = max(peak, v)
	}
	limit := max(cfg.MinResponse, cfg.Quality*peak)
	if peak <= 0 || peak < limit {
		return nil, nil
	}

	var cands []Corner
	for y := 1; y < src.H-1; y++ {
		for x := 1; x < src.W-1; x++ {
			v := r.At(x, y)
			if v >= limit && isLocalMax(r, x, y) {
				cands = append(cands, Corner{X: x, Y: y, Response: v})
			}
		}
	}
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].Response > cands[j].Response })

	d2 := cfg.MinDistance * cfg.MinDistance
	var out []Corner
	for _, c := range cands {
		if cfg.MaxCorners > 0 && len(out) == cfg.MaxCorners {
			break
		}
		keep := true
		for _, o := range out {
			dx, dy := c.X-o.X, c.Y-o.Y
			if dx*dx+dy*dy < d2 {
				keep = false
				break
			}
		}
		if keep {
			out = append(out, c)
		}
	}
	return out, nil
}

// isLocalMax reports whether (x, y) is not smaller than any of its 8
// neighbours. On a plateau the first pixel in raster order wins.
func isLocalMax(r *imgx.Gray, x, y int) bool {
	v := r.At(x, y)
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			n := r.At(x+dx, y+dy)
			// Neighbours visited earlier in raster order must be strictly smaller.
			if n > v || (n == v && (dy < 0 || (dy == 0 && dx < 0))) {
				return false
			}
		}
	}
	return true
}

// gaussianKernel returns a normalised 1-D Gaussian of radius ⌈3σ⌉.
func gaussianKernel(sigma float32) []float32 {
	radius := int(math.Ceil(float64(3 * sigma)))
	k := make([]float32, 2*radius+1)
	var sum float32
	for i := range k {
		d := float64(i - radius)
		k[i] = float32(math.Exp(-d * d / (2 * float64(sigma*sigma))))
		sum += k[i]
	}
	for i := range k {
		k[i] /= sum
	}
	return k
}

// gaussianBlur convolves a w×h plane with the separable kernel, first along
// rows and then along columns, replicating the border.
func gaussianBlur(src []float32, w, h int, k []float32) []float32 {
	r := len(k) / 2
	tmp := make([]float32, len(src))
	for y := range h {
		row := src[y*w : (y+1)*w]
		for x := range w {
			var s float32
			for i, kv := range k {
				s += kv * row[min(max(x+i-r, 0), w-1)]
			}
			tmp[y*w+x] = s
		}
	}
	dst := make([]float32, len(src))
	for y := range h {
		for x := range w {
			var s float32
			for i, kv := range k {
				s += kv * tmp[min(max(y+i-r, 0), h-1)*w+x]
			}
			dst[y*w+x] = s
		}
	}
	return dst
}
