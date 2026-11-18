// Package experiment runs many pipeline runs: full experiments (pipelines ×
// sequences × noise levels) in parallel, reference test cases with metric
// thresholds, and a sequential-versus-parallel benchmark.
package experiment

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"time"

	"heatmap/internal/config"
	"heatmap/internal/dataset"
	"heatmap/internal/noise"
	"heatmap/internal/pipeline"
	"heatmap/internal/report"
)

// Env says where pipelines and data live.
type Env struct {
	PipelinesDir string
	DataDir      string
}

// Job is one run of an experiment.
type Job struct {
	Pipeline *config.Pipeline
	Sequence dataset.Sequence
	Noise    noise.Spec
	Start    int
	Frames   int
}

// JobResult is the outcome of one job.
type JobResult struct {
	Job      Job
	Summary  report.Summary
	Rows     []report.Row
	Duration time.Duration
	Err      error
}

// Jobs expands an experiment into the cross product pipelines × sequences × noise.
func Jobs(env Env, e *config.Experiment) ([]Job, error) {
	var pipes []*config.Pipeline
	for _, ref := range e.Pipelines {
		p, err := config.FindPipeline(env.PipelinesDir, ref)
		if err != nil {
			return nil, err
		}
		if _, err := pipeline.Build(p); err != nil { // fail fast, before any run starts
			return nil, err
		}
		pipes = append(pipes, p)
	}
	var seqs []dataset.Sequence
	for _, ref := range e.Sequences {
		s, err := dataset.OpenInput(env.DataDir, ref)
		if err != nil {
			return nil, err
		}
		seqs = append(seqs, s)
	}
	specs, err := e.NoiseSpecs()
	if err != nil {
		return nil, err
	}
	var jobs []Job
	for _, p := range pipes {
		for _, s := range seqs {
			for _, n := range specs {
				jobs = append(jobs, Job{Pipeline: p, Sequence: s, Noise: n, Start: e.StartFrame(), Frames: e.Frames})
			}
		}
	}
	return jobs, nil
}

// RunJobs runs jobs on a pool of workers (0 = number of CPUs). Each job gets
// its own freshly built pipeline, so stateful stages never share state.
// outRoot, if not empty, receives one subdirectory per run. done, if set, is
// called after each job (from the worker goroutine, serialized). Results are
// returned in job order; a failed job does not stop the others.
func RunJobs(ctx context.Context, jobs []Job, workers int, outRoot string, saveFrames bool, done func(JobResult)) []JobResult {
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	results := make([]JobResult, len(jobs))
	next := make(chan int)
	var wg sync.WaitGroup
	var mu sync.Mutex
	for range min(workers, max(len(jobs), 1)) {
		wg.Go(func() {
			for i := range next {
				results[i] = runJob(ctx, jobs[i], outRoot, saveFrames)
				if done != nil {
					mu.Lock()
					done(results[i])
					mu.Unlock()
				}
			}
		})
	}
	for i := range jobs {
		if ctx.Err() != nil {
			break
		}
		next <- i
	}
	close(next)
	wg.Wait()
	for i := range results {
		if results[i].Err == nil && results[i].Summary.RunID == "" {
			results[i] = JobResult{Job: jobs[i], Err: ctx.Err()}
		}
	}
	return results
}

func runJob(ctx context.Context, j Job, outRoot string, saveFrames bool) JobResult {
	id := pipeline.NewRunID(j.Pipeline.ID, j.Sequence.Info().Name+"-"+j.Noise.String(), time.Now())
	dir := ""
	if outRoot != "" {
		dir = filepath.Join(outRoot, id)
	}
	start := time.Now()
	res, err := pipeline.Run(ctx, pipeline.Options{
		Pipeline: j.Pipeline, Sequence: j.Sequence, Noise: j.Noise, Start: j.Start, Frames: j.Frames,
		OutDir: dir, RunID: id, SaveFrames: saveFrames,
	})
	r := JobResult{Job: j, Duration: time.Since(start), Err: err}
	if err == nil {
		r.Summary, r.Rows = res.Run.Summary, res.Rows
	}
	return r
}

// Result is the outcome of an experiment.
type Result struct {
	Name      string           `json:"name"`
	Dir       string           `json:"dir"`
	Workers   int              `json:"workers"`
	Duration  float64          `json:"duration_s"`
	Summaries []report.Summary `json:"runs"`
	Failures  []string         `json:"failures,omitempty"`
}

// Run executes an experiment and writes, under outRoot/<name>-<time>/, one
// directory per run plus the combined metrics.csv, summary.csv and
// experiment.json.
func Run(ctx context.Context, env Env, e *config.Experiment, outRoot string, progress func(done, total int, r JobResult)) (*Result, error) {
	jobs, err := Jobs(env, e)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(outRoot, e.Name+"-"+time.Now().Format("20060102-150405"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("experiment: %w", err)
	}
	workers := e.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	start := time.Now()
	n := 0
	results := RunJobs(ctx, jobs, workers, dir, e.SaveFrames, func(r JobResult) {
		n++
		if progress != nil {
			progress(n, len(jobs), r)
		}
	})
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	out := &Result{Name: e.Name, Dir: dir, Workers: workers, Duration: time.Since(start).Seconds()}
	var rows []report.Row
	for _, r := range results {
		if r.Err != nil {
			out.Failures = append(out.Failures, fmt.Sprintf("%s / %s / %s: %v",
				r.Job.Pipeline.ID, r.Job.Sequence.Info().ID, r.Job.Noise, r.Err))
			continue
		}
		out.Summaries = append(out.Summaries, r.Summary)
		rows = append(rows, r.Rows...)
	}
	sort.SliceStable(out.Summaries, func(i, j int) bool {
		a, b := out.Summaries[i], out.Summaries[j]
		if a.Pipeline != b.Pipeline {
			return a.Pipeline < b.Pipeline
		}
		if a.Sequence != b.Sequence {
			return a.Sequence < b.Sequence
		}
		return a.Noise.Level < b.Noise.Level
	})
	if err := report.WriteMetricsCSV(filepath.Join(dir, "metrics.csv"), rows); err != nil {
		return nil, err
	}
	if err := report.WriteSummaryCSV(filepath.Join(dir, "summary.csv"), out.Summaries); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("experiment: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "experiment.json"), data, 0o644); err != nil {
		return nil, fmt.Errorf("experiment: %w", err)
	}
	return out, nil
}
