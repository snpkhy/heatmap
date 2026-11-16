package segmentation

import (
	"errors"
	"fmt"

	"heatmap/internal/imgx"
)

// Mask values produced by the segmentation methods. They follow the CDnet
// ground-truth convention, so a mask with marked shadows can be compared
// with ground truth directly: anything below 128 counts as background.
const (
	MaskBackground = 0
	MaskShadow     = 50
	MaskForeground = 255
)

// GaussianBGConfig holds the parameters of GaussianBG.
// Start from DefaultGaussianBGConfig and change what you need.
type GaussianBGConfig struct {
	// Alpha is the learning rate for pixels classified as background, in (0, 1].
	// Larger values adapt faster to lighting changes but absorb slow objects.
	Alpha float32
	// FGAlpha is the learning rate for foreground and shadow pixels, in [0, Alpha].
	// A small non-zero value lets the model eventually absorb objects that
	// stopped, and "ghosts" of objects present in the very first frame.
	FGAlpha float32
	// K is the decision threshold in standard deviations: a channel deviates
	// when |x − μ| > K·σ.
	K float32
	// InitVar is the variance assigned to every pixel on the first frame.
	InitVar float32
	// MinVar is the lower bound of the variance. It keeps a perfectly still
	// pixel from becoming so "certain" that any noise looks like motion.
	MinVar float32
	// Warmup is the number of first frames during which all pixels are learned
	// with a plain cumulative average (rate 1/n), giving a quick initial model.
	Warmup int
	// ShadowChannel is the index of the brightness channel used for shadow
	// suppression, or −1 to disable it.
	ShadowChannel int
	// RatioChannel is the index of a passive channel on which the shadow
	// ratio x/μ is measured, or −1 to measure it on ShadowChannel itself.
	// A passive channel is tracked by the model but never votes for
	// foreground. Use it when ShadowChannel went through a non-linear
	// transform (such as histogram equalization) that destroys ratios:
	// pass the raw brightness here.
	RatioChannel int
	// ShadowLow is the darkest a shadow may be relative to the background
	// brightness, in (0, 1): a shadow has ShadowLow ≤ x/μ < 1.
	ShadowLow float32
	// ShadowK is how far, in standard deviations, the other (color) channels
	// may be from the background for a darker pixel to still be a shadow.
	// It must be at least K. With ShadowK = K a shadow must not change the
	// color at all; real shadows do shift it a little, because the shaded
	// surface is lit by the sky instead of the sun, so outdoor scenes need
	// a larger value.
	ShadowK float32
	// MarkShadows writes MaskShadow (50) for shadow pixels instead of
	// MaskBackground (0). Both count as background in the metrics.
	MarkShadows bool
}

// DefaultGaussianBGConfig returns parameters that work for 8-bit-range
// features. Shadow suppression is off; set ShadowChannel to enable it.
func DefaultGaussianBGConfig() GaussianBGConfig {
	return GaussianBGConfig{
		Alpha:         0.02,
		FGAlpha:       0.002,
		K:             3.5,
		InitVar:       64,
		MinVar:        16,
		Warmup:        20,
		ShadowChannel: -1,
		RatioChannel:  -1,
		ShadowLow:     0.5,
		ShadowK:       3.5,
	}
}

