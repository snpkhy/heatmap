package pipeline

import (
	"fmt"
	"math"

	"heatmap/internal/imgx"
	"heatmap/internal/methods/color"
	"heatmap/internal/methods/edges"
	"heatmap/internal/methods/filtering"
	"heatmap/internal/methods/fourier"
	"heatmap/internal/methods/geometry"
	"heatmap/internal/methods/histogram"
	"heatmap/internal/methods/keypoints"
	"heatmap/internal/methods/morphology"
	"heatmap/internal/methods/segmentation"
	"heatmap/internal/methods/stats"
	"heatmap/internal/methods/texture"
)

// funcStage is a stateless stage.
type funcStage func(f *Frame) (*imgx.Gray, error)

func (s funcStage) Process(f *Frame) (*imgx.Gray, error) { return s(f) }
func (funcStage) Reset()                                 {}

// stateStage is a stage with state that reset clears.
type stateStage struct {
	process func(f *Frame) (*imgx.Gray, error)
	reset   func()
}

func (s *stateStage) Process(f *Frame) (*imgx.Gray, error) { return s.process(f) }
func (s *stateStage) Reset()                               { s.reset() }

// probe is a tiny image used to validate parameters of stateless methods at
// build time, so a bad window size is reported before the first frame.
func probe() *imgx.Gray { return imgx.NewGray(3, 3) }

func needMask(f *Frame) error {
	if f.Mask == nil {
		return fmt.Errorf("needs a motion mask from an earlier stage")
	}
	return nil
}

// imageStage builds a stateless stage that replaces Image.
func imageStage(fn func(*imgx.Gray) (*imgx.Gray, error)) (Stage, error) {
	if _, err := fn(probe()); err != nil {
		return nil, err
	}
	return funcStage(func(f *Frame) (*imgx.Gray, error) {
		out, err := fn(f.Image)
		if err != nil {
			return nil, err
		}
		f.Image = out
		return out, nil
	}), nil
}

// maskStage builds a stateless stage that transforms Mask.
func maskStage(fn func(*imgx.Gray) (*imgx.Gray, error)) (Stage, error) {
	if _, err := fn(probe()); err != nil {
		return nil, err
	}
	return funcStage(func(f *Frame) (*imgx.Gray, error) {
		if err := needMask(f); err != nil {
			return nil, err
		}
		out, err := fn(f.Mask)
		if err != nil {
			return nil, err
		}
		f.Mask = out
		return out, nil
	}), nil
}

// detector is a stateful method that turns an image into a mask.
type detector interface {
	Apply(*imgx.Gray) (*imgx.Gray, error)
	Reset()
}

// detectorStage runs a detector on Image and writes Mask.
func detectorStage(d detector) Stage {
	return &stateStage{
		process: func(f *Frame) (*imgx.Gray, error) {
			m, err := d.Apply(f.Image)
			if err != nil {
				return nil, err
			}
			f.Mask = m
			return m, nil
		},
		reset: d.Reset,
	}
}

func init() {
	registerFiltering()
	registerColor()
	registerHistogram()
	registerSegmentation()
	registerMorphology()
	registerEdges()
	registerTexture()
	registerStats()
	registerFourier()
	registerGeometry()
	registerKeypoints()
}

func registerFiltering() {
	Register(Method{Name: "gaussian", Category: CatFiltering, Denoise: true,
		Description: "Gaussian blur with a k×k kernel",
		New: func(p *Params) (Stage, error) {
			k := p.Int("k", 5, "kernel size, odd")
			sigma := p.Float("sigma", 1.0, "standard deviation in pixels")
			return imageStage(func(g *imgx.Gray) (*imgx.Gray, error) { return filtering.Gaussian(g, k, sigma) })
		}})
	Register(Method{Name: "median", Category: CatFiltering, Denoise: true,
		Description: "median of the k×k neighbourhood; removes impulse noise, keeps edges",
		New: func(p *Params) (Stage, error) {
			k := p.Int("k", 3, "window size, odd")
			return imageStage(func(g *imgx.Gray) (*imgx.Gray, error) { return filtering.Median(g, k) })
		}})
	Register(Method{Name: "box", Category: CatFiltering, Denoise: true,
		Description: "mean of the k×k neighbourhood via an integral image (constant cost per pixel)",
		New: func(p *Params) (Stage, error) {
			k := p.Int("k", 3, "window size, odd")
			return imageStage(func(g *imgx.Gray) (*imgx.Gray, error) { return filtering.Box(g, k) })
		}})
}

