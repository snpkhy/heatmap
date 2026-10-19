package metrics

import (
	"fmt"
	"math"

	"heatmap/internal/imgx"
)

// MaxValue is the peak pixel value used by PSNR and the SSIM constants.
const MaxValue = 255.0

// MSE returns the mean squared error between two images of equal size:
//
//	MSE = (1/N) · Σ (a_i − b_i)²
func MSE(a, b *imgx.Gray) (float64, error) {
	return MSEMasked(a, b, nil)
}

// MSEMasked is MSE computed only over pixels where valid[i] is true.
// A nil valid slice means all pixels count. It returns NaN when no pixel is valid.
func MSEMasked(a, b *imgx.Gray, valid []bool) (float64, error) {
	if err := checkInputs(a, b, valid); err != nil {
		return 0, err
	}
	var sum float64
	n := 0
	for i := range a.Pix {
		if valid != nil && !valid[i] {
			continue
		}
		d := float64(a.Pix[i]) - float64(b.Pix[i])
		sum += d * d
		n++
	}
	if n == 0 {
		return math.NaN(), nil
	}
	return sum / float64(n), nil
}

// PSNR converts an MSE value to peak signal-to-noise ratio in decibels:
//
//	PSNR = 10 · log10(MAX² / MSE),  MAX = 255
//
// Identical images (MSE = 0) give +Inf.
func PSNR(mse float64) float64 {
	if mse == 0 {
		return math.Inf(1)
	}
	return 10 * math.Log10(MaxValue*MaxValue/mse)
}

func checkInputs(a, b *imgx.Gray, valid []bool) error {
	if err := imgx.CheckSameSize(a, b); err != nil {
		return err
	}
	if valid != nil && len(valid) != len(a.Pix) {
		return fmt.Errorf("metrics: valid mask has %d entries, image has %d pixels", len(valid), len(a.Pix))
	}
	return nil
}
