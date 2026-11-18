package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"heatmap/internal/config"
	"heatmap/internal/dataset"
	"heatmap/internal/noise"
	"heatmap/internal/pipeline"
	"heatmap/internal/report"
)

func init() {
	commands["list"] = cmdList
	commands["run"] = cmdRun
}

// Default locations, relative to the repository root.
const (
	defaultPipelines = "configs/pipelines"
	defaultData      = "data"
	defaultResults   = "results"
)

func cmdList(args []string, stdout, stderr io.Writer) int {
	fs := newFlags("list", stderr)
	pipelinesDir := fs.String("pipelines", defaultPipelines, "directory with pipeline YAML files")
	dataDir := fs.String("data", defaultData, "data directory (cdnet/, frames/)")
	asJSON := fs.Bool("json", false, "print JSON instead of text")
	if code, stop := parse(fs, args); stop {
		return code
	}
	pipes, err := config.LoadPipelines(*pipelinesDir)
	if err != nil {
		return fail(stderr, err)
	}
	seqs, err := dataset.Discover(*dataDir)
	if err != nil {
		return fail(stderr, err)
	}
	if *asJSON {
		type method struct {
			pipeline.Method
			Params []pipeline.ParamSpec `json:"params"`
		}
		var ms []method
		for _, m := range pipeline.Methods() {
			ms = append(ms, method{m, m.Params()})
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(map[string]any{"methods": ms, "pipelines": pipes, "sequences": seqs}); err != nil {
			return fail(stderr, err)
		}
		return 0
	}

	tw := tabwriter.NewWriter(stdout, 2, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "METHODS")
	cat := ""
	for _, m := range pipeline.Methods() {
		if m.Category != cat {
			cat = m.Category
			_, _ = fmt.Fprintf(tw, "  [%s]\n", cat)
		}
		var ps []string
		for _, p := range m.Params() {
			ps = append(ps, fmt.Sprintf("%s=%v", p.Name, p.Default))
		}
		_, _ = fmt.Fprintf(tw, "    %s\t%s\t%s\n", m.Name, m.Description, strings.Join(ps, " "))
	}
	_, _ = fmt.Fprintln(tw, "\nPIPELINES")
	for _, p := range pipes {
		var st []string
		for _, s := range p.Stages {
			st = append(st, s.Method)
		}
		_, _ = fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\n", p.ID, p.Name, p.Owner, strings.Join(st, " → "))
	}
	_, _ = fmt.Fprintln(tw, "\nSEQUENCES")
	for _, s := range seqs {
		gt := "no GT"
		if s.HasGT {
			gt = fmt.Sprintf("GT %d–%d", s.EvalStart, s.EvalEnd)
		}
		_, _ = fmt.Fprintf(tw, "  %s\t%d frames\t%dx%d\t%s\n", s.ID, s.Frames, s.Width, s.Height, gt)
	}
	if err := tw.Flush(); err != nil {
		return fail(stderr, err)
	}
	return 0
}

func cmdRun(args []string, stdout, stderr io.Writer) int {
	fs := newFlags("run", stderr)
	pipe := fs.String("pipeline", "p1", "pipeline id (from -pipelines) or path to a YAML file")
	input := fs.String("input", "synthetic/street", "sequence: synthetic/<name>, cdnet/<cat>/<seq>, frames/<name> or a path")
	pipelinesDir := fs.String("pipelines", defaultPipelines, "directory with pipeline YAML files")
	dataDir := fs.String("data", defaultData, "data directory (cdnet/, frames/)")
	out := fs.String("out", defaultResults, "results directory; the run goes into <out>/<run_id>")
	noiseSpec := fs.String("noise", "none", "synthetic noise: none, gaussian:<sigma> or salt_pepper:<fraction>")
	seed := fs.Uint64("seed", 1, "noise seed")
	start := fs.Int("start", -1, "first frame (-1: shortly before the ground truth starts)")
	frames := fs.Int("frames", 0, "number of frames (0: to the end)")
	saveFrames := fs.Bool("save-frames", false, "save every frame's stage images (for the web viewer)")
	decay := fs.Float64("decay", 1, "heatmap decay per frame in (0, 1]")
	cmap := fs.String("colormap", "inferno", "heatmap colors: inferno or jet")
	if code, stop := parse(fs, args); stop {
		return code
	}

	cfg, err := config.FindPipeline(*pipelinesDir, *pipe)
	if err != nil {
		return fail(stderr, err)
	}
	seq, err := dataset.OpenInput(*dataDir, *input)
	if err != nil {
		return fail(stderr, err)
	}
	ns, err := noise.Parse(*noiseSpec)
	if err != nil {
		return fail(stderr, err)
	}
	ns.Seed = *seed
	runID := pipeline.NewRunID(cfg.ID, seq.Info().Name, time.Now())

	ctx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stopSignals()
	res, err := pipeline.Run(ctx, pipeline.Options{
		Pipeline: cfg, Sequence: seq, Noise: ns, Start: *start, Frames: *frames,
		OutDir: filepath.Join(*out, runID), RunID: runID, SaveFrames: *saveFrames,
		Decay: float32(*decay), Colormap: *cmap,
		Progress: func(p pipeline.Progress) {
			if p.Done%25 == 0 || p.Done == p.Total {
				_, _ = fmt.Fprintf(stderr, "\r%s: frame %d/%d, %.0f fps", cfg.ID, p.Done, p.Total, p.FPS)
			}
		},
	})
	_, _ = fmt.Fprintln(stderr)
	if err != nil {
		return fail(stderr, err)
	}
	printSummary(stdout, &res.Run.Summary, filepath.Join(*out, runID))
	return 0
}

func printSummary(w io.Writer, s *report.Summary, dir string) {
	f := func(v report.JSONFloat) string {
		if math.IsNaN(float64(v)) {
			return "—"
		}
		return fmt.Sprintf("%.3f", float64(v))
	}
	_, _ = fmt.Fprintf(w, "run       %s\n", s.RunID)
	_, _ = fmt.Fprintf(w, "pipeline  %s (%s)\n", s.Pipeline, s.PipelineName)
	_, _ = fmt.Fprintf(w, "sequence  %s, frames %d–%d, noise %s\n", s.Sequence, s.Start, s.Start+s.Frames-1, s.Noise)
	_, _ = fmt.Fprintf(w, "metrics   precision %s  recall %s  F1 %s  IoU %s  (%d frames with GT)\n",
		f(s.Precision), f(s.Recall), f(s.F1), f(s.IoU), s.EvalFrames)
	if !math.IsNaN(float64(s.PSNR)) {
		_, _ = fmt.Fprintf(w, "denoise   PSNR %.2f dB  SSIM %.3f\n", float64(s.PSNR), float64(s.SSIM))
	}
	_, _ = fmt.Fprintf(w, "speed     %.1f fps\n", s.FPS)
	keys := make([]string, 0, len(s.Extra))
	for k := range s.Extra {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if m, ok := s.Extra[k].(map[string]any); ok {
			if d, ok := m["dominant_deg"]; ok {
				_, _ = fmt.Fprintf(w, "direction dominant %v° (%s)\n", d, k)
			}
		}
	}
	_, _ = fmt.Fprintf(w, "output    %s\n", dir)
}
