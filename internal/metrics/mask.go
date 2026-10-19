package metrics

import (
	"math"

	"heatmap/internal/imgx"
)

// CDnet 2014 ground-truth labels.
const (
	GTStatic   = 0   // static background
	GTShadow   = 50  // hard shadow, counted as background
	GTOutside  = 85  // outside the region of interest, ignored
	GTUnknown  = 170 // unknown motion (object boundaries), ignored
	GTMotion   = 255 // moving object
	maskThresh = 128 // a predicted mask pixel ≥ 128 is foreground
)

// Confusion holds pixel counts of a predicted mask against ground truth.
type Confusion struct {
	TP, FP, FN, TN int
}

// CompareMask counts true/false positives/negatives of a binary predicted
// mask (foreground ≥ 128) against a CDnet ground-truth mask. Ground-truth
// pixels labelled 85 (outside ROI) or 170 (unknown) are ignored; 50 (shadow)
// counts as background.
func CompareMask(pred, gt *imgx.Gray) (Confusion, error) {
	var c Confusion
	if err := imgx.CheckSameSize(pred, gt); err != nil {
		return c, err
	}
	for i, g := range gt.Pix {
		label := imgx.ToByte(g)
		if label == GTOutside || label == GTUnknown {
			continue
		}
		predFG := pred.Pix[i] >= maskThresh
		gtFG := label == GTMotion
		switch {
		case predFG && gtFG:
			c.TP++
		case predFG && !gtFG:
			c.FP++
		case !predFG && gtFG:
			c.FN++
		default:
			c.TN++
		}
	}
	return c, nil
}

// GTValid returns, for every pixel, whether the ground truth is usable for
// metrics, i.e. not labelled 85 (outside ROI) or 170 (unknown).
func GTValid(gt *imgx.Gray) []bool {
	valid := make([]bool, len(gt.Pix))
	for i, g := range gt.Pix {
		label := imgx.ToByte(g)
		valid[i] = label != GTOutside && label != GTUnknown
	}
	return valid
}

// GTBinary converts a CDnet ground-truth mask to a binary mask:
// 255 where the label is motion, 0 everywhere else.
func GTBinary(gt *imgx.Gray) *imgx.Gray {
	out := imgx.NewGray(gt.W, gt.H)
	for i, g := range gt.Pix {
		if imgx.ToByte(g) == GTMotion {
			out.Pix[i] = 255
		}
	}
	return out
}

// Add returns the element-wise sum of two confusion matrices,
// used to accumulate counts over a whole sequence.
func (c Confusion) Add(o Confusion) Confusion {
	return Confusion{TP: c.TP + o.TP, FP: c.FP + o.FP, FN: c.FN + o.FN, TN: c.TN + o.TN}
}

// Precision = TP / (TP + FP). NaN when nothing was predicted as foreground.
func (c Confusion) Precision() float64 {
	return ratio(c.TP, c.TP+c.FP)
}

// Recall = TP / (TP + FN). NaN when the ground truth has no foreground.
func (c Confusion) Recall() float64 {
	return ratio(c.TP, c.TP+c.FN)
}

// F1 = 2·TP / (2·TP + FP + FN), the harmonic mean of precision and recall.
// NaN when both the prediction and the ground truth are empty.
func (c Confusion) F1() float64 {
	return ratio(2*c.TP, 2*c.TP+c.FP+c.FN)
}

// IoU = TP / (TP + FP + FN), intersection over union of the foregrounds.
// NaN when both the prediction and the ground truth are empty.
func (c Confusion) IoU() float64 {
	return ratio(c.TP, c.TP+c.FP+c.FN)
}

func ratio(num, den int) float64 {
	if den == 0 {
		return math.NaN()
	}
	return float64(num) / float64(den)
}