func registerColor() {
	Register(Method{Name: "rgb_to_ycbcr", Category: CatColor,
		Description: "BT.601 YCbCr; Channels = Y, Cb, Cr and Image = Y (luma)",
		New: func(_ *Params) (Stage, error) {
			return funcStage(func(f *Frame) (*imgx.Gray, error) {
				p := color.RGBToYCbCr(f.RGB)
				y := p.Luma()
				f.Channels = []*imgx.Gray{y, plane(p.W, p.H, p.Cb), plane(p.W, p.H, p.Cr)}
				f.Image = y
				return y, nil
			}), nil
		}})
	Register(Method{Name: "rgb_to_hsv", Category: CatColor,
		Description: "HSV; Channels = H (scaled to 0–255), S (0–255), V and Image = V",
		New: func(_ *Params) (Stage, error) {
			return funcStage(func(f *Frame) (*imgx.Gray, error) {
				p := color.RGBToHSV(f.RGB)
				h, s := imgx.NewGray(p.W, p.H), imgx.NewGray(p.W, p.H)
				for i := range p.Hue {
					h.Pix[i] = p.Hue[i] * 255 / 360
					s.Pix[i] = p.Sat[i] * 255
				}
				v := p.Value()
				f.Channels = []*imgx.Gray{h, s, v}
				f.Image = v
				return v, nil
			}), nil
		}})
	Register(Method{Name: "chroma_value", Category: CatColor,
		Description: "chromaticity r, g plus HSV value V (p7); Channels = r, g, V and Image = V",
		New: func(p *Params) (Stage, error) {
			minSum := p.Float32("min_sum", color.DefaultMinSum, "R+G+B below which chromaticity is neutral")
			if !(minSum >= 0) {
				return nil, fmt.Errorf("min_sum %v must not be negative", minSum)
			}
			return funcStage(func(f *Frame) (*imgx.Gray, error) {
				c := color.ChromaValue(f.RGB, minSum)
				r, g, v := c.Channel(0), c.Channel(1), c.Channel(2)
				f.Channels = []*imgx.Gray{r, g, v}
				f.Image = v
				return v, nil
			}), nil
		}})
}

func plane(w, h int, pix []float32) *imgx.Gray {
	return &imgx.Gray{W: w, H: h, Pix: pix}
}

func registerHistogram() {
	Register(Method{Name: "equalize", Category: CatHistogram,
		Description: "global histogram equalization of Image, or of one of the Channels",
		New: func(p *Params) (Stage, error) {
			ch := p.Int("channel", -1, "index into Channels to equalize, -1 for Image")
			keepRaw := p.Bool("keep_raw", false, "append the unequalized channel to Channels (for shadow ratios)")
			if ch < -1 {
				return nil, fmt.Errorf("channel %d must be -1 or an index", ch)
			}
			return funcStage(func(f *Frame) (*imgx.Gray, error) {
				if ch < 0 {
					f.Image = histogram.Equalize(f.Image)
					return f.Image, nil
				}
				if ch >= len(f.Channels) {
					return nil, fmt.Errorf("channel %d: the frame has %d channels (add a color stage first)", ch, len(f.Channels))
				}
				raw := f.Channels[ch]
				eq := histogram.Equalize(raw)
				f.Channels[ch] = eq
				if keepRaw {
					f.Channels = append(f.Channels, raw)
				}
				f.Image = eq
				return eq, nil
			}), nil
		}})
	Register(Method{Name: "clahe", Category: CatHistogram,
		Description: "contrast limited adaptive histogram equalization of Image",
		New: func(p *Params) (Stage, error) {
			def := histogram.DefaultCLAHEConfig()
			cfg := histogram.CLAHEConfig{
				GridX:     p.Int("grid_x", def.GridX, "tiles across"),
				GridY:     p.Int("grid_y", def.GridY, "tiles down"),
				ClipLimit: p.Float("clip_limit", def.ClipLimit, "contrast limit, >= 1"),
			}
			return imageStage(func(g *imgx.Gray) (*imgx.Gray, error) { return histogram.CLAHE(g, cfg) })
		}})
}

