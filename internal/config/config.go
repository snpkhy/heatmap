package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"heatmap/internal/noise"
)

// Stage is one step of a pipeline: a registered method and its parameters.
type Stage struct {
	Method string         `yaml:"method" json:"method"`
	Params map[string]any `yaml:"params,omitempty" json:"params,omitempty"`
}

// Pipeline is a named sequence of stages, loaded from configs/pipelines/<id>.yaml.
type Pipeline struct {
	ID          string  `yaml:"id" json:"id"`
	Name        string  `yaml:"name" json:"name"`
	Owner       string  `yaml:"owner,omitempty" json:"owner,omitempty"`
	Description string  `yaml:"description,omitempty" json:"description,omitempty"`
	Stages      []Stage `yaml:"stages" json:"stages"`
	// Path is the file the pipeline was loaded from (not part of the YAML).
	Path string `yaml:"-" json:"-"`
}

// Validate checks the structure. Whether the methods exist and their
// parameters are valid is checked when the pipeline is built.
func (p *Pipeline) Validate() error {
	if strings.TrimSpace(p.ID) == "" {
		return errors.New("config: pipeline needs an id")
	}
	if len(p.Stages) == 0 {
		return fmt.Errorf("config: pipeline %s has no stages", p.ID)
	}
	for i, s := range p.Stages {
		if strings.TrimSpace(s.Method) == "" {
			return fmt.Errorf("config: pipeline %s: stage %d has no method", p.ID, i+1)
		}
	}
	return nil
}

