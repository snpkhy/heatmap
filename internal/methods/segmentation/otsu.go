package segmentation

import (
	"fmt"
	"math"

	"heatmap/internal/imgx"
)

// Otsu calculates the optimal global binarization threshold for a grayscale image
// using Otsu's method (maximizing the inter-class variance between background and foreground).
// When multiple thresholds achieve the same maximum variance (plateau), the midpoint is returned.
// Intensities are clamped to [0, 255].
func Otsu(src *imgx.Gray) float32 {
	if src.W == 0 || src.H == 0 || len(src.Pix) == 0 {
		return 0
	}

	var hist [256]int
	total := len(src.Pix)
	for _, v := range src.Pix {
		b := int(math.Round(float64(v)))
		if b < 0 {
			b = 0
		} else if b > 255 {
			b = 255
		}
		hist[b]++
	}

	var sumTotal float64
	for i := 0; i < 256; i++ {
		sumTotal += float64(i * hist[i])
	}

	var (
		sumB      float64
		wB        int
		maxVar    float64
		variances [256]float64
	)

	for t := 0; t < 256; t++ {
		wB += hist[t]
		if wB == 0 {
			continue
		}
		wF := total - wB
		if wF == 0 {
			break
		}

		sumB += float64(t * hist[t])
		mB := sumB / float64(wB)
		mF := (sumTotal - sumB) / float64(wF)

		diff := mB - mF
		variance := float64(wB) * float64(wF) * diff * diff
		variances[t] = variance

		if variance > maxVar {
			maxVar = variance
		}
	}

	if maxVar <= 0 {
		return 0
	}

	var sumT, count int
	for t := 0; t < 256; t++ {
		if math.Abs(variances[t]-maxVar) <= 1e-4 {
			sumT += t
			count++
		}
	}

	return float32(sumT) / float32(count)
}

// OtsuThreshold segments src into a binary mask (0 or 255) using Otsu's threshold.
// Pixels > threshold become 255 (foreground), others become 0 (background).
func OtsuThreshold(src *imgx.Gray) (*imgx.Gray, float32, error) {
	if src == nil {
		return nil, 0, fmt.Errorf("segmentation: src image cannot be nil")
	}
	t := Otsu(src)
	dst := imgx.NewGray(src.W, src.H)
	for i, v := range src.Pix {
		if v > t {
			dst.Pix[i] = MaskForeground
		} else {
			dst.Pix[i] = MaskBackground
		}
	}
	return dst, t, nil
}