func registerSegmentation() {
	Register(Method{Name: "running_avg_otsu", Category: CatSegmentation,
		Description: "running-average background model with an Otsu threshold on the difference (p1)",
		New: func(p *Params) (Stage, error) {
			def := segmentation.DefaultRunningAvgBGConfig()
			m, err := segmentation.NewRunningAvgBG(segmentation.RunningAvgBGConfig{
				Alpha:        p.Float32("alpha", def.Alpha, "background learning rate"),
				FGAlpha:      p.Float32("fg_alpha", def.FGAlpha, "learning rate under foreground"),
				MinThreshold: p.Float32("min_threshold", def.MinThreshold, "lowest allowed threshold"),
			})
			if err != nil {
				return nil, err
			}
			return detectorStage(m), nil
		}})
	Register(Method{Name: "frame_diff", Category: CatSegmentation,
		Description: "|I(t) - I(t-1)| > threshold (p5)",
		New: func(p *Params) (Stage, error) {
			d, err := segmentation.NewFrameDiff(p.Float32("threshold", 25, "smallest change that counts as motion"))
			if err != nil {
				return nil, err
			}
			return detectorStage(d), nil
		}})
	Register(Method{Name: "frame_diff_region_growing", Category: CatSegmentation,
		Description: "frame difference seeds grown into connected regions (p2)",
		New: func(p *Params) (Stage, error) {
			def := segmentation.DefaultRegionGrowingConfig()
			m, err := segmentation.NewFrameDiffRegionGrowing(segmentation.RegionGrowingConfig{
				SeedThreshold:    p.Float32("seed_threshold", def.SeedThreshold, "difference that starts a region"),
				GrowThreshold:    p.Float32("grow_threshold", def.GrowThreshold, "difference that lets a region grow"),
				MaxIntensityDiff: p.Float32("max_intensity_diff", def.MaxIntensityDiff, "intensity similarity limit, <= 0 off"),
				Connectivity:     p.Int("connectivity", def.Connectivity, "4 or 8"),
			})
			if err != nil {
				return nil, err
			}
			return detectorStage(m), nil
		}})
	Register(Method{Name: "gaussian_bg", Category: CatSegmentation,
		Description: "per-pixel Gaussian background model on Channels (or Image), optional shadow suppression (p7)",
		New: func(p *Params) (Stage, error) {
			def := segmentation.DefaultGaussianBGConfig()
			m, err := segmentation.NewGaussianBG(segmentation.GaussianBGConfig{
				Alpha:         p.Float32("alpha", def.Alpha, "background learning rate"),
				FGAlpha:       p.Float32("fg_alpha", def.FGAlpha, "learning rate under foreground"),
				K:             p.Float32("k", def.K, "threshold in standard deviations"),
				InitVar:       p.Float32("init_var", def.InitVar, "initial variance"),
				MinVar:        p.Float32("min_var", def.MinVar, "variance floor"),
				Warmup:        p.Int("warmup", def.Warmup, "frames of cumulative averaging"),
				ShadowChannel: p.Int("shadow_channel", def.ShadowChannel, "brightness channel for shadows, -1 off"),
				RatioChannel:  p.Int("ratio_channel", def.RatioChannel, "passive channel for the shadow ratio, -1 none"),
				ShadowLow:     p.Float32("shadow_low", def.ShadowLow, "darkest shadow relative to background"),
				ShadowK:       p.Float32("shadow_k", def.ShadowK, "color tolerance of shadows in std devs"),
				MarkShadows:   p.Bool("mark_shadows", false, "write 50 for shadow pixels"),
			})
			if err != nil {
				return nil, err
			}
			return &stateStage{
				process: func(f *Frame) (*imgx.Gray, error) {
					in := f.Channels
					if len(in) == 0 {
						in = []*imgx.Gray{f.Image}
					}
					mask, err := m.Apply(in...)
					if err != nil {
						return nil, err
					}
					f.Mask = mask
					return mask, nil
				},
				reset: m.Reset,
			}, nil
		}})
	Register(Method{Name: "otsu", Category: CatSegmentation,
		Description: "global Otsu threshold of Image",
		New: func(_ *Params) (Stage, error) {
			return funcStage(func(f *Frame) (*imgx.Gray, error) {
				m, _, err := segmentation.OtsuThreshold(f.Image)
				if err != nil {
					return nil, err
				}
				f.Mask = m
				return m, nil
			}), nil
		}})
	Register(Method{Name: "block_threshold", Category: CatSegmentation,
		Description: "a block is foreground when its mean Image value is above threshold (p4)",
		New: func(p *Params) (Stage, error) {
			block := p.Int("block", 16, "block size in pixels")
			th := p.Float32("threshold", 100, "mean value above which a block is foreground")
			if _, err := segmentation.BlockThreshold(probe(), block, th); err != nil {
				return nil, err
			}
			return funcStage(func(f *Frame) (*imgx.Gray, error) {
				m, err := segmentation.BlockThreshold(f.Image, block, th)
				if err != nil {
					return nil, err
				}
				f.Mask = m
				return m, nil
			}), nil
		}})
	Register(Method{Name: "components", Category: CatSegmentation,
		Description: "keep connected components of Mask with area in [min_area, max_area] (p8)",
		New: func(p *Params) (Stage, error) {
			conn := segmentation.Connectivity(p.Int("connectivity", 8, "4 or 8"))
			minArea := p.Int("min_area", 20, "smallest kept component in pixels")
			maxArea := p.Int("max_area", 0, "largest kept component, 0 = no limit")
			return maskStage(func(m *imgx.Gray) (*imgx.Gray, error) {
				out, _, err := segmentation.FilterByArea(m, conn, minArea, maxArea)
				return out, err
			})
		}})
	Register(Method{Name: "block_matching", Category: CatSegmentation,
		Description: "match the previous frame's corners into this frame (SAD); writes Vectors and a Mask of moving corners (p6)",
		New:         newBlockMatching,
	})
}

