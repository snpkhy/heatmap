package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"heatmap/internal/noise"
)

func write(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

const p1 = `
id: p1
name: Baseline
owner: someone
stages:
  - method: gaussian
    params: {k: 5, sigma: 1.2}
  - method: running_avg_otsu
  - method: closing
    params:
      k: 3
`

func TestParsePipeline(t *testing.T) {
	p, err := ParsePipeline([]byte(p1))
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "p1" || p.Name != "Baseline" || len(p.Stages) != 3 {
		t.Fatalf("%+v", p)
	}
	if p.Stages[0].Method != "gaussian" || p.Stages[0].Params["k"] != 5 || p.Stages[0].Params["sigma"] != 1.2 {
		t.Errorf("stage 0 %+v", p.Stages[0])
	}
	if p.Stages[1].Params != nil {
		t.Errorf("stage without params: %+v", p.Stages[1])
	}
}

func TestParsePipelineErrors(t *testing.T) {
	cases := map[string]string{
		"empty":          "",
		"no id":          "stages: [{method: a}]",
		"no stages":      "id: x",
		"empty method":   "id: x\nstages: [{method: ''}]",
		"unknown key":    "id: x\nstages: [{method: a, parms: {k: 1}}]",
		"top-level typo": "id: x\nstage: [{method: a}]",
		"two documents":  "id: x\nstages: [{method: a}]\n---\nstage: []\n",
		"bad yaml":       "id: [",
	}
	for name, body := range cases {
		if _, err := ParsePipeline([]byte(body)); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func TestLoadPipelinesAndFind(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "b.yaml", strings.Replace(p1, "id: p1", "id: p2", 1))
	write(t, dir, "a.yml", p1)
	write(t, dir, "notes.txt", "ignored")
	if err := os.Mkdir(filepath.Join(dir, "sub.yaml"), 0o755); err != nil {
		t.Fatal(err)
	}
	all, err := LoadPipelines(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || all[0].ID != "p1" || all[1].ID != "p2" || all[0].Path != filepath.Join(dir, "a.yml") {
		t.Fatalf("loaded %+v", all)
	}

	p, err := FindPipeline(dir, "p2")
	if err != nil || p.ID != "p2" {
		t.Errorf("find by id: %v %v", p, err)
	}
	p, err = FindPipeline(dir, filepath.Join(dir, "a.yml"))
	if err != nil || p.ID != "p1" {
		t.Errorf("find by path: %v %v", p, err)
	}
	if _, err := FindPipeline(dir, "p9"); err == nil || !strings.Contains(err.Error(), "p1, p2") {
		t.Errorf("unknown id: %v", err)
	}

	write(t, dir, "c.yaml", p1) // duplicate id
	if _, err := LoadPipelines(dir); err == nil {
		t.Error("duplicate id: want error")
	}
	if _, err := LoadPipelines(filepath.Join(dir, "missing")); err == nil {
		t.Error("missing dir: want error")
	}
	bad := t.TempDir()
	write(t, bad, "x.yaml", "id: [")
	if _, err := LoadPipelines(bad); err == nil {
		t.Error("broken file: want error")
	}
}

func TestLoadExperiment(t *testing.T) {
	dir := t.TempDir()
	path := write(t, dir, "main.yaml", `
pipelines: [p1, p3]
sequences: [synthetic/street]
noise: [none, "gaussian:10"]
frames: 50
seed: 7
workers: 2
`)
	e, err := LoadExperiment(path)
	if err != nil {
		t.Fatal(err)
	}
	if e.Name != "main" || e.Frames != 50 || e.StartFrame() != -1 || e.Workers != 2 {
		t.Errorf("%+v", e)
	}
	specs, err := e.NoiseSpecs()
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 2 || specs[1] != (noise.Spec{Type: noise.Gaussian, Level: 10, Seed: 7}) {
		t.Errorf("noise %+v", specs)
	}

	noNoise := &Experiment{Pipelines: []string{"p1"}, Sequences: []string{"s"}, Seed: 3}
	if specs, _ := noNoise.NoiseSpecs(); len(specs) != 1 || !specs[0].IsNone() || specs[0].Seed != 3 {
		t.Errorf("default noise %+v", specs)
	}
	zero := 0
	noNoise.Start = &zero
	if noNoise.StartFrame() != 0 {
		t.Error("explicit start 0 must be kept")
	}

	for name, body := range map[string]string{
		"no pipelines": "sequences: [a]",
		"no sequences": "pipelines: [a]",
		"bad noise":    "pipelines: [a]\nsequences: [b]\nnoise: [fog:1]",
		"negative":     "pipelines: [a]\nsequences: [b]\nframes: -1",
		"bad start":    "pipelines: [a]\nsequences: [b]\nstart: -5",
		"unknown key":  "pipelines: [a]\nsequences: [b]\nframe: 3",
	} {
		if _, err := LoadExperiment(write(t, dir, "x.yaml", body)); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
	if _, err := LoadExperiment(filepath.Join(dir, "missing.yaml")); err == nil {
		t.Error("missing file: want error")
	}
}

func TestLoadTestSuite(t *testing.T) {
	dir := t.TempDir()
	path := write(t, dir, "tc.yaml", `
cases:
  - id: TC-1
    pipeline: p1
    sequence: synthetic/street
    noise: "gaussian:5"
    frames: 40
    expect: {min_f1: 0.5, min_fps: 10}
  - id: TC-2
    pipeline: p2
    sequence: synthetic/night
    expect: {}
`)
	s, err := LoadTestSuite(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Cases) != 2 || *s.Cases[0].Expect.MinF1 != 0.5 || s.Cases[0].Expect.MinRecall != nil {
		t.Errorf("%+v", s.Cases)
	}
	for name, body := range map[string]string{
		"empty":     "cases: []",
		"no id":     "cases: [{pipeline: p, sequence: s}]",
		"duplicate": "cases: [{id: a, pipeline: p, sequence: s}, {id: a, pipeline: p, sequence: s}]",
		"bad noise": "cases: [{id: a, pipeline: p, sequence: s, noise: fog}]",
		"negative":  "cases: [{id: a, pipeline: p, sequence: s, frames: -2}]",
		"typo":      "cases: [{id: a, pipeline: p, sequence: s, expect: {f1: 1}}]",
	} {
		if _, err := LoadTestSuite(write(t, dir, "x.yaml", body)); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
	if _, err := LoadTestSuite(filepath.Join(dir, "missing.yaml")); err == nil {
		t.Error("missing file: want error")
	}
}
