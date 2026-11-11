package filtering

import (
	"fmt"

	"heatmap/internal/imgx"
)

// Median replaces every pixel with the median of its k×k neighbourhood.
// It removes impulse (salt-and-pepper) noise while keeping edges sharp,
// which matters for the edge detection that follows it in pipeline p3.
//
// k must be odd and positive; k = 1 returns an unchanged copy.
// Pixels outside the image replicate the nearest border pixel.
func Median(src *imgx.Gray, k int) (*imgx.Gray, error) {
	if k < 1 || k%2 == 0 {
		return nil, fmt.Errorf("filtering: median window size must be odd and positive, got %d", k)
	}
	r := k / 2
	dst := imgx.NewGray(src.W, src.H)
	window := make([]float32, k*k)

	for y := 0; y < src.H; y++ {
		for x := 0; x < src.W; x++ {
			n := 0
			for dy := -r; dy <= r; dy++ {
				for dx := -r; dx <= r; dx++ {
					window[n] = src.AtClamped(x+dx, y+dy)
					n++
				}
			}
			insertionSort(window)
			dst.Set(x, y, window[len(window)/2])
		}
	}
	return dst, nil
}

// insertionSort sorts v in ascending order. Median windows are small
// (9, 25 or 49 values), where insertion sort is simple and fast enough.
func insertionSort(v []float32) {
	for i := 1; i < len(v); i++ {
		cur := v[i]
		j := i - 1
		for j >= 0 && v[j] > cur {
			v[j+1] = v[j]
			j--
		}
		v[j+1] = cur
	}
}