// ParsePipeline decodes a pipeline from YAML. Unknown keys are errors, so a
// typo such as "parms:" does not silently fall back to defaults.
func ParsePipeline(data []byte) (*Pipeline, error) {
	var p Pipeline
	if err := decodeStrict(data, &p); err != nil {
		return nil, fmt.Errorf("config: pipeline: %w", err)
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return &p, nil
}

// LoadPipeline reads and validates one pipeline file.
func LoadPipeline(path string) (*Pipeline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	p, err := ParsePipeline(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	p.Path = path
	return p, nil
}

// LoadPipelines reads every *.yaml / *.yml file in dir, sorted by id.
// Duplicate ids are an error.
func LoadPipelines(dir string) ([]*Pipeline, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	var out []*Pipeline
	seen := map[string]string{}
	for _, e := range entries {
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if e.IsDir() || (ext != ".yaml" && ext != ".yml") {
			continue
		}
		p, err := LoadPipeline(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		if other, dup := seen[p.ID]; dup {
			return nil, fmt.Errorf("config: pipeline id %q is used by both %s and %s", p.ID, other, p.Path)
		}
		seen[p.ID] = p.Path
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// FindPipeline resolves ref either as a path to a YAML file or as the id of
// a pipeline in dir.
func FindPipeline(dir, ref string) (*Pipeline, error) {
	if st, err := os.Stat(ref); err == nil && !st.IsDir() {
		return LoadPipeline(ref)
	}
	all, err := LoadPipelines(dir)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, p := range all {
		if p.ID == ref {
			return p, nil
		}
		ids = append(ids, p.ID)
	}
	return nil, fmt.Errorf("config: no pipeline %q in %s (have %s)", ref, dir, strings.Join(ids, ", "))
}

// Experiment runs every pipeline on every sequence with every noise setting.
type Experiment struct {
	Name      string   `yaml:"name" json:"name"`
	Pipelines []string `yaml:"pipelines" json:"pipelines"` // ids or paths
	Sequences []string `yaml:"sequences" json:"sequences"` // dataset ids
	Noise     []string `yaml:"noise,omitempty" json:"noise,omitempty"`
	// Frames is the number of frames per run; 0 means the whole sequence.
	Frames int `yaml:"frames,omitempty" json:"frames,omitempty"`
	// Start is the first frame; −1 (the default) starts shortly before the
	// frames with ground truth, see pipeline.AutoStart.
	Start   *int   `yaml:"start,omitempty" json:"start,omitempty"`
	Seed    uint64 `yaml:"seed,omitempty" json:"seed,omitempty"`
	Workers int    `yaml:"workers,omitempty" json:"workers,omitempty"`
	// SaveFrames stores per-frame stage images (large; off by default).
	SaveFrames bool `yaml:"save_frames,omitempty" json:"save_frames,omitempty"`
}

// StartFrame returns Start, or −1 when it is not set.
func (e *Experiment) StartFrame() int {
	if e.Start == nil {
		return -1
	}
	return *e.Start
}

// NoiseSpecs parses Noise (an empty list means no noise) and applies Seed.
func (e *Experiment) NoiseSpecs() ([]noise.Spec, error) {
	if len(e.Noise) == 0 {
		return []noise.Spec{{Type: noise.None, Seed: e.Seed}}, nil
	}
	out := make([]noise.Spec, 0, len(e.Noise))
	for _, s := range e.Noise {
		spec, err := noise.Parse(s)
		if err != nil {
			return nil, err
		}
		spec.Seed = e.Seed
		out = append(out, spec)
	}
	return out, nil
}

// Validate checks the experiment.
func (e *Experiment) Validate() error {
	switch {
	case len(e.Pipelines) == 0:
		return errors.New("config: experiment needs at least one pipeline")
	case len(e.Sequences) == 0:
		return errors.New("config: experiment needs at least one sequence")
	case e.Frames < 0 || e.Workers < 0:
		return fmt.Errorf("config: frames (%d) and workers (%d) must not be negative", e.Frames, e.Workers)
	case e.Start != nil && *e.Start < -1:
		return fmt.Errorf("config: start %d must be -1 (auto) or a frame index", *e.Start)
	}
	_, err := e.NoiseSpecs()
	return err
}

// LoadExperiment reads and validates an experiment file.
func LoadExperiment(path string) (*Experiment, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	var e Experiment
	if err := decodeStrict(data, &e); err != nil {
		return nil, fmt.Errorf("config: %s: %w", path, err)
	}
	if e.Name == "" {
		e.Name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if err := e.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &e, nil
}

// Expect lists the thresholds a test case must meet. Unset thresholds are not checked.
type Expect struct {
	MinF1        *float64 `yaml:"min_f1,omitempty" json:"min_f1,omitempty"`
	MinPrecision *float64 `yaml:"min_precision,omitempty" json:"min_precision,omitempty"`
	MinRecall    *float64 `yaml:"min_recall,omitempty" json:"min_recall,omitempty"`
	MinIoU       *float64 `yaml:"min_iou,omitempty" json:"min_iou,omitempty"`
	MinFPS       *float64 `yaml:"min_fps,omitempty" json:"min_fps,omitempty"`
	// MinPSNR applies to the denoising stages (filtering, fourier): their
	// output compared with the clean frame.
	MinPSNR *float64 `yaml:"min_psnr,omitempty" json:"min_psnr,omitempty"`
}

// TestCase is a reference run with expected metric thresholds.
type TestCase struct {
	ID          string `yaml:"id" json:"id"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Pipeline    string `yaml:"pipeline" json:"pipeline"`
	Sequence    string `yaml:"sequence" json:"sequence"`
	Noise       string `yaml:"noise,omitempty" json:"noise,omitempty"`
	Seed        uint64 `yaml:"seed,omitempty" json:"seed,omitempty"`
	Frames      int    `yaml:"frames,omitempty" json:"frames,omitempty"`
	Start       *int   `yaml:"start,omitempty" json:"start,omitempty"`
	Expect      Expect `yaml:"expect" json:"expect"`
}

// TestSuite is the content of configs/testcases.yaml.
type TestSuite struct {
	Cases []TestCase `yaml:"cases" json:"cases"`
}

// LoadTestSuite reads and validates a test case file.
func LoadTestSuite(path string) (*TestSuite, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	var s TestSuite
	if err := decodeStrict(data, &s); err != nil {
		return nil, fmt.Errorf("config: %s: %w", path, err)
	}
	if len(s.Cases) == 0 {
		return nil, fmt.Errorf("config: %s has no test cases", path)
	}
	seen := map[string]bool{}
	for i, c := range s.Cases {
		switch {
		case c.ID == "" || c.Pipeline == "" || c.Sequence == "":
			return nil, fmt.Errorf("config: %s: case %d needs id, pipeline and sequence", path, i+1)
		case seen[c.ID]:
			return nil, fmt.Errorf("config: %s: duplicate case id %q", path, c.ID)
		case c.Frames < 0:
			return nil, fmt.Errorf("config: %s: case %s: frames must not be negative", path, c.ID)
		}
		if _, err := noise.Parse(c.Noise); err != nil {
			return nil, fmt.Errorf("config: %s: case %s: %w", path, c.ID, err)
		}
		seen[c.ID] = true
	}
	return &s, nil
}

func decodeStrict(data []byte, v any) error {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(v); err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("empty document")
		}
		return err
	}
	// A second document ("---") would otherwise be silently ignored.
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return err
		}
		return errors.New("only one YAML document is allowed")
	}
	return nil
}