func newBlockMatching(p *Params) (Stage, error) {
	def := keypoints.DefaultTrackerConfig()
	block := p.Int("block", def.Block, "matched window size, odd")
	search := p.Int("search", def.Search, "largest displacement tried")
	maxCost := p.Float32("max_cost", def.MaxCost, "largest mean absolute difference accepted")
	minMotion := p.Float("min_motion", def.MinMotion, "displacement from which a corner is moving")
	radius := p.Int("mask_radius", def.MaskRadius, "disc radius drawn per moving corner")
	if _, err := keypoints.MatchBlocks(probe(), probe(), nil, block, search); err != nil {
		return nil, err
	}
	if !(maxCost > 0) || !(minMotion >= 0) || radius < 0 {
		return nil, fmt.Errorf("max_cost must be positive, min_motion and mask_radius not negative")
	}
	var prev *imgx.Gray
	var prevCorners []keypoints.Corner
	return &stateStage{
		process: func(f *Frame) (*imgx.Gray, error) {
			defer func() { prev, prevCorners = f.Image, f.Corners }()
			f.Vectors = nil
			if prev == nil || !imgx.SameSize(prev, f.Image) {
				f.Mask = imgx.NewGray(f.Image.W, f.Image.H)
				return f.Mask, nil
			}
			all, err := keypoints.MatchBlocks(prev, f.Image, prevCorners, block, search)
			if err != nil {
				return nil, err
			}
			for _, m := range all {
				if m.Cost <= maxCost && m.Magnitude() >= minMotion && m.Magnitude() > 0 {
					f.Vectors = append(f.Vectors, m)
				}
			}
			f.Mask = keypoints.MotionMask(f.Image.W, f.Image.H, f.Vectors, radius)
			return f.Mask, nil
		},
		reset: func() { prev, prevCorners = nil, nil },
	}, nil
}

