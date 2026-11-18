package pipeline

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"heatmap/internal/config"
	"heatmap/internal/imgx"
	"heatmap/internal/methods/keypoints"
)

// Frame is the data that flows through the stages of a pipeline. Each stage
// reads what it needs and writes its result back:
//
//   - filtering, histogram, edges, texture and fourier stages replace Image;
//   - color stages fill Channels (and set Image to the brightness channel);
//   - segmentation, edges-based detectors and stats detectors write Mask;
//   - morphology and component filters clean Mask;
//   - keypoint stages write Corners, block matching writes Vectors and Mask;
//   - geometry stages warp Mask onto a floor plan and write Plan.
type Frame struct {
	// Index is the frame number in the sequence.
	Index int
	// RGB is the input frame (after synthetic noise, if any). Read-only.
	RGB *imgx.RGB
	// Image is the current single-channel working image. It starts as the
	// luma of RGB.
	Image *imgx.Gray
	// Channels are multi-channel features produced by color stages.
	Channels []*imgx.Gray
	// Mask is the motion mask (0/255, shadows may be 50), in camera coordinates.
	Mask *imgx.Gray
	// Plan is the motion mask warped onto a floor plan, if a geometry stage ran.
	Plan *imgx.Gray
	// Corners are keypoints found in this frame.
	Corners []keypoints.Corner
	// Vectors are motion vectors of keypoints from the previous frame to this one.
	Vectors []keypoints.Match
}

// NewFrame starts a frame from an RGB image.
func NewFrame(index int, rgb *imgx.RGB) *Frame {
	return &Frame{Index: index, RGB: rgb, Image: rgb.Luma()}
}

// Stage is one built processing step. Stateful stages (background models,
// trackers) keep their state between frames until Reset.
type Stage interface {
	// Process runs the stage on f, updating it in place, and returns an image
	// that shows the stage's output (for the frame viewer).
	Process(f *Frame) (*imgx.Gray, error)
	// Reset forgets the state learned from earlier frames.
	Reset()
}

// Summarizer is implemented by stages that collect run-level results, such
// as the motion direction histogram.
type Summarizer interface {
	Summary() map[string]any
}

// Categories of methods. The pipeline rule counts distinct categories.
const (
	CatFiltering    = "filtering"
	CatColor        = "color"
	CatHistogram    = "histogram"
	CatSegmentation = "segmentation"
	CatMorphology   = "morphology"
	CatEdges        = "edges"
	CatTexture      = "texture"
	CatStats        = "stats"
	CatFourier      = "fourier"
	CatGeometry     = "geometry"
	CatKeypoints    = "keypoints"
)

// Method is a registered processing method that can be used as a stage.
type Method struct {
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
	// Denoise marks methods whose output is compared with the clean frame
	// (MSE, PSNR, SSIM) to measure how well they remove noise.
	Denoise bool `json:"denoise"`
	// New builds a stage from parameters. It must succeed with no parameters.
	New func(p *Params) (Stage, error) `json:"-"`
}

// Params describes the method's parameters and their defaults.
func (m Method) Params() []ParamSpec {
	p := NewParams(nil)
	_, _ = m.New(p)
	return p.Specs()
}

var registry = map[string]Method{}

// Register adds a method. It panics on a duplicate name: that is a
// programming error caught by any test.
func Register(m Method) {
	if _, dup := registry[m.Name]; dup {
		panic("pipeline: method registered twice: " + m.Name)
	}
	registry[m.Name] = m
}

// Lookup returns the method with the given name.
func Lookup(name string) (Method, bool) {
	m, ok := registry[name]
	return m, ok
}

