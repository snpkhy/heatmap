package experiment

import (
	"context"
	"fmt"
	"math"
	"runtime"
	"time"

	"heatmap/internal/config"
	"heatmap/internal/dataset"
	"heatmap/internal/noise"
	"heatmap/internal/pipeline"
	"heatmap/internal/report"
)

// CaseResult is the outcome of one reference test case.
type CaseResult struct {
	Case     config.TestCase `json:"case"`
	Passed   bool            `json:"passed"`
	Failures []string        `json:"failures,omitempty"`
	Summary  *report.Summary `json:"summary,omitempty"`
}

// CheckCases runs every test case (in parallel, one worker per CPU) and
// compares the results with the thresholds of its expect block.
func CheckCases(ctx context.Context, env Env, suite *config.TestSuite, workers int) ([]CaseResult, error) {
	jobs := make([]Job, len(suite.Cases))
	results := make([]CaseResult, len(suite.Cases))
	for i, c := range suite.Cases {
		results[i].Case = c
		p, err := config.FindPipeline(env.PipelinesDir, c.Pipeline)
		if err != nil {
			return nil, fmt.Errorf("case %s: %w", c.ID, err)
		}
		s, err := dataset.OpenInput(env.DataDir, c.Sequence)
		if err != nil {
			return nil, fmt.Errorf("case %s: %w", c.ID, err)
		}
		n, err := noise.Parse(c.Noise)
		if err != nil {
			return nil, fmt.Errorf("case %s: %w", c.ID, err)
		}
		n.Seed = c.Seed
		start := -1
		if c.Start != nil {
			start = *c.Start
		}
		jobs[i] = Job{Pipeline: p, Sequence: s, Noise: n, Start: start, Frames: c.Frames}
	}
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	for i, r := range RunJobs(ctx, jobs, workers, "", false, nil) {
		if r.Err != nil {
			results[i].Failures = []string{r.Err.Error()}
			continue
		}
		sum := r.Summary
		results[i].Summary = &sum
		results[i].Failures = Check(results[i].Case.Expect, &sum)
		results[i].Passed = len(results[i].Failures) == 0
	}
	return results, ctx.Err()
}

// Check compares a run summary with expected thresholds and returns one
// message per failed threshold. A metric that could not be computed (no
// ground truth, no denoising stage) fails its threshold.
func Check(e config.Expect, s *report.Summary) []string {
	var fails []string
	atLeast := func(name string, want *float64, got float64) {
		if want == nil {
			return
		}
		if math.IsNaN(got) || got < *want {
			fails = append(fails, fmt.Sprintf("%s %.3f < %.3f", name, got, *want))
		}
	}
	atLeast("F1", e.MinF1, float64(s.F1))
	atLeast("precision", e.MinPrecision, float64(s.Precision))
	atLeast("recall", e.MinRecall, float64(s.Recall))
	atLeast("IoU", e.MinIoU, float64(s.IoU))
	atLeast("PSNR", e.MinPSNR, float64(s.PSNR))
	atLeast("FPS", e.MinFPS, s.FPS)
	return fails
}

// BenchResult compares running the same set of jobs one after another and
// on a pool of workers. Pipelines are stateful along a sequence, so the
// parallelism is across runs (pipelines, sequences, noise levels), not
// across the frames of one run.
type BenchResult struct {
	Jobs          int     `json:"jobs"`
	Frames        int     `json:"frames"`
	Workers       int     `json:"workers"`
	SequentialSec float64 `json:"sequential_s"`
	ParallelSec   float64 `json:"parallel_s"`
	SequentialFPS float64 `json:"sequential_fps"`
	ParallelFPS   float64 `json:"parallel_fps"`
	Speedup       float64 `json:"speedup"`
	// PerPipelineFPS is the single-run processing speed of each pipeline.
	PerPipelineFPS map[string]float64 `json:"per_pipeline_fps"`
}

// Bench runs jobs sequentially, then in parallel, and reports wall-clock
// throughput (frames per second over all jobs) and the speedup.
func Bench(ctx context.Context, jobs []Job, workers int) (*BenchResult, error) {
	if len(jobs) == 0 {
		return nil, fmt.Errorf("experiment: nothing to benchmark")
	}
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	res := &BenchResult{Jobs: len(jobs), Workers: workers, PerPipelineFPS: map[string]float64{}}

	start := time.Now()
	seq := RunJobs(ctx, jobs, 1, "", false, nil)
	res.SequentialSec = time.Since(start).Seconds()
	for _, r := range seq {
		if r.Err != nil {
			return nil, r.Err
		}
		res.Frames += r.Summary.Frames
		res.PerPipelineFPS[r.Summary.Pipeline] = r.Summary.FPS
	}

	start = time.Now()
	for _, r := range RunJobs(ctx, jobs, workers, "", false, nil) {
		if r.Err != nil {
			return nil, r.Err
		}
	}
	res.ParallelSec = time.Since(start).Seconds()

	res.SequentialFPS = float64(res.Frames) / res.SequentialSec
	res.ParallelFPS = float64(res.Frames) / res.ParallelSec
	res.Speedup = res.SequentialSec / res.ParallelSec
	return res, nil
}

// PipelineJobs returns one job per pipeline in dir on the given sequence.
func PipelineJobs(env Env, sequence string, frames int) ([]Job, error) {
	pipes, err := config.LoadPipelines(env.PipelinesDir)
	if err != nil {
		return nil, err
	}
	s, err := dataset.OpenInput(env.DataDir, sequence)
	if err != nil {
		return nil, err
	}
	jobs := make([]Job, 0, len(pipes))
	for _, p := range pipes {
		if _, err := pipeline.Build(p); err != nil {
			return nil, err
		}
		jobs = append(jobs, Job{Pipeline: p, Sequence: s, Noise: noise.Spec{Type: noise.None}, Start: 0, Frames: frames})
	}
	return jobs, nil
}
