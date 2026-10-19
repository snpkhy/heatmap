package metrics

import (
	"math"
	"time"
)

// Milliseconds converts a duration to fractional milliseconds for reports.
func Milliseconds(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}

// FPS returns frames per second for the given number of frames processed in d.
// It returns NaN when d is zero.
func FPS(frames int, d time.Duration) float64 {
	if d <= 0 {
		return math.NaN()
	}
	return float64(frames) / d.Seconds()
}
