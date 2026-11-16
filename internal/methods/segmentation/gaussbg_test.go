package segmentation

import (
	"errors"
	"math"
	"math/rand/v2"
	"testing"

	"heatmap/internal/imgx"
)

func flat(w, h int, v float32) *imgx.Gray {
	g := imgx.NewGray(w, h)
	for i := range g.Pix {
		g.Pix[i] = v
	}
	return g
}

func countFG(m *imgx.Gray) int {
	n := 0
	for _, v := range m.Pix {
		if v >= 128 {
			n++
		}
	}
	return n
}

func mustModel(t *testing.T, cfg GaussianBGConfig) *GaussianBG {
	t.Helper()
	m, err := NewGaussianBG(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func mustApply(t *testing.T, m *GaussianBG, channels ...*imgx.Gray) *imgx.Gray {
	t.Helper()
	mask, err := m.Apply(channels...)
	if err != nil {
		t.Fatal(err)
	}
	return mask
}

func TestGaussianBGConfigValidation(t *testing.T) {
	bad := []func(*GaussianBGConfig){
		func(c *GaussianBGConfig) { c.Alpha = 0 },
		func(c *GaussianBGConfig) { c.Alpha = 1.5 },
		func(c *GaussianBGConfig) { c.FGAlpha = c.Alpha * 2 },
		func(c *GaussianBGConfig) { c.K = 0 },
		func(c *GaussianBGConfig) { c.MinVar = 0 },
		func(c *GaussianBGConfig) { c.InitVar = c.MinVar / 2 },
		func(c *GaussianBGConfig) { c.Warmup = -1 },
		func(c *GaussianBGConfig) { c.ShadowChannel = -2 },
		func(c *GaussianBGConfig) { c.ShadowChannel, c.ShadowLow = 0, 1 },
		func(c *GaussianBGConfig) { c.ShadowChannel, c.ShadowK = 0, c.K-1 },
		func(c *GaussianBGConfig) { c.RatioChannel = 1 }, // without ShadowChannel
		func(c *GaussianBGConfig) { c.ShadowChannel, c.RatioChannel = 1, 1 },
	}
	for i, mutate := range bad {
		cfg := DefaultGaussianBGConfig()
		mutate(&cfg)
		if _, err := NewGaussianBG(cfg); err == nil {
			t.Errorf("bad config #%d was accepted: %+v", i, cfg)
		}
	}
	if _, err := NewGaussianBG(DefaultGaussianBGConfig()); err != nil {
		t.Errorf("default config rejected: %v", err)
	}
}

func TestGaussianBGFirstFrameIsBackground(t *testing.T) {
	m := mustModel(t, DefaultGaussianBGConfig())
	mask := mustApply(t, m, flat(4, 4, 100))
	if countFG(mask) != 0 || m.Frames() != 1 {
		t.Errorf("first frame: %d foreground pixels, %d frames", countFG(mask), m.Frames())
	}
	bg, err := m.Background(0)
	if err != nil || bg.Pix[0] != 100 {
		t.Errorf("background = %v, err %v; want 100", bg, err)
	}
	if _, err := m.Background(1); err == nil {
		t.Error("Background(1) on a one-channel model did not fail")
	}
}

func TestGaussianBGDetectsObject(t *testing.T) {
	cfg := DefaultGaussianBGConfig()
	cfg.Warmup = 0
	m := mustModel(t, cfg)
	mustApply(t, m, flat(4, 4, 100))

	// Threshold: K·σ = 3.5·√64 = 28. A change of 20 stays background, 40 does not.
	frame := flat(4, 4, 100)
	frame.Set(1, 1, 120)
	frame.Set(2, 2, 140)
	frame.Set(3, 3, 60)
	mask := mustApply(t, m, frame)

	if mask.At(1, 1) != MaskBackground {
		t.Error("a change within K·σ was reported as foreground")
	}
	if mask.At(2, 2) != MaskForeground || mask.At(3, 3) != MaskForeground {
		t.Error("changes beyond K·σ were not reported")
	}
	if countFG(mask) != 2 {
		t.Errorf("%d foreground pixels, want 2", countFG(mask))
	}
}

func TestGaussianBGUpdateFormulas(t *testing.T) {
	cfg := DefaultGaussianBGConfig()
	cfg.Warmup, cfg.Alpha, cfg.FGAlpha, cfg.InitVar, cfg.MinVar = 0, 0.1, 0.01, 64, 1
	m := mustModel(t, cfg)
	mustApply(t, m, flat(2, 1, 100))

	// Pixel 0 moves by 10 (background, rate 0.1), pixel 1 by 50 (foreground:
	// the mean moves at rate 0.01, the variance stays).
	frame, _ := imgx.GrayFromSlice(2, 1, []float32{110, 150})
	mustApply(t, m, frame)

	wantMean := []float32{100 + 0.1*10, 100 + 0.01*50}
	wantVar := []float32{0.9*64 + 0.1*100, 64}
	for i := range wantMean {
		if d := math.Abs(float64(m.mean[0][i] - wantMean[i])); d > 1e-3 {
			t.Errorf("mean[%d] = %v, want %v", i, m.mean[0][i], wantMean[i])
		}
		if d := math.Abs(float64(m.variance[0][i] - wantVar[i])); d > 1e-3 {
			t.Errorf("variance[%d] = %v, want %v", i, m.variance[0][i], wantVar[i])
		}
	}
}

func TestGaussianBGLearnsNoiseLevel(t *testing.T) {
	// Left half is quiet (σ = 1), right half is noisy (σ = 12). After
	// training, the model must tolerate the noise on the right without
	// raising false alarms, and its variance must reflect both levels.
	const w, h = 20, 10
	rng := rand.New(rand.NewPCG(1, 2))
	noisy := func() *imgx.Gray {
		g := imgx.NewGray(w, h)
		for y := range h {
			for x := range w {
				sigma := 1.0
				if x >= w/2 {
					sigma = 12
				}
				g.Set(x, y, float32(120+rng.NormFloat64()*sigma))
			}
		}
		return g
	}
	m := mustModel(t, DefaultGaussianBGConfig())
	for range 150 {
		mustApply(t, m, noisy())
	}
	false1 := 0
	for range 20 {
		false1 += countFG(mustApply(t, m, noisy()))
	}
	// 3.5σ leaves 0.05% of a Gaussian outside; allow for the variance still settling.
	if rate := float64(false1) / (20 * w * h); rate > 0.03 {
		t.Errorf("false alarm rate %.3f on pure noise, want < 0.03", rate)
	}
	quiet, loud := m.variance[0][0], m.variance[0][w-1]
	if quiet > 30 || loud < 70 {
		t.Errorf("variance: quiet %v, noisy %v; want about 16 (MinVar) and about 144", quiet, loud)
	}
}

func TestGaussianBGAbsorbsStoppedObject(t *testing.T) {
	cfg := DefaultGaussianBGConfig()
	cfg.Warmup, cfg.FGAlpha = 0, 0.02
	m := mustModel(t, cfg)
	mustApply(t, m, flat(1, 1, 100))

	// An object appears and never leaves. It is foreground at first and
	// becomes background once the model has adapted.
	if mask := mustApply(t, m, flat(1, 1, 200)); countFG(mask) != 1 {
		t.Fatal("a new object was not detected")
	}
	absorbed := false
	for range 400 {
		if countFG(mustApply(t, m, flat(1, 1, 200))) == 0 {
			absorbed = true
			break
		}
	}
	if !absorbed {
		t.Error("a stopped object was never absorbed into the background")
	}
}

func TestGaussianBGShadow(t *testing.T) {
	cfg := DefaultGaussianBGConfig()
	cfg.Warmup = 0
	cfg.ShadowChannel = 1 // channel 0: color feature, channel 1: brightness
	cfg.ShadowLow = 0.5
	cfg.MarkShadows = true
	m := mustModel(t, cfg)
	mustApply(t, m, flat(5, 1, 80), flat(5, 1, 200))

	color, _ := imgx.GrayFromSlice(5, 1, []float32{80, 80, 80, 80, 160})
	bright, _ := imgx.GrayFromSlice(5, 1, []float32{200, 140, 60, 250, 140})
	mask := mustApply(t, m, color, bright)

	want := []float32{
		MaskBackground, // nothing changed
		MaskShadow,     // same color, 0.7 of the brightness
		MaskForeground, // same color but 0.3 of the brightness: too dark for a shadow
		MaskForeground, // same color, brighter: a highlight, not a shadow
		MaskForeground, // darker and a different color: an object
	}
	for i := range want {
		if mask.Pix[i] != want[i] {
			t.Errorf("pixel %d = %v, want %v", i, mask.Pix[i], want[i])
		}
	}

	// Without MarkShadows the shadow is plain background.
	cfg.MarkShadows = false
	m = mustModel(t, cfg)
	mustApply(t, m, flat(5, 1, 80), flat(5, 1, 200))
	if mask := mustApply(t, m, color, bright); mask.Pix[1] != MaskBackground {
		t.Errorf("unmarked shadow = %v, want 0", mask.Pix[1])
	}
}

func TestGaussianBGShadowK(t *testing.T) {
	// A shadow lit by the sky shifts the color feature by 5σ (σ = √64 = 8).
	// With ShadowK = K = 3 that is an object; with ShadowK = 6 it is a shadow.
	// A shift of 10σ is an object either way.
	color, _ := imgx.GrayFromSlice(2, 1, []float32{80 + 40, 80 + 80})
	bright, _ := imgx.GrayFromSlice(2, 1, []float32{140, 140})
	for _, tt := range []struct {
		shadowK float32
		want    []float32
	}{
		{3, []float32{MaskForeground, MaskForeground}},
		{6, []float32{MaskShadow, MaskForeground}},
	} {
		cfg := DefaultGaussianBGConfig()
		cfg.K, cfg.Warmup, cfg.ShadowChannel, cfg.MarkShadows, cfg.ShadowK = 3, 0, 1, true, tt.shadowK
		m := mustModel(t, cfg)
		mustApply(t, m, flat(2, 1, 80), flat(2, 1, 200))
		mask := mustApply(t, m, color, bright)
		for i := range tt.want {
			if mask.Pix[i] != tt.want[i] {
				t.Errorf("ShadowK %v, pixel %d = %v, want %v", tt.shadowK, i, mask.Pix[i], tt.want[i])
			}
		}
	}
}

func TestGaussianBGRatioChannel(t *testing.T) {
	// Channel 0 is an "equalized" brightness whose ratios mean nothing;
	// channel 1 is the raw brightness, passive and used only for the ratio.
	cfg := DefaultGaussianBGConfig()
	cfg.Warmup = 0
	cfg.ShadowChannel, cfg.RatioChannel, cfg.MarkShadows = 0, 1, true
	m := mustModel(t, cfg)
	mustApply(t, m, flat(4, 1, 200), flat(4, 1, 100))

	equalized, _ := imgx.GrayFromSlice(4, 1, []float32{10, 10, 200, 200})
	raw, _ := imgx.GrayFromSlice(4, 1, []float32{70, 30, 250, 10})
	mask := mustApply(t, m, equalized, raw)

	want := []float32{
		MaskShadow,     // equalized fell to 5% but raw is 0.7 of the background
		MaskForeground, // raw is 0.3 of the background: an object
		MaskBackground, // only the passive channel changed (brighter)
		MaskBackground, // only the passive channel changed (darker)
	}
	for i := range want {
		if mask.Pix[i] != want[i] {
			t.Errorf("pixel %d = %v, want %v", i, mask.Pix[i], want[i])
		}
	}
}

func TestGaussianBGInputErrors(t *testing.T) {
	m := mustModel(t, DefaultGaussianBGConfig())
	if _, err := m.Apply(); err == nil {
		t.Error("Apply with no channels did not fail")
	}
	if _, err := m.Apply(flat(2, 2, 0), flat(3, 2, 0)); !errors.Is(err, imgx.ErrSizeMismatch) {
		t.Errorf("channels of different size: err = %v", err)
	}

	mustApply(t, m, flat(2, 2, 0))
	if _, err := m.Apply(flat(3, 3, 0)); !errors.Is(err, imgx.ErrSizeMismatch) {
		t.Errorf("frame of another size: err = %v", err)
	}
	if _, err := m.Apply(flat(2, 2, 0), flat(2, 2, 0)); err == nil {
		t.Error("a different number of channels did not fail")
	}

	m.Reset()
	if mask := mustApply(t, m, flat(3, 3, 0)); m.Frames() != 1 || countFG(mask) != 0 {
		t.Error("Reset did not return the model to its initial state")
	}

	cfg := DefaultGaussianBGConfig()
	cfg.ShadowChannel = 2
	if _, err := mustModel(t, cfg).Apply(flat(2, 2, 0)); err == nil {
		t.Error("ShadowChannel beyond the given channels did not fail")
	}
}
