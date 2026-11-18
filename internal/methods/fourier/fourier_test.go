package fourier

import (
	"math"
	"math/cmplx"
	"math/rand/v2"
	"testing"

	"heatmap/internal/imgx"
)

// naiveDFT is the direct O(N²) definition, used as the reference.
func naiveDFT(x []complex128) []complex128 {
	n := len(x)
	out := make([]complex128, n)
	for k := range n {
		for j := range n {
			out[k] += x[j] * cmplx.Rect(1, -2*math.Pi*float64(k*j)/float64(n))
		}
	}
	return out
}

func randomComplex(rng *rand.Rand, n int) []complex128 {
	x := make([]complex128, n)
	for i := range x {
		x[i] = complex(rng.Float64()*2-1, rng.Float64()*2-1)
	}
	return x
}

func TestFFTMatchesDirectDFT(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	for _, n := range []int{1, 2, 4, 8, 64} {
		x := randomComplex(rng, n)
		want := naiveDFT(x)
		got := append([]complex128(nil), x...)
		if err := FFT(got); err != nil {
			t.Fatal(err)
		}
		for k := range n {
			if cmplx.Abs(got[k]-want[k]) > 1e-9 {
				t.Fatalf("n=%d: X[%d] = %v, want %v", n, k, got[k], want[k])
			}
		}
	}
}

func TestIFFTInvertsFFT(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 4))
	x := randomComplex(rng, 128)
	y := append([]complex128(nil), x...)
	if err := FFT(y); err != nil {
		t.Fatal(err)
	}
	if err := IFFT(y); err != nil {
		t.Fatal(err)
	}
	for i := range x {
		if cmplx.Abs(x[i]-y[i]) > 1e-12 {
			t.Fatalf("x[%d] = %v after round trip, want %v", i, y[i], x[i])
		}
	}
}

func TestFFTParseval(t *testing.T) {
	// Σ|x|² = (1/N)·Σ|X|²
	rng := rand.New(rand.NewPCG(5, 6))
	x := randomComplex(rng, 256)
	var et, ef float64
	for _, v := range x {
		et += real(v)*real(v) + imag(v)*imag(v)
	}
	if err := FFT(x); err != nil {
		t.Fatal(err)
	}
	for _, v := range x {
		ef += real(v)*real(v) + imag(v)*imag(v)
	}
	if math.Abs(et-ef/256) > 1e-9 {
		t.Errorf("energy %v in time, %v in frequency", et, ef/256)
	}
}

func TestFFTRejectsNonPowerOfTwo(t *testing.T) {
	for _, n := range []int{0, 3, 6, 100} {
		if err := FFT(make([]complex128, n)); err == nil {
			t.Errorf("n=%d: want error", n)
		}
	}
	if err := FFT2D(make([]complex128, 12), 4, 3); err == nil {
		t.Error("4x3: want error")
	}
	if err := FFT2D(make([]complex128, 7), 4, 2); err == nil {
		t.Error("length mismatch: want error")
	}
}

func TestFFT2DSingleFrequency(t *testing.T) {
	// cos(2π·2x/8) has all its energy at u = ±2, v = 0.
	const w, h = 8, 4
	data := make([]complex128, w*h)
	for y := range h {
		for x := range w {
			data[y*w+x] = complex(math.Cos(2*math.Pi*2*float64(x)/w), 0)
		}
	}
	if err := FFT2D(data, w, h); err != nil {
		t.Fatal(err)
	}
	for v := range h {
		for u := range w {
			want := 0.0
			if v == 0 && (u == 2 || u == w-2) {
				want = w * h / 2
			}
			if math.Abs(cmplx.Abs(data[v*w+u])-want) > 1e-9 {
				t.Errorf("|F(%d,%d)| = %v, want %v", u, v, cmplx.Abs(data[v*w+u]), want)
			}
		}
	}
}

func TestPow2Helpers(t *testing.T) {
	for n, want := range map[int]int{0: 1, 1: 1, 2: 2, 3: 4, 5: 8, 320: 512, 512: 512} {
		if got := NextPow2(n); got != want {
			t.Errorf("NextPow2(%d) = %d, want %d", n, got, want)
		}
	}
	if IsPow2(0) || IsPow2(6) || !IsPow2(1) || !IsPow2(1024) {
		t.Error("IsPow2 is wrong")
	}
}

func TestReflect(t *testing.T) {
	// n = 3: … 1 0 | 0 1 2 | 2 1 0 | 0 …
	want := map[int]int{-3: 2, -2: 1, -1: 0, 0: 0, 2: 2, 3: 2, 4: 1, 5: 0, 6: 0, 9: 2}
	for i, w := range want {
		if got := reflect(i, 3); got != w {
			t.Errorf("reflect(%d, 3) = %d, want %d", i, got, w)
		}
	}
}

func flat(w, h int, v float32) *imgx.Gray {
	g := imgx.NewGray(w, h)
	for i := range g.Pix {
		g.Pix[i] = v
	}
	return g
}

func TestLowPassKeepsFlatImage(t *testing.T) {
	// The zero frequency has gain 1, and mirrored padding of a flat image is flat.
	src := flat(37, 23, 120)
	for _, f := range []Filter{
		{Shape: Gaussian, Cutoff: 0.1},
		{Shape: Butterworth, Cutoff: 0.1, Order: 2},
		{Shape: Ideal, Cutoff: 0.1},
	} {
		got, err := f.Apply(src)
		if err != nil {
			t.Fatal(err)
		}
		if got.W != src.W || got.H != src.H {
			t.Fatalf("%v: size %dx%d", f.Shape, got.W, got.H)
		}
		for i, v := range got.Pix {
			if math.Abs(float64(v-120)) > 1e-3 {
				t.Fatalf("%v: pixel %d = %v, want 120", f.Shape, i, v)
			}
		}
	}
}