func registerMorphology() {
	for _, m := range []struct {
		name, desc string
		fn         func(*imgx.Gray, int) (*imgx.Gray, error)
	}{
		{"opening", "erosion then dilation of Mask: removes specks", morphology.Opening},
		{"closing", "dilation then erosion of Mask: fills holes, joins fragments", morphology.Closing},
		{"dilate", "binary dilation of Mask", morphology.Dilate},
		{"erode", "binary erosion of Mask", morphology.Erode},
	} {
		fn := m.fn
		Register(Method{Name: m.name, Category: CatMorphology, Description: m.desc,
			New: func(p *Params) (Stage, error) {
				k := p.Int("k", 3, "structuring element size, odd")
				return maskStage(func(g *imgx.Gray) (*imgx.Gray, error) { return fn(g, k) })
			}})
	}
}

func registerEdges() {
	Register(Method{Name: "sobel", Category: CatEdges,
		Description: "Sobel gradient magnitude of Image",
		New: func(_ *Params) (Stage, error) {
			return imageStage(func(g *imgx.Gray) (*imgx.Gray, error) { return edges.SobelMagnitude(g), nil })
		}})
	Register(Method{Name: "prewitt", Category: CatEdges,
		Description: "Prewitt gradient magnitude of Image (p8)",
		New: func(_ *Params) (Stage, error) {
			return imageStage(func(g *imgx.Gray) (*imgx.Gray, error) { return edges.Prewitt(g), nil })
		}})
	Register(Method{Name: "sobel_diff", Category: CatEdges,
		Description: "motion as a change of Sobel gradients against a background model (p3)",
		New: func(p *Params) (Stage, error) {
			d, err := edges.NewSobelDiff(
				p.Float32("threshold", 60, "smallest gradient change that counts as motion"),
				p.Float32("alpha", 0.05, "background learning rate"))
			if err != nil {
				return nil, err
			}
			return detectorStage(d), nil
		}})
}

func registerTexture() {
	Register(Method{Name: "lbp", Category: CatTexture,
		Description: "local binary pattern code of every pixel",
		New: func(p *Params) (Stage, error) {
			margin := p.Float32("margin", 0.05, "relative noise margin")
			return imageStage(func(g *imgx.Gray) (*imgx.Gray, error) { return texture.LBP(g, margin), nil })
		}})
	Register(Method{Name: "block_lbp", Category: CatTexture,
		Description: "per-block LBP histogram distance to a background model; Image = change map 0–255 (p4)",
		New: func(p *Params) (Stage, error) {
			b, err := texture.NewBlockLBP(
				p.Int("block", 16, "block size in pixels"),
				p.Float32("margin", 0.05, "relative LBP noise margin"),
				p.Float32("alpha", 0.05, "background learning rate"))
			if err != nil {
				return nil, err
			}
			return &stateStage{
				process: func(f *Frame) (*imgx.Gray, error) {
					out, err := b.Apply(f.Image)
					if err != nil {
						return nil, err
					}
					f.Image = out
					return out, nil
				},
				reset: func() {
					fresh, _ := texture.NewBlockLBP(b.Block, b.Margin, b.Alpha)
					*b = *fresh
				},
			}, nil
		}})
}

