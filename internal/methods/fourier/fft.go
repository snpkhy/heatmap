package fourier

import (
	"fmt"
	"math"
	"math/cmplx"
)

// FFT computes the discrete Fourier transform of x in place with the
// iterative radix-2 Cooley–Tukey algorithm:
//
//	X[k] = Σₙ x[n]·e^(−2πi·kn/N),   k = 0 … N−1
//
// N = len(x) must be a power of two. The cost is O(N log N) instead of the
// O(N²) of the direct sum: the input is reordered by bit-reversed index and
// then log₂N passes of "butterflies" combine transforms of length 1, 2, 4, …
func FFT(x []complex128) error {
	return fft(x, false)
}

// IFFT computes the inverse transform of x in place, including the 1/N
// factor, so IFFT(FFT(x)) == x up to rounding. len(x) must be a power of two.
func IFFT(x []complex128) error {
	return fft(x, true)
}

func fft(x []complex128, inverse bool) error {
	n := len(x)
	if !IsPow2(n) {
		return fmt.Errorf("fourier: FFT length %d is not a power of two", n)
	}
	bitReverse(x)

	sign := -1.0
	if inverse {
		sign = 1
	}
	for size := 2; size <= n; size <<= 1 {
		half := size / 2
		// w = e^(∓2πi/size) is the primitive root of unity for this pass.
		w := cmplx.Rect(1, sign*2*math.Pi/float64(size))
		for start := 0; start < n; start += size {
			tw := complex(1, 0)
			for k := range half {
				a := x[start+k]
				b := x[start+k+half] * tw
				x[start+k] = a + b
				x[start+k+half] = a - b
				tw *= w
			}
		}
	}
	if inverse {
		scale := complex(1/float64(n), 0)
		for i := range x {
			x[i] *= scale
		}
	}
	return nil
}

// bitReverse swaps x[i] with x[rev(i)], where rev reverses the log₂N bits of i.
func bitReverse(x []complex128) {
	n := len(x)
	j := 0
	for i := 1; i < n; i++ {
		bit := n >> 1
		for ; j&bit != 0; bit >>= 1 {
			j ^= bit
		}
		j |= bit
		if i < j {
			x[i], x[j] = x[j], x[i]
		}
	}
}

// FFT2D transforms a w×h complex matrix (row by row, element (u, v) at
// v*w+u) in place: first every row, then every column. The 2-D DFT is
// separable, so this equals the direct double sum. w and h must be powers of two.
func FFT2D(data []complex128, w, h int) error {
	return fft2D(data, w, h, false)
}

// IFFT2D is the inverse of FFT2D, including the 1/(w·h) factor.
func IFFT2D(data []complex128, w, h int) error {
	return fft2D(data, w, h, true)
}

func fft2D(data []complex128, w, h int, inverse bool) error {
	if len(data) != w*h {
		return fmt.Errorf("fourier: %d values do not form a %dx%d matrix", len(data), w, h)
	}
	if !IsPow2(w) || !IsPow2(h) {
		return fmt.Errorf("fourier: 2-D FFT size %dx%d is not a power of two", w, h)
	}
	for y := range h {
		if err := fft(data[y*w:(y+1)*w], inverse); err != nil {
			return err
		}
	}
	col := make([]complex128, h)
	for x := range w {
		for y := range h {
			col[y] = data[y*w+x]
		}
		if err := fft(col, inverse); err != nil {
			return err
		}
		for y := range h {
			data[y*w+x] = col[y]
		}
	}
	return nil
}

// IsPow2 reports whether n is a positive power of two.
func IsPow2(n int) bool {
	return n > 0 && n&(n-1) == 0
}

// NextPow2 returns the smallest power of two that is ≥ n (1 for n ≤ 1).
func NextPow2(n int) int {
	p := 1
	for p < n {
		p <<= 1
	}
	return p
}
