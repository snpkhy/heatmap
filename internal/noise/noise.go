package noise

import (
	"fmt"
	"math"
	"math/rand/v2"
	"strconv"
	"strings"

	"heatmap/internal/imgx"
)

// Noise types accepted by Spec.Type.
const (
	None       = "none"
	Gaussian   = "gaussian"
	SaltPepper = "salt_pepper"
)

// Spec describes synthetic noise added to every frame before a pipeline sees it.
type Spec struct {
	// Type is None, Gaussian or SaltPepper.
	Type string `json:"type" yaml:"type"`
	// Level is σ in intensity units for Gaussian noise, and the fraction of
	// corrupted pixels in [0, 1] for salt-and-pepper noise.
	Level float64 `json:"level" yaml:"level"`
	// Seed makes the noise reproducible. Frame i always gets the same noise
	// for the same seed, whatever order frames are processed in.
	Seed uint64 `json:"seed" yaml:"seed"`
}

// Parse reads a spec written as "type" or "type:level", e.g. "none",
// "gaussian:10" or "salt_pepper:0.02". The seed is set separately.
func Parse(s string) (Spec, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == None {
		return Spec{Type: None}, nil
	}
	typ, lvl, found := strings.Cut(s, ":")
	if !found {
		return Spec{}, fmt.Errorf("noise: %q needs a level, e.g. %s:10", s, typ)
	}
	level, err := strconv.ParseFloat(lvl, 64)
	if err != nil {
		return Spec{}, fmt.Errorf("noise: bad level in %q: %w", s, err)
	}
	spec := Spec{Type: typ, Level: level}
	return spec, spec.Validate()
}

// String returns the spec in the form accepted by Parse.
func (s Spec) String() string {
	if s.Type == "" || s.Type == None {
		return None
	}
	return s.Type + ":" + strconv.FormatFloat(s.Level, 'g', -1, 64)
}

// IsNone reports whether the spec adds no noise.
func (s Spec) IsNone() bool {
	return s.Type == "" || s.Type == None || s.Level == 0
}

// Validate checks the type and the level range.
func (s Spec) Validate() error {
	switch s.Type {
	case "", None:
		return nil
	case Gaussian:
		if !(s.Level >= 0 && s.Level <= 255) {
			return fmt.Errorf("noise: Gaussian sigma %v must be in [0, 255]", s.Level)
		}
	case SaltPepper:
		if !(s.Level >= 0 && s.Level <= 1) {
			return fmt.Errorf("noise: salt-and-pepper amount %v must be in [0, 1]", s.Level)
		}
	default:
		return fmt.Errorf("noise: unknown type %q (want %s, %s or %s)", s.Type, None, Gaussian, SaltPepper)
	}
	return nil
}

// Apply returns a noisy copy of frame number index. The source is not modified.
func (s Spec) Apply(c *imgx.RGB, index int) *imgx.RGB {
	if s.IsNone() {
		return c
	}
	// One generator per frame, seeded from (seed, frame), so results do not
	// depend on the processing order (parallel experiments stay reproducible).
	rng := rand.New(rand.NewPCG(s.Seed, uint64(index)*0x9E3779B97F4A7C15+1))
	switch s.Type {
	case Gaussian:
		return AddGaussian(c, s.Level, rng)
	case SaltPepper:
		return AddSaltPepper(c, s.Level, rng)
	}
	return c
}

// AddGaussian adds independent zero-mean Gaussian noise with standard
// deviation sigma to every channel of every pixel and clamps to [0, 255].
// Normal samples come from math/rand's ziggurat generator.
func AddGaussian(c *imgx.RGB, sigma float64, rng *rand.Rand) *imgx.RGB {
	out := c.Clone()
	for _, plane := range [][]float32{out.R, out.G, out.B} {
		for i, v := range plane {
			plane[i] = imgx.Clamp255(v + float32(sigma*rng.NormFloat64()))
		}
	}
	return out
}

// AddSaltPepper sets a random fraction amount of the pixels to black or white
// (with equal probability) in all three channels, like dead or hot sensor
// pixels and transmission errors.
func AddSaltPepper(c *imgx.RGB, amount float64, rng *rand.Rand) *imgx.RGB {
	out := c.Clone()
	n := len(out.R)
	hits := int(math.Round(amount * float64(n)))
	// Partial Fisher–Yates shuffle picks hits distinct pixels.
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	for k := range hits {
		j := k + rng.IntN(n-k)
		idx[k], idx[j] = idx[j], idx[k]
		var v float32
		if rng.IntN(2) == 1 {
			v = 255
		}
		p := idx[k]
		out.R[p], out.G[p], out.B[p] = v, v, v
	}
	return out
}