func registerStats() {
	Register(Method{Name: "block_change", Category: CatStats,
		Description: "motion from changes of block mean and variance, adaptive thresholds (p8)",
		New: func(p *Params) (Stage, error) {
			def := stats.DefaultBlockChangeConfig()
			d, err := stats.NewBlockChange(stats.BlockChangeConfig{
				Block:       p.Int("block", def.Block, "block size in pixels"),
				MeanThresh:  p.Float32("mean_thresh", def.MeanThresh, "floor of the mean threshold"),
				StdThresh:   p.Float32("std_thresh", def.StdThresh, "floor of the std threshold"),
				NoiseFactor: p.Float32("noise_factor", def.NoiseFactor, "threshold in units of typical fluctuation, 0 off"),
				NoiseStep:   p.Float32("noise_step", def.NoiseStep, "fluctuation tracking step"),
				Step:        p.Float32("step", def.Step, "reference tracking step"),
				Warmup:      p.Int("warmup", def.Warmup, "frames of cumulative averaging"),
				Persist:     p.Int("persist", def.Persist, "frames a block must stay changed"),
			})
			if err != nil {
				return nil, err
			}
			return detectorStage(d), nil
		}})
	Register(Method{Name: "direction_histogram", Category: CatStats,
		Description: "histogram of motion directions over the run (p6); view draws the vectors",
		New: func(p *Params) (Stage, error) {
			bins := p.Int("bins", 8, "number of direction bins")
			minMag := p.Float("min_magnitude", 1, "ignore shorter vectors")
			weighted := p.Bool("weighted", false, "weight vectors by length")
			h, err := stats.NewDirectionHistogram(bins, minMag, weighted)
			if err != nil {
				return nil, err
			}
			return &directionStage{h: h}, nil
		}})
}

// directionStage accumulates motion vectors into a direction histogram.
type directionStage struct {
	h       *stats.DirectionHistogram
	vectors int
}

func (s *directionStage) Process(f *Frame) (*imgx.Gray, error) {
	view := f.Image.Clone()
	for _, v := range f.Vectors {
		if s.h.Add(float64(v.DX), float64(v.DY)) {
			s.vectors++
		}
		drawArrow(view, v.X, v.Y, v.X+v.DX*3, v.Y+v.DY*3)
	}
	return view, nil
}

func (s *directionStage) Reset() {
	s.h.Reset()
	s.vectors = 0
}

// Summary implements Summarizer.
func (s *directionStage) Summary() map[string]any {
	sum := map[string]any{
		"type":       "direction_histogram",
		"bins":       append([]float64(nil), s.h.Bins...),
		"normalized": s.h.Normalized(),
		"vectors":    s.vectors,
	}
	centers := make([]float64, len(s.h.Bins))
	for i := range centers {
		centers[i] = s.h.BinCenter(i)
	}
	sum["bin_centers"] = centers
	if a, ok := s.h.Dominant(); ok {
		sum["dominant_deg"] = a
	}
	return sum
}

func registerFourier() {
	Register(Method{Name: "fft_filter", Category: CatFourier, Denoise: true,
		Description: "frequency-domain filter of Image via the FFT (p5)",
		New: func(p *Params) (Stage, error) {
			shape, err := fourier.ParseShape(p.String("shape", "gaussian", "gaussian, butterworth or ideal"))
			if err != nil {
				return nil, err
			}
			flt := fourier.Filter{
				Shape:    shape,
				Cutoff:   p.Float("cutoff", 0.1, "cut-off in cycles per pixel, (0, 0.5]"),
				Order:    p.Int("order", 2, "Butterworth order"),
				HighPass: p.Bool("high_pass", false, "keep high frequencies instead"),
			}
			return imageStage(flt.Apply)
		}})
}

