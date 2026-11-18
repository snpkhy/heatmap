package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/signal"
	"sort"
	"text/tabwriter"

	"heatmap/internal/config"
	"heatmap/internal/experiment"
)

func init() {
	commands["experiment"] = cmdExperiment
	commands["testcases"] = cmdTestcases
	commands["bench"] = cmdBench
}

func signalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt)
}

func fmtMetric(v float64) string {
	if math.IsNaN(v) {
		return "—"
	}
	return fmt.Sprintf("%.3f", v)
}

func cmdExperiment(args []string, stdout, stderr io.Writer) int {
	fs := newFlags("experiment", stderr)
	cfgPath := fs.String("config", "configs/experiments/main.yaml", "experiment YAML file")
	pipelinesDir := fs.String("pipelines", defaultPipelines, "directory with pipeline YAML files")
	dataDir := fs.String("data", defaultData, "data directory (cdnet/, frames/)")
	out := fs.String("out", defaultResults, "results directory")
	workers := fs.Int("workers", 0, "parallel runs (0: from the config, else number of CPUs)")
	if code, stop := parse(fs, args); stop {
		return code
	}
	e, err := config.LoadExperiment(*cfgPath)
	if err != nil {
		return fail(stderr, err)
	}
	if *workers > 0 {
		e.Workers = *workers
	}
	ctx, cancel := signalContext()
	defer cancel()
	res, err := experiment.Run(ctx, experiment.Env{PipelinesDir: *pipelinesDir, DataDir: *dataDir}, e, *out,
		func(done, total int, r experiment.JobResult) {
			var status string
			if r.Err != nil {
				status = "FAILED: " + r.Err.Error()
			} else {
				status = fmt.Sprintf("F1 %s, %.0f fps", fmtMetric(float64(r.Summary.F1)), r.Summary.FPS)
			}
			_, _ = fmt.Fprintf(stderr, "[%d/%d] %s on %s, noise %s: %s\n", done, total,
				r.Job.Pipeline.ID, r.Job.Sequence.Info().ID, r.Job.Noise, status)
		})
	if err != nil {
		return fail(stderr, err)
	}
	tw := tabwriter.NewWriter(stdout, 2, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "PIPELINE\tSEQUENCE\tNOISE\tPRECISION\tRECALL\tF1\tIOU\tPSNR\tFPS")
	for _, s := range res.Summaries {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%.0f\n", s.Pipeline, s.Sequence, s.Noise,
			fmtMetric(float64(s.Precision)), fmtMetric(float64(s.Recall)), fmtMetric(float64(s.F1)),
			fmtMetric(float64(s.IoU)), fmtMetric(float64(s.PSNR)), s.FPS)
	}
	_ = tw.Flush()
	_, _ = fmt.Fprintf(stdout, "\n%d runs in %.1fs with %d workers → %s\n", len(res.Summaries), res.Duration, res.Workers, res.Dir)
	if len(res.Failures) > 0 {
		for _, f := range res.Failures {
			_, _ = fmt.Fprintln(stderr, "failed:", f)
		}
		return 1
	}
	return 0
}

func cmdTestcases(args []string, stdout, stderr io.Writer) int {
	fs := newFlags("testcases", stderr)
	cfgPath := fs.String("config", "configs/testcases.yaml", "test case YAML file")
	pipelinesDir := fs.String("pipelines", defaultPipelines, "directory with pipeline YAML files")
	dataDir := fs.String("data", defaultData, "data directory (cdnet/, frames/)")
	workers := fs.Int("workers", 0, "parallel runs (0: number of CPUs)")
	asJSON := fs.Bool("json", false, "print JSON results")
	if code, stop := parse(fs, args); stop {
		return code
	}
	suite, err := config.LoadTestSuite(*cfgPath)
	if err != nil {
		return fail(stderr, err)
	}
	ctx, cancel := signalContext()
	defer cancel()
	results, err := experiment.CheckCases(ctx, experiment.Env{PipelinesDir: *pipelinesDir, DataDir: *dataDir}, suite, *workers)
	if err != nil {
		return fail(stderr, err)
	}
	failed := 0
	for _, r := range results {
		if !r.Passed {
			failed++
		}
	}
	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(results)
	} else {
		tw := tabwriter.NewWriter(stdout, 2, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(tw, "CASE\tPIPELINE\tSEQUENCE\tNOISE\tF1\tPSNR\tFPS\tRESULT")
		for _, r := range results {
			f1, psnr, fps := math.NaN(), math.NaN(), 0.0
			if r.Summary != nil {
				f1, psnr, fps = float64(r.Summary.F1), float64(r.Summary.PSNR), r.Summary.FPS
			}
			verdict := "PASS"
			if !r.Passed {
				verdict = fmt.Sprintf("FAIL %v", r.Failures)
			}
			noiseName := r.Case.Noise
			if noiseName == "" {
				noiseName = "none"
			}
			_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%.0f\t%s\n", r.Case.ID, r.Case.Pipeline, r.Case.Sequence,
				noiseName, fmtMetric(f1), fmtMetric(psnr), fps, verdict)
		}
		_ = tw.Flush()
		_, _ = fmt.Fprintf(stdout, "\n%d/%d test cases passed\n", len(results)-failed, len(results))
	}
	if failed > 0 {
		return 1
	}
	return 0
}

func cmdBench(args []string, stdout, stderr io.Writer) int {
	fs := newFlags("bench", stderr)
	pipelinesDir := fs.String("pipelines", defaultPipelines, "directory with pipeline YAML files")
	dataDir := fs.String("data", defaultData, "data directory (cdnet/, frames/)")
	input := fs.String("input", "synthetic/street", "sequence to benchmark on")
	frames := fs.Int("frames", 100, "frames per run")
	workers := fs.Int("workers", 0, "parallel workers (0: number of CPUs)")
	if code, stop := parse(fs, args); stop {
		return code
	}
	env := experiment.Env{PipelinesDir: *pipelinesDir, DataDir: *dataDir}
	jobs, err := experiment.PipelineJobs(env, *input, *frames)
	if err != nil {
		return fail(stderr, err)
	}
	ctx, cancel := signalContext()
	defer cancel()
	res, err := experiment.Bench(ctx, jobs, *workers)
	if err != nil {
		return fail(stderr, err)
	}
	tw := tabwriter.NewWriter(stdout, 2, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "PIPELINE\tFPS (single run)")
	ids := make([]string, 0, len(res.PerPipelineFPS))
	for id := range res.PerPipelineFPS {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		_, _ = fmt.Fprintf(tw, "%s\t%.1f\n", id, res.PerPipelineFPS[id])
	}
	_ = tw.Flush()
	_, _ = fmt.Fprintf(stdout, "\n%d runs × %d frames on %s\n", res.Jobs, *frames, *input)
	_, _ = fmt.Fprintf(stdout, "sequential  %6.2fs  %8.1f frames/s\n", res.SequentialSec, res.SequentialFPS)
	_, _ = fmt.Fprintf(stdout, "parallel    %6.2fs  %8.1f frames/s  (%d workers)\n", res.ParallelSec, res.ParallelFPS, res.Workers)
	_, _ = fmt.Fprintf(stdout, "speedup     %.2f×\n", res.Speedup)
	return 0
}