func TestHighPassRemovesFlatImage(t *testing.T) {
	got, err := Filter{Shape: Gaussian, Cutoff: 0.1, HighPass: true}.Apply(flat(16, 16, 200))
	if err != nil {
		t.Fatal(err)
	}
	for i, v := range got.Pix {
		if math.Abs(float64(v)) > 1e-3 {
			t.Fatalf("pixel %d = %v, want 0", i, v)
		}
	}
}

// stripes returns a w×h image of vertical cosine stripes with the given
// frequency in cycles per pixel, around 128 with amplitude 100.
func stripes(w, h int, freq float64) *imgx.Gray {
	g := imgx.NewGray(w, h)
	for y := range h {
		for x := range w {
			g.Set(x, y, float32(128+100*math.Cos(2*math.Pi*freq*float64(x))))
		}
	}
	return g
}

func amplitude(g *imgx.Gray) float64 {
	// Ignore the outer columns, where the mirrored padding is not periodic.
	var lo, hi float32 = 255, 0
	for y := range g.H {
		for x := 8; x < g.W-8; x++ {
			lo, hi = min(lo, g.At(x, y)), max(hi, g.At(x, y))
		}
	}
	return float64(hi-lo) / 2
}

func TestGaussianLowPassAttenuatesHighFrequencies(t *testing.T) {
	f := Filter{Shape: Gaussian, Cutoff: 0.08}
	slow, err := f.Apply(stripes(64, 16, 1.0/32))
	if err != nil {
		t.Fatal(err)
	}
	fast, err := f.Apply(stripes(64, 16, 0.4))
	if err != nil {
		t.Fatal(err)
	}
	// H(1/32) ≈ 0.94, H(0.4) ≈ 4e-6.
	if a := amplitude(slow); a < 85 {
		t.Errorf("slow stripes amplitude %v, want ≈ 94", a)
	}
	if a := amplitude(fast); a > 1 {
		t.Errorf("fast stripes amplitude %v, want ≈ 0", a)
	}
}

func TestLowPassReducesNoise(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 8))
	src := imgx.NewGray(48, 32)
	for i := range src.Pix {
		src.Pix[i] = float32(128 + 20*rng.NormFloat64())
	}
	got, err := Filter{Shape: Butterworth, Cutoff: 0.1, Order: 2}.Apply(src)
	if err != nil {
		t.Fatal(err)
	}
	if before, after := stddev(src), stddev(got); after > before/3 {
		t.Errorf("noise σ %v → %v, want at least 3× lower", before, after)
	}
}

func stddev(g *imgx.Gray) float64 {
	var s, s2 float64
	for _, v := range g.Pix {
		s += float64(v)
		s2 += float64(v) * float64(v)
	}
	n := float64(len(g.Pix))
	return math.Sqrt(s2/n - (s/n)*(s/n))
}

func TestFilterGain(t *testing.T) {
	g := Filter{Shape: Gaussian, Cutoff: 0.1}
	if v := g.Gain(0.1); math.Abs(v-math.Exp(-0.5)) > 1e-12 {
		t.Errorf("Gaussian H(D0) = %v, want e^-0.5", v)
	}
	b := Filter{Shape: Butterworth, Cutoff: 0.1, Order: 3}
	if v := b.Gain(0.1); math.Abs(v-0.5) > 1e-12 {
		t.Errorf("Butterworth H(D0) = %v, want 0.5", v)
	}
	i := Filter{Shape: Ideal, Cutoff: 0.1, HighPass: true}
	if i.Gain(0.05) != 0 || i.Gain(0.2) != 1 {
		t.Error("ideal high-pass gains are wrong")
	}
}

func TestFilterValidation(t *testing.T) {
	src := flat(4, 4, 0)
	for _, f := range []Filter{
		{Shape: Gaussian, Cutoff: 0},
		{Shape: Gaussian, Cutoff: 0.6},
		{Shape: Butterworth, Cutoff: 0.1, Order: 0},
		{Shape: Shape(9), Cutoff: 0.1},
	} {
		if _, err := f.Apply(src); err == nil {
			t.Errorf("%+v: want error", f)
		}
	}
}

func TestParseShape(t *testing.T) {
	for _, s := range []Shape{Gaussian, Butterworth, Ideal} {
		got, err := ParseShape(s.String())
		if err != nil || got != s {
			t.Errorf("ParseShape(%q) = %v, %v", s.String(), got, err)
		}
	}
	if _, err := ParseShape("box"); err == nil {
		t.Error("unknown shape: want error")
	}
}

func TestLogSpectrum(t *testing.T) {
	spec := LogSpectrum(stripes(30, 10, 0.25))
	if spec.W != 32 || spec.H != 16 {
		t.Fatalf("size %dx%d, want 32x16", spec.W, spec.H)
	}
	// The mean (DC) is the strongest component and sits in the centre.
	if spec.At(16, 8) != 255 {
		t.Errorf("centre = %v, want 255", spec.At(16, 8))
	}
	if z := LogSpectrum(flat(4, 4, 0)); z.At(2, 2) != 0 {
		t.Error("spectrum of a black image must be black")
	}
}
