package experiment

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"heatmap/internal/config"
	"heatmap/internal/report"
)

var env = Env{PipelinesDir: "../../configs/pipelines", DataDir: "testdata-missing"}

func ptr(v float64) *float64 { return &v }

func TestJobsCrossProduct(t *testing.T) {
	e := &config.Experiment{
		Pipelines: []string{"p1", "p3"},
		Sequences: []string{"synthetic/street", "synthetic/night"},
		Noise:     []string{"none", "gaussian:5", "salt_pepper:0.01"},
		Frames:    10,
		Seed:      9,
	}
	jobs, err := Jobs(env, e)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 2*2*3 {
		t.Fatalf("%d jobs, want 12", len(jobs))
	}
	if jobs[0].Pipeline.ID != "p1" || jobs[0].Sequence.Info().ID != "synthetic/street" ||
		jobs[1].Noise.Level != 5 || jobs[1].Noise.Seed != 9 || jobs[0].Start != -1 || jobs[0].Frames != 10 {
		t.Errorf("job order or fields: %+v / %+v", jobs[0], jobs[1])
	}
	for _, bad := range []*config.Experiment{
		{Pipelines: []string{"p99"}, Sequences: []string{"synthetic/street"}},
		{Pipelines: []string{"p1"}, Sequences: []string{"synthetic/ocean"}},
		{Pipelines: []string{"p1"}, Sequences: []string{"synthetic/street"}, Noise: []string{"fog:1"}},
	} {
		if _, err := Jobs(env, bad); err == nil {
			t.Errorf("%+v: want error", bad)
		}
	}
}

