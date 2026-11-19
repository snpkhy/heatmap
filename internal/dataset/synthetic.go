package dataset

import (
	"fmt"
	"math"
	"math/rand/v2"
	"sort"

	"heatmap/internal/imgx"
)

// Ground-truth labels used by synthetic scenes (CDnet convention).
const (
	gtStatic = 0
	gtShadow = 50
	gtMotion = 255
)

// object is a textured rectangle moving at constant velocity between frames
// t0 and t1 (inclusive). With person set it is drawn as a walking figure
// instead: see person.go.
type object struct {
	w, h           int
	x0, y0, vx, vy float64 // top-left corner at frame t0, pixels per frame
	t0, t1         int
	color          [3]float32
	texture        float32 // amplitude of the 4×4 checker texture
	// Shadow: a copy of the rectangle offset by (shadowDX, shadowDY) where the
	// background is multiplied by shadow (0 = no shadow).
	shadowDX, shadowDY int
	shadow             float32
	// person draws a walking figure: (x0, y0) is then the point between the
	// feet, h the height of the figure at the bottom edge of the frame (it
	// shrinks with distance), color the clothes, pants and skin the rest.
	person      bool
	pants, skin [3]float32
}

// scene describes a built-in synthetic sequence.
type scene struct {
	name, description string
	w, h, frames      int
	sensorNoise       float64 // σ of per-frame Gaussian noise
	background        func(x, y int, rng *rand.Rand) (r, g, b float32)
	objects           []object
}

// scenes are the built-in synthetic sequences. They let every pipeline, the
// CLI and the web UI run without downloading CDnet, and give the test cases
// a ground truth that is exact by construction.
var scenes = map[string]scene{
	"street": {
		name:        "street",
		description: "Road with lane markings, a car driving right and a pedestrian walking diagonally",
		w:           160, h: 120, frames: 150,
		sensorNoise: 3,
		background: func(x, y int, rng *rand.Rand) (r, g, b float32) {
			v := 95 + float32(y)*0.25 + float32(rng.NormFloat64()*6) // asphalt
			if y >= 60 && y < 62 && (x/12)%2 == 0 {
				v = 220 // dashed lane marking
			}
			if y < 30 {
				return v * 0.7, v * 0.9, v * 0.6 // grass verge
			}
			return v, v, v * 1.05
		},
		objects: []object{
			{w: 32, h: 16, x0: -32, y0: 72, vx: 2, t0: 15, t1: 111, color: [3]float32{200, 40, 40}, texture: 35},
			{w: 10, h: 24, x0: 140, y0: 34, vx: -1, vy: 0.5, t0: 30, t1: 140, color: [3]float32{40, 70, 210}, texture: 35},
		},
	},
	"shadow": {
		name:        "shadow",
		description: "Paved square with an object casting a hard shadow (shadow pixels are labelled 50)",
		w:           160, h: 120, frames: 120,
		sensorNoise: 2,
		background: func(x, y int, rng *rand.Rand) (r, g, b float32) {
			v := float32(70+((x*7+y*13)%5)*30) + float32(rng.NormFloat64()*3) // paving
			return v, v * 0.95, v * 0.85
		},
		objects: []object{
			{w: 20, h: 20, x0: -20, y0: 36, vx: 1.5, t0: 15, t1: 119, color: [3]float32{230, 220, 60}, texture: 30,
				shadowDX: 6, shadowDY: 20, shadow: 0.5},
		},
	},
	"plaza": plaza,
	"night": {
		name:        "night",
		description: "Dark low-contrast scene with two dim objects moving in opposite directions",
		w:           160, h: 120, frames: 120,
		sensorNoise: 2.5,
		background: func(x, y int, rng *rand.Rand) (r, g, b float32) {
			v := 14 + float32((x+2*y)%9) + float32(rng.NormFloat64()*1.5)
			return v * 0.8, v * 0.9, v * 1.3
		},
		objects: []object{
			{w: 26, h: 14, x0: -26, y0: 50, vx: 1.5, t0: 15, t1: 119, color: [3]float32{48, 44, 60}, texture: 10},
			{w: 14, h: 22, x0: 160, y0: 80, vx: -1.2, t0: 25, t1: 119, color: [3]float32{55, 50, 40}, texture: 10},
		},
	},
}

// syntheticWarmup is the number of first frames without ground truth, left
// for background models to settle (like the frames before CDnet's temporal ROI).
const syntheticWarmup = 10