// Methods returns all methods sorted by category, then name.
func Methods() []Method {
	out := make([]Method, 0, len(registry))
	for _, m := range registry {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// Rule limits: every pipeline needs this many stages from this many categories.
const (
	MinStages     = 3
	MinCategories = 3
)

// StageInfo describes a built stage.
type StageInfo struct {
	// Key identifies the stage within the pipeline, e.g. "1-gaussian".
	Key      string `json:"key"`
	Method   string `json:"method"`
	Category string `json:"category"`
	Denoise  bool   `json:"denoise"`
}

type builtStage struct {
	info  StageInfo
	stage Stage
}

// Pipeline is a built, runnable pipeline.
type Pipeline struct {
	cfg    *config.Pipeline
	stages []builtStage
}

// ErrRule is returned when a pipeline breaks the "≥ 3 stages from ≥ 3
// categories" rule.
var ErrRule = errors.New("pipeline: rule violated")

// Build creates the stages of a pipeline config and checks the rule
// "at least MinStages stages from at least MinCategories distinct categories".
func Build(cfg *config.Pipeline) (*Pipeline, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	p := &Pipeline{cfg: cfg}
	cats := map[string]bool{}
	for i, sc := range cfg.Stages {
		m, ok := Lookup(sc.Method)
		if !ok {
			return nil, fmt.Errorf("pipeline %s: stage %d: unknown method %q (see `heatmap list`)", cfg.ID, i+1, sc.Method)
		}
		params := NewParams(sc.Params)
		st, err := m.New(params)
		if err == nil {
			err = params.Err()
		}
		if err != nil {
			return nil, fmt.Errorf("pipeline %s: stage %d (%s): %w", cfg.ID, i+1, sc.Method, err)
		}
		cats[m.Category] = true
		p.stages = append(p.stages, builtStage{
			info:  StageInfo{Key: fmt.Sprintf("%d-%s", i+1, m.Name), Method: m.Name, Category: m.Category, Denoise: m.Denoise},
			stage: st,
		})
	}
	if len(p.stages) < MinStages || len(cats) < MinCategories {
		return nil, fmt.Errorf("%w: pipeline %s has %d stages from %d categories, need at least %d from %d",
			ErrRule, cfg.ID, len(p.stages), len(cats), MinStages, MinCategories)
	}
	return p, nil
}

// Config returns the config the pipeline was built from.
func (p *Pipeline) Config() *config.Pipeline { return p.cfg }

// Stages describes the stages in order.
func (p *Pipeline) Stages() []StageInfo {
	out := make([]StageInfo, len(p.stages))
	for i, s := range p.stages {
		out[i] = s.info
	}
	return out
}

// StageOutput is what one stage produced for one frame.
type StageOutput struct {
	View     *imgx.Gray
	Duration time.Duration
}

// ErrNoMask is returned when no stage of a pipeline produced a motion mask.
var ErrNoMask = errors.New("pipeline: no stage produced a motion mask")

// Process runs all stages on one frame in order. It returns one output per
// stage. After the last stage f.Mask must be set.
func (p *Pipeline) Process(f *Frame) ([]StageOutput, error) {
	out := make([]StageOutput, len(p.stages))
	for i, s := range p.stages {
		start := time.Now()
		view, err := s.stage.Process(f)
		if err != nil {
			return nil, fmt.Errorf("frame %d, stage %s: %w", f.Index, s.info.Key, err)
		}
		out[i] = StageOutput{View: view, Duration: time.Since(start)}
	}
	if f.Mask == nil {
		return nil, fmt.Errorf("%w (pipeline %s)", ErrNoMask, p.cfg.ID)
	}
	return out, nil
}

// Reset resets every stage, e.g. before another sequence.
func (p *Pipeline) Reset() {
	for _, s := range p.stages {
		s.stage.Reset()
	}
}

// Summaries collects run-level results from stages that provide them, keyed
// by stage key.
func (p *Pipeline) Summaries() map[string]any {
	out := map[string]any{}
	for _, s := range p.stages {
		if sum, ok := s.stage.(Summarizer); ok {
			out[s.info.Key] = sum.Summary()
		}
	}
	return out
}