func TestRunWritesCombinedOutputs(t *testing.T) {
	out := t.TempDir()
	e := &config.Experiment{
		Name:      "unit",
		Pipelines: []string{"p8", "p1"},
		Sequences: []string{"synthetic/night"},
		Noise:     []string{"none", "gaussian:8"},
		Frames:    20,
		Workers:   3,
	}
	var calls int
	res, err := Run(context.Background(), env, e, out, func(done, total int, r JobResult) {
		calls++
		if total != 4 || done < 1 || done > 4 || r.Err != nil {
			t.Errorf("progress %d/%d err %v", done, total, r.Err)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 4 || len(res.Summaries) != 4 || len(res.Failures) != 0 || res.Workers != 3 {
		t.Fatalf("result %+v", res)
	}
	// Sorted by pipeline, then noise level.
	if res.Summaries[0].Pipeline != "p1" || res.Summaries[0].Noise.Level != 0 || res.Summaries[3].Pipeline != "p8" {
		t.Errorf("order: %s %v … %s", res.Summaries[0].Pipeline, res.Summaries[0].Noise, res.Summaries[3].Pipeline)
	}
	if !strings.HasPrefix(filepath.Base(res.Dir), "unit-") {
		t.Errorf("dir %s", res.Dir)
	}
	for _, f := range []string{"metrics.csv", "summary.csv", "experiment.json"} {
		if _, err := os.Stat(filepath.Join(res.Dir, f)); err != nil {
			t.Errorf("missing %s", f)
		}
	}
	entries, _ := os.ReadDir(res.Dir)
	runs := 0
	for _, en := range entries {
		if en.IsDir() {
			runs++
			if _, err := os.Stat(filepath.Join(res.Dir, en.Name(), "run.json")); err != nil {
				t.Errorf("run %s has no run.json", en.Name())
			}
		}
	}
	if runs != 4 {
		t.Errorf("%d run directories, want 4", runs)
	}
	data, _ := os.ReadFile(filepath.Join(res.Dir, "experiment.json"))
	var back Result
	if err := json.Unmarshal(data, &back); err != nil || len(back.Summaries) != 4 {
		t.Errorf("experiment.json: %v", err)
	}
}

func TestRunJobsCancelled(t *testing.T) {
	jobs, err := PipelineJobs(env, "synthetic/street", 5)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, r := range RunJobs(ctx, jobs, 2, "", false, nil) {
		if r.Err == nil {
			t.Fatal("cancelled jobs must report an error")
		}
	}
	e := &config.Experiment{Name: "c", Pipelines: []string{"p1"}, Sequences: []string{"synthetic/street"}}
	if _, err := Run(ctx, env, e, t.TempDir(), nil); err == nil {
		t.Error("cancelled experiment must fail")
	}
}

func TestCheck(t *testing.T) {
	s := &report.Summary{F1: 0.8, Precision: 0.9, Recall: 0.7, IoU: 0.6, PSNR: report.JSONFloat(math.NaN()), FPS: 50}
	if f := Check(config.Expect{MinF1: ptr(0.75), MinFPS: ptr(40)}, s); len(f) != 0 {
		t.Errorf("unexpected failures %v", f)
	}
	f := Check(config.Expect{MinF1: ptr(0.85), MinRecall: ptr(0.8), MinPrecision: ptr(0.95), MinIoU: ptr(0.7),
		MinPSNR: ptr(20), MinFPS: ptr(100)}, s)
	if len(f) != 6 {
		t.Errorf("failures %v, want 6 (PSNR NaN must fail too)", f)
	}
}

func TestCheckCases(t *testing.T) {
	suite := &config.TestSuite{Cases: []config.TestCase{
		{ID: "ok", Pipeline: "p1", Sequence: "synthetic/street", Frames: 40, Expect: config.Expect{MinF1: ptr(0.5)}},
		{ID: "too-strict", Pipeline: "p1", Sequence: "synthetic/street", Frames: 40, Expect: config.Expect{MinF1: ptr(0.999)}},
		{ID: "noisy", Pipeline: "p5", Sequence: "synthetic/street", Noise: "gaussian:10", Seed: 1, Frames: 40,
			Expect: config.Expect{MinPSNR: ptr(20)}},
	}}
	res, err := CheckCases(context.Background(), env, suite, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !res[0].Passed || res[1].Passed || !res[2].Passed || res[1].Summary == nil || len(res[1].Failures) != 1 {
		t.Errorf("results %+v", res)
	}
	for _, bad := range []config.TestCase{
		{ID: "a", Pipeline: "p99", Sequence: "synthetic/street"},
		{ID: "b", Pipeline: "p1", Sequence: "synthetic/ocean"},
		{ID: "c", Pipeline: "p1", Sequence: "synthetic/street", Noise: "fog"},
	} {
		if _, err := CheckCases(context.Background(), env, &config.TestSuite{Cases: []config.TestCase{bad}}, 1); err == nil {
			t.Errorf("case %s: want error", bad.ID)
		}
	}
	// A run error (start past the end) is a failed case, not an error.
	start := 999
	res, err = CheckCases(context.Background(), env, &config.TestSuite{Cases: []config.TestCase{
		{ID: "late", Pipeline: "p1", Sequence: "synthetic/street", Start: &start},
	}}, 1)
	if err != nil || res[0].Passed || len(res[0].Failures) != 1 {
		t.Errorf("late start: %+v %v", res, err)
	}
}

func TestShippedTestCasesParse(t *testing.T) {
	suite, err := config.LoadTestSuite("../../configs/testcases.yaml")
	if err != nil {
		t.Fatal(err)
	}
	pipes := map[string]bool{}
	for _, c := range suite.Cases {
		pipes[c.Pipeline] = true
	}
	if len(pipes) != 8 {
		t.Errorf("test cases cover %d pipelines, want all 8", len(pipes))
	}
	for _, name := range []string{"main.yaml", "quick.yaml"} {
		e, err := config.LoadExperiment(filepath.Join("../../configs/experiments", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Jobs(env, e); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestBench(t *testing.T) {
	jobs, err := PipelineJobs(env, "synthetic/night", 10)
	if err != nil {
		t.Fatal(err)
	}
	res, err := Bench(context.Background(), jobs[:3], 3)
	if err != nil {
		t.Fatal(err)
	}
	if res.Jobs != 3 || res.Frames != 30 || res.SequentialFPS <= 0 || res.ParallelFPS <= 0 || res.Speedup <= 0 ||
		len(res.PerPipelineFPS) != 3 {
		t.Errorf("%+v", res)
	}
	if _, err := Bench(context.Background(), nil, 1); err == nil {
		t.Error("no jobs: want error")
	}
	if _, err := PipelineJobs(env, "synthetic/ocean", 1); err == nil {
		t.Error("bad sequence: want error")
	}
	if _, err := PipelineJobs(Env{PipelinesDir: "missing"}, "synthetic/street", 1); err == nil {
		t.Error("bad pipelines dir: want error")
	}
}