func (c GaussianBGConfig) validate() error {
	switch {
	case c.Alpha <= 0 || c.Alpha > 1:
		return fmt.Errorf("segmentation: Alpha %v must be in (0, 1]", c.Alpha)
	case c.FGAlpha < 0 || c.FGAlpha > c.Alpha:
		return fmt.Errorf("segmentation: FGAlpha %v must be in [0, Alpha]", c.FGAlpha)
	case c.K <= 0:
		return fmt.Errorf("segmentation: K %v must be positive", c.K)
	case c.MinVar <= 0 || c.InitVar < c.MinVar:
		return fmt.Errorf("segmentation: need 0 < MinVar (%v) <= InitVar (%v)", c.MinVar, c.InitVar)
	case c.Warmup < 0:
		return fmt.Errorf("segmentation: Warmup %d must not be negative", c.Warmup)
	case c.ShadowChannel < -1:
		return fmt.Errorf("segmentation: ShadowChannel %d must be -1 or a channel index", c.ShadowChannel)
	case c.RatioChannel < -1 || (c.RatioChannel >= 0 && c.ShadowChannel < 0) ||
		(c.RatioChannel >= 0 && c.RatioChannel == c.ShadowChannel):
		return fmt.Errorf("segmentation: RatioChannel %d must be -1 or a channel other than ShadowChannel %d",
			c.RatioChannel, c.ShadowChannel)
	case c.ShadowChannel >= 0 && (c.ShadowLow <= 0 || c.ShadowLow >= 1):
		return fmt.Errorf("segmentation: ShadowLow %v must be in (0, 1)", c.ShadowLow)
	case c.ShadowChannel >= 0 && c.ShadowK < c.K:
		return fmt.Errorf("segmentation: ShadowK %v must be at least K %v", c.ShadowK, c.K)
	}
	return nil
}

// GaussianBG is a per-pixel Gaussian background model (a "running Gaussian
// average"). Every pixel of every input channel keeps its own mean μ and
// variance σ², updated recursively from frame to frame:
//
//	μ  ← (1 − ρ)·μ  + ρ·x
//	σ² ← (1 − ρ)·σ² + ρ·(x − μ)²
//
// where ρ is Alpha for background pixels. Foreground and shadow pixels move
// only the mean, at the much slower rate FGAlpha, and leave the variance
// alone: an object standing on a pixel says nothing about how noisy the
// background behind it is, and letting it widen σ would blind the model. A pixel is
// foreground when some channel deviates from its mean by more than K·σ.
// Because σ is learned per pixel, noisy areas (dark corners, waving leaves)
// automatically get a wider tolerance than quiet ones.
//
// With shadow suppression on, a deviating pixel that got moderately darker
// while its other channels stayed within ShadowK·σ is classified as shadow,
// not as an object: a shadow dims the surface but hardly changes its color.
//
// The model is stateful: feed it the frames of one sequence in order.
// It is not safe for concurrent use.
type GaussianBG struct {
	cfg      GaussianBGConfig
	w, h     int
	mean     [][]float32 // [channel][pixel]
	variance [][]float32
	frames   int
}

// NewGaussianBG returns an empty model, or an error if the configuration is invalid.
func NewGaussianBG(cfg GaussianBGConfig) (*GaussianBG, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &GaussianBG{cfg: cfg}, nil
}

// Reset forgets everything learned, so the model can be reused for another sequence.
func (m *GaussianBG) Reset() {
	m.mean, m.variance, m.frames = nil, nil, 0
}

// Frames returns how many frames the model has seen.
func (m *GaussianBG) Frames() int { return m.frames }

// Background returns a copy of the current mean of the given channel,
// i.e. what the model believes the empty scene looks like.
func (m *GaussianBG) Background(channel int) (*imgx.Gray, error) {
	if channel < 0 || channel >= len(m.mean) {
		return nil, fmt.Errorf("segmentation: no channel %d in a model with %d channels", channel, len(m.mean))
	}
	out := imgx.NewGray(m.w, m.h)
	copy(out.Pix, m.mean[channel])
	return out, nil
}

// Apply classifies one frame, given as one or more feature channels of equal
// size, then updates the model with it. It returns a mask with MaskForeground
// (255) on moving pixels. The first frame only initializes the model and
// yields an empty mask.
func (m *GaussianBG) Apply(channels ...*imgx.Gray) (*imgx.Gray, error) {
	if err := m.check(channels); err != nil {
		return nil, err
	}
	mask := imgx.NewGray(channels[0].W, channels[0].H)
	if m.frames == 0 {
		m.init(channels)
		return mask, nil
	}

	rate := m.cfg.Alpha
	warm := m.frames < m.cfg.Warmup
	if warm {
		rate = max(rate, 1/float32(m.frames+1))
	}
	for i := range mask.Pix {
		label := m.classify(i, channels)
		switch {
		case label == MaskForeground:
			mask.Pix[i] = MaskForeground
		case label == MaskShadow && m.cfg.MarkShadows:
			mask.Pix[i] = MaskShadow
		}

		if warm || label == MaskBackground {
			m.update(i, channels, rate, true)
		} else {
			m.update(i, channels, m.cfg.FGAlpha, false)
		}
	}
	m.frames++
	return mask, nil
}