// SyntheticNames returns the names of the built-in scenes, sorted.
func SyntheticNames() []string {
	names := make([]string, 0, len(scenes))
	for n := range scenes {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Synthetic is a deterministic generated sequence with exact ground truth.
type Synthetic struct {
	sc   scene
	info Info
	bg   *imgx.RGB
}

// NewSynthetic returns the built-in scene with the given name.
func NewSynthetic(name string) (*Synthetic, error) {
	sc, ok := scenes[name]
	if !ok {
		return nil, fmt.Errorf("%w: synthetic scene %q (have %v)", ErrNotFound, name, SyntheticNames())
	}
	rng := rand.New(rand.NewPCG(1, uint64(len(name))))
	bg := imgx.NewRGB(sc.w, sc.h)
	for y := range sc.h {
		for x := range sc.w {
			r, g, b := sc.background(x, y, rng)
			bg.Set(x, y, imgx.Clamp255(r), imgx.Clamp255(g), imgx.Clamp255(b))
		}
	}
	return &Synthetic{
		sc: sc,
		bg: bg,
		info: Info{
			ID:        SourceSynthetic + "/" + name,
			Name:      name,
			Category:  SourceSynthetic,
			Source:    SourceSynthetic,
			Frames:    sc.frames,
			Width:     sc.w,
			Height:    sc.h,
			HasGT:     true,
			EvalStart: syntheticWarmup,
			EvalEnd:   sc.frames - 1,
		},
	}, nil
}

// Info implements Sequence.
func (s *Synthetic) Info() Info {
	info := s.info
	info.Path = s.sc.description
	return info
}

// Frame implements Sequence.
func (s *Synthetic) Frame(i int) (*imgx.RGB, error) {
	if err := checkIndex(i, s.sc.frames); err != nil {
		return nil, err
	}
	f, _ := s.render(i)
	return f, nil
}

// GroundTruth implements Sequence.
func (s *Synthetic) GroundTruth(i int) (*imgx.Gray, error) {
	if err := checkIndex(i, s.sc.frames); err != nil {
		return nil, err
	}
	if i < s.info.EvalStart {
		return nil, nil
	}
	_, gt := s.render(i)
	return gt, nil
}

// render draws frame t and its labels: background, then shadows, then
// objects, then sensor noise (seeded by t, so every call is identical).
func (s *Synthetic) render(t int) (*imgx.RGB, *imgx.Gray) {
	f := s.bg.Clone()
	gt := imgx.NewGray(s.sc.w, s.sc.h)
	for _, o := range s.sc.objects {
		if o.person {
			s.personShadow(o, t, f, gt)
			continue
		}
		if x, y, ok := o.at(t); ok && o.shadow > 0 {
			s.fill(x+o.shadowDX, y+o.shadowDY, o.w, o.h, func(_, _, i int) {
				f.R[i] *= o.shadow
				f.G[i] *= o.shadow
				f.B[i] *= o.shadow
				gt.Pix[i] = gtShadow
			})
		}
	}
	for _, o := range s.sc.objects {
		if o.person {
			s.person(o, t, f, gt)
			continue
		}
		x, y, ok := o.at(t)
		if !ok {
			continue
		}
		s.fill(x, y, o.w, o.h, func(px, py, i int) {
			// The checker texture is anchored to the object, so it moves with it.
			d := o.texture
			if ((px-x)/4+(py-y)/4)%2 == 1 {
				d = -d
			}
			f.R[i], f.G[i], f.B[i] = o.color[0]+d, o.color[1]+d, o.color[2]+d
			gt.Pix[i] = gtMotion
		})
	}
	if s.sc.sensorNoise > 0 {
		rng := rand.New(rand.NewPCG(7, uint64(t)))
		for _, plane := range [][]float32{f.R, f.G, f.B} {
			for i, v := range plane {
				plane[i] = imgx.Clamp255(v + float32(rng.NormFloat64()*s.sc.sensorNoise))
			}
		}
	}
	return f, gt
}

// fill calls fn for every pixel of the rectangle clipped to the frame.
func (s *Synthetic) fill(x0, y0, w, h int, fn func(x, y, i int)) {
	for y := max(y0, 0); y < min(y0+h, s.sc.h); y++ {
		for x := max(x0, 0); x < min(x0+w, s.sc.w); x++ {
			fn(x, y, y*s.sc.w+x)
		}
	}
}

// at returns the integer top-left corner of the object at frame t, and
// whether it exists then.
func (o object) at(t int) (x, y int, ok bool) {
	if t < o.t0 || t > o.t1 {
		return 0, 0, false
	}
	dt := float64(t - o.t0)
	return int(math.Round(o.x0 + o.vx*dt)), int(math.Round(o.y0 + o.vy*dt)), true
}
