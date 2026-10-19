package imgx

import (
	"errors"
	"fmt"
	"math"
)

// ErrSizeMismatch is returned when two images must have the same size but do not.
var ErrSizeMismatch = errors.New("imgx: image sizes differ")

// Clamp255 limits v to [0, 255].
func Clamp255(v float32) float32 {
	switch {
	case v < 0:
		return 0
	case v > 255:
		return 255
	default:
		return v
	}
}

// ToByte rounds v to the nearest integer and clamps it to [0, 255].
func ToByte(v float32) uint8 {
	return uint8(math.Round(float64(Clamp255(v))))
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func mustPositive(w, h int) {
	if w <= 0 || h <= 0 {
		panic(fmt.Sprintf("imgx: invalid image size %dx%d", w, h))
	}
}
