package metrics

import (
	"math"

	"heatmap/internal/imgx"
)

// SSIM parameters from Wang et al. (2004).
const (
	ssimWindow = 11  // Gaussian window is 11×11
	ssimSigma  = 1.5 // with σ = 1.5
)

var (
	ssimC1 = (0.01 * MaxValue) * (0.01 * MaxValue) // C1 = (0.01·255)² = 6.5025
	ssimC2 = (0.03 * MaxValue) * (0.03 * MaxValue) // C2 = (0.03·255)² = 58.5225
	// ssimKernel is computed once; it never changes.
	ssimKernel = gaussianKernel(ssimWindow, ssimSigma)
)

// SSIM returns the mean structural similarity index of two equal-size images.
// See SSIMMasked for details.
func SSIM(a, b *imgx.Gray) (float64, error) {
	return SSIMMasked(a, b, nil)
}

// SSIMMasked returns the mean SSIM over pixels where valid[i] is true
// (nil means all pixels). For every valid pixel, local statistics are
// computed in an 11×11 Gaussian window (σ = 1.5) centred on it:
//
//	μx = Σ w·x,  σx² = Σ w·x² − μx²,  σxy = Σ w·x·y − μx·μy
//	SSIM = (2μxμy + C1)(2σxy + C2) / ((μx² + μy² + C1)(σx² + σy² + C2))
//
// Window pixels that fall outside the image or are not valid are dropped and
// the remaining weights are renormalised to sum to 1. This lets SSIM work on
// images smaller than the window (down to 1×1) and on CDnet masks with
// ignored pixels. It returns NaN when no pixel is valid.
func SSIMMasked(a, b *imgx.Gray, valid []bool) (float64, error) {
	if err := checkInputs(a, b, valid); err != nil {
		return 0, err
	}
	half := ssimWindow / 2
	var total float64
	n := 0

	for cy := 0; cy < a.H; cy++ {
		for cx := 0; cx < a.W; cx++ {
			if valid != nil && !valid[cy*a.W+cx] {
				continue
			}
			var wSum, mx, my, sxx, syy, sxy float64
			for ky := -half; ky <= half; ky++ {
				for kx := -half; kx <= half; kx++ {
					x, y := cx+kx, cy+ky
					if !a.InBounds(x, y) {
						continue
					}
					i := y*a.W + x
					if valid != nil && !valid[i] {
						continue
					}
					w := ssimKernel[(ky+half)*ssimWindow+(kx+half)]
					xv, yv := float64(a.Pix[i]), float64(b.Pix[i])
					wSum += w
					mx += w * xv
					my += w * yv
					sxx += w * xv * xv
					syy += w * yv * yv
					sxy += w * xv * yv
				}
			}
			// Renormalise so the used weights sum to 1.
			mx, my = mx/wSum, my/wSum
			varX := sxx/wSum - mx*mx
			varY := syy/wSum - my*my
			cov := sxy/wSum - mx*my

			num := (2*mx*my + ssimC1) * (2*cov + ssimC2)
			den := (mx*mx + my*my + ssimC1) * (varX + varY + ssimC2)
			total += num / den
			n++
		}
	}
	if n == 0 {
		return math.NaN(), nil
	}
	return total / float64(n), nil
}

// gaussianKernel returns a size×size Gaussian kernel (row by row) whose weights sum to 1:
//
//	w(x, y) ∝ exp(−(x² + y²) / (2σ²)),  x, y ∈ [−size/2, size/2]
func gaussianKernel(size int, sigma float64) []float64 {
	k := make([]float64, size*size)
	half := size / 2
	var sum float64
	for y := -half; y <= half; y++ {
		for x := -half; x <= half; x++ {
			v := math.Exp(-float64(x*x+y*y) / (2 * sigma * sigma))
			k[(y+half)*size+(x+half)] = v
			sum += v
		}
	}
	for i := range k {
		k[i] /= sum
	}
	return k
}