func (m *GaussianBG) check(channels []*imgx.Gray) error {
	if len(channels) == 0 {
		return errors.New("segmentation: GaussianBG needs at least one channel")
	}
	for _, c := range channels[1:] {
		if err := imgx.CheckSameSize(channels[0], c); err != nil {
			return err
		}
	}
	if max(m.cfg.ShadowChannel, m.cfg.RatioChannel) >= len(channels) {
		return fmt.Errorf("segmentation: ShadowChannel %d and RatioChannel %d need more than the %d channels given",
			m.cfg.ShadowChannel, m.cfg.RatioChannel, len(channels))
	}
	if m.frames == 0 {
		return nil
	}
	if len(channels) != len(m.mean) {
		return fmt.Errorf("segmentation: got %d channels, model was built with %d", len(channels), len(m.mean))
	}
	if channels[0].W != m.w || channels[0].H != m.h {
		return fmt.Errorf("%w: frame %dx%d, model %dx%d",
			imgx.ErrSizeMismatch, channels[0].W, channels[0].H, m.w, m.h)
	}
	return nil
}

func (m *GaussianBG) init(channels []*imgx.Gray) {
	m.w, m.h = channels[0].W, channels[0].H
	m.mean = make([][]float32, len(channels))
	m.variance = make([][]float32, len(channels))
	for c, ch := range channels {
		m.mean[c] = make([]float32, len(ch.Pix))
		copy(m.mean[c], ch.Pix)
		m.variance[c] = make([]float32, len(ch.Pix))
		for i := range m.variance[c] {
			m.variance[c][i] = m.cfg.InitVar
		}
	}
	m.frames = 1
}

// classify returns MaskBackground, MaskShadow or MaskForeground for pixel i.
func (m *GaussianBG) classify(i int, channels []*imgx.Gray) int {
	k2 := m.cfg.K * m.cfg.K
	shadowK2 := m.cfg.ShadowK * m.cfg.ShadowK
	deviates := false
	colorKept := true // the non-brightness channels are within ShadowK·σ
	for c, ch := range channels {
		if c == m.cfg.RatioChannel {
			continue // passive: tracked, but does not vote
		}
		d := ch.Pix[i] - m.mean[c][i]
		d2 := d * d
		if d2 > k2*m.variance[c][i] {
			deviates = true
		}
		if c != m.cfg.ShadowChannel && d2 > shadowK2*m.variance[c][i] {
			colorKept = false
		}
	}
	if !deviates {
		return MaskBackground
	}
	if m.cfg.ShadowChannel >= 0 && colorKept {
		s := m.cfg.ShadowChannel
		if m.cfg.RatioChannel >= 0 {
			s = m.cfg.RatioChannel
		}
		x, mu := channels[s].Pix[i], m.mean[s][i]
		if x < mu && x >= m.cfg.ShadowLow*mu {
			return MaskShadow
		}
	}
	return MaskForeground
}

// update moves the mean of pixel i towards the frame and, if withVariance is
// set, the variance towards the squared deviation.
func (m *GaussianBG) update(i int, channels []*imgx.Gray, rate float32, withVariance bool) {
	if rate == 0 {
		return
	}
	for c, ch := range channels {
		d := ch.Pix[i] - m.mean[c][i]
		m.mean[c][i] += rate * d
		if withVariance {
			m.variance[c][i] = max((1-rate)*m.variance[c][i]+rate*d*d, m.cfg.MinVar)
		}
	}
}