func registerGeometry() {
	Register(Method{Name: "perspective_warp", Category: CatGeometry,
		Description: "warp Mask onto a top-down floor plan; Plan = warped mask (p5)",
		New: func(p *Params) (Stage, error) {
			src := p.Points("src", [][2]float64{{0.3, 0.35}, {0.7, 0.35}, {1, 1}, {0, 1}},
				"floor quadrilateral in the image, as fractions of width/height, clockwise from top-left")
			w := p.Int("width", 160, "plan width in pixels")
			h := p.Int("height", 200, "plan height in pixels")
			interp := p.String("interp", "nearest", "nearest or bilinear")
			if len(src) != 4 {
				return nil, fmt.Errorf("src needs exactly 4 points, got %d", len(src))
			}
			if w <= 0 || h <= 0 || w > 4096 || h > 4096 {
				return nil, fmt.Errorf("plan size %dx%d must be in 1..4096", w, h)
			}
			mode := geometry.Nearest
			switch interp {
			case "nearest":
			case "bilinear":
				mode = geometry.Bilinear
			default:
				return nil, fmt.Errorf("interp %q must be nearest or bilinear", interp)
			}
			dst := [4]geometry.Point{{X: 0, Y: 0}, {X: float64(w - 1), Y: 0}, {X: float64(w - 1), Y: float64(h - 1)}, {X: 0, Y: float64(h - 1)}}
			// Validate the shape on a unit frame now; the real warper is built
			// once the frame size is known.
			if _, err := geometry.HomographyFromPoints(scalePoints(src, 100, 100), dst); err != nil {
				return nil, fmt.Errorf("src: %w", err)
			}
			var warper *geometry.Warper
			var fw, fh int
			return &stateStage{
				process: func(f *Frame) (*imgx.Gray, error) {
					if err := needMask(f); err != nil {
						return nil, err
					}
					if warper == nil || fw != f.Mask.W || fh != f.Mask.H {
						fw, fh = f.Mask.W, f.Mask.H
						hm, err := geometry.HomographyFromPoints(scalePoints(src, fw-1, fh-1), dst)
						if err != nil {
							return nil, err
						}
						if warper, err = geometry.NewWarper(hm, fw, fh, w, h, mode); err != nil {
							return nil, err
						}
					}
					plan, err := warper.Apply(f.Mask)
					if err != nil {
						return nil, err
					}
					f.Plan = plan
					return plan, nil
				},
				reset: func() { warper = nil },
			}, nil
		}})
}

func scalePoints(src [][2]float64, w, h int) [4]geometry.Point {
	var out [4]geometry.Point
	for i := range out {
		out[i] = geometry.Point{X: src[i][0] * float64(w), Y: src[i][1] * float64(h)}
	}
	return out
}

func registerKeypoints() {
	Register(Method{Name: "harris", Category: CatKeypoints,
		Description: "Harris corners of Image; writes Corners, view marks them (p6)",
		New: func(p *Params) (Stage, error) {
			def := keypoints.DefaultHarrisConfig()
			cfg := keypoints.HarrisConfig{
				K:           p.Float32("k", def.K, "Harris sensitivity"),
				Sigma:       p.Float32("sigma", def.Sigma, "window standard deviation"),
				Quality:     p.Float32("quality", def.Quality, "fraction of the strongest response"),
				MinResponse: p.Float32("min_response", def.MinResponse, "absolute response floor"),
				MinDistance: p.Int("min_distance", def.MinDistance, "minimum distance between corners"),
				MaxCorners:  p.Int("max_corners", def.MaxCorners, "cap on corners, 0 = no limit"),
			}
			if _, err := keypoints.Harris(probe(), cfg); err != nil {
				return nil, err
			}
			return funcStage(func(f *Frame) (*imgx.Gray, error) {
				c, err := keypoints.Harris(f.Image, cfg)
				if err != nil {
					return nil, err
				}
				f.Corners = c
				view := f.Image.Clone()
				for _, k := range c {
					drawCross(view, k.X, k.Y, 3)
				}
				return view, nil
			}), nil
		}})
}

// drawCross draws a white + of the given arm length.
func drawCross(g *imgx.Gray, x, y, arm int) {
	for d := -arm; d <= arm; d++ {
		if g.InBounds(x+d, y) {
			g.Set(x+d, y, 255)
		}
		if g.InBounds(x, y+d) {
			g.Set(x, y+d, 255)
		}
	}
}

// drawArrow draws a white line from (x0, y0) to (x1, y1) with a dot at the
// head (Bresenham's algorithm).
func drawArrow(g *imgx.Gray, x0, y0, x1, y1 int) {
	dx, dy := int(math.Abs(float64(x1-x0))), -int(math.Abs(float64(y1-y0)))
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	err := dx + dy
	for {
		if g.InBounds(x0, y0) {
			g.Set(x0, y0, 255)
		}
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
	drawCross(g, x1, y1, 1)
}
