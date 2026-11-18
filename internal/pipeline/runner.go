package pipeline

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"heatmap/internal/config"
	"heatmap/internal/dataset"
	"heatmap/internal/heatmap"
	"heatmap/internal/imgx"
	"heatmap/internal/metrics"
	"heatmap/internal/noise"
	"heatmap/internal/report"
)

// AutoWarmup is how many frames before the first ground-truth frame a run
// starts when Options.Start is −1, so background models have time to settle.
const AutoWarmup = 50

// AutoStart returns the first frame to process for a sequence: AutoWarmup
// frames before its ground truth begins, or 0 without ground truth.
func AutoStart(info dataset.Info) int {
	if !info.HasGT {
		return 0
	}
	return max(0, info.EvalStart-AutoWarmup)
}

// Options configure one run.
type Options struct {
	Pipeline *config.Pipeline
	Sequence dataset.Sequence
	Noise    noise.Spec
	// Start is the first frame; −1 means AutoStart.
	Start int
	// Frames is the number of frames to process; 0 means to the end.
	Frames int
	// OutDir is the directory of this run's outputs; empty writes nothing.
	OutDir string
	// RunID names the run; empty generates one.
	RunID string
	// SaveFrames writes every frame's input, stage outputs, mask and ground
	// truth as PNGs under OutDir/frames/<index>/ for the frame viewer.
	SaveFrames bool
	// Decay is the heatmap decay in (0, 1]; 0 means 1 (plain counting).
	Decay float32
	// Colormap is the heatmap color map name; empty means inferno.
	Colormap string
	// Progress, if set, is called after every frame.
	Progress func(Progress)
}

// Progress reports how far a run is.
type Progress struct {
	RunID string  `json:"run_id"`
	Done  int     `json:"done"`
	Total int     `json:"total"`
	Frame int     `json:"frame"`
	FPS   float64 `json:"fps"`
}

// Result is what Run returns besides the files it writes.
type Result struct {
	Run  *report.Run
	Rows []report.Row
	// Heatmap is the normalized camera-view heatmap; PlanHeatmap the
	// floor-plan one, or nil when no geometry stage ran.
	Heatmap, PlanHeatmap *imgx.Gray
}

var unsafeID = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

// NewRunID returns a unique, URL- and path-safe run id such as
// "20261119-143005-p1-street-a1b2".
func NewRunID(pipelineID, sequenceName string, now time.Time) string {
	var b [2]byte
	_, _ = rand.Read(b[:])
	parts := []string{now.Format("20060102-150405"), pipelineID, sequenceName, hex.EncodeToString(b[:])}
	for i, p := range parts {
		parts[i] = strings.Trim(unsafeID.ReplaceAllString(p, "_"), "_")
	}
	return strings.Join(parts, "-")
}

// ValidRunID reports whether id is a safe run id (used by the HTTP API).
func ValidRunID(id string) bool {
	return id != "" && len(id) <= 128 && !unsafeID.MatchString(id)
}

// runState accumulates metrics during a run.
type runState struct {
	opts        Options
	p           *Pipeline
	info        dataset.Info
	stages      []StageInfo
	heat, plan  *heatmap.Accumulator
	conf        metrics.Confusion
	evalFrames  int
	psnrSum     float64
	ssimSum     float64
	psnrN       int
	stageTotal  []time.Duration
	procTotal   time.Duration
	rows        []report.Row
	frames      []report.FrameResult
	firstFrame  *imgx.RGB
	denoiseStep int // index of the first denoising stage, −1 if none
}

// Run processes a sequence with a pipeline: noise is added to every frame,
// the stages run, the mask is compared with ground truth, the heatmap is
// accumulated and, with OutDir set, everything is written there.
func Run(ctx context.Context, opts Options) (*Result, error) {
	if opts.Pipeline == nil || opts.Sequence == nil {
		return nil, errors.New("pipeline: Run needs a pipeline and a sequence")
	}
	if err := opts.Noise.Validate(); err != nil {
		return nil, err
	}
	p, err := Build(opts.Pipeline)
	if err != nil {
		return nil, err
	}
	info := opts.Sequence.Info()
	start := opts.Start
	if start < 0 {
		start = AutoStart(info)
	}
	if start >= info.Frames {
		return nil, fmt.Errorf("pipeline: start %d is past the end of %s (%d frames)", start, info.ID, info.Frames)
	}
	end := info.Frames
	if opts.Frames > 0 {
		end = min(end, start+opts.Frames)
	}
	if opts.RunID == "" {
		opts.RunID = NewRunID(opts.Pipeline.ID, info.Name, time.Now())
	}
	decay := opts.Decay
	if decay == 0 {
		decay = 1
	}
	heat, err := heatmap.NewAccumulator(decay)
	if err != nil {
		return nil, err
	}
	plan, _ := heatmap.NewAccumulator(decay)

	st := &runState{opts: opts, p: p, info: info, stages: p.Stages(), heat: heat, plan: plan, denoiseStep: -1}
	st.stageTotal = make([]time.Duration, len(st.stages))
	for i, s := range st.stages {
		if s.Denoise {
			st.denoiseStep = i
			break
		}
	}

	for i := start; i < end; i++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := st.frame(i); err != nil {
			return nil, err
		}
		if opts.Progress != nil {
			done := i - start + 1
			opts.Progress(Progress{RunID: opts.RunID, Done: done, Total: end - start, Frame: i,
				FPS: metrics.FPS(done, st.procTotal)})
		}
	}
	return st.finish(start, end-start)
}

func (st *runState) frame(i int) error {
	clean, err := st.opts.Sequence.Frame(i)
	if err != nil {
		return err
	}
	if st.firstFrame == nil {
		st.firstFrame = clean
	}
	input := st.opts.Noise.Apply(clean, i)
	f := NewFrame(i, input)
	outs, err := st.p.Process(f)
	if err != nil {
		return err
	}
	var frameTime time.Duration
	for k, o := range outs {
		st.stageTotal[k] += o.Duration
		frameTime += o.Duration
	}
	st.procTotal += frameTime

	gt, err := st.opts.Sequence.GroundTruth(i)
	if err != nil {
		return err
	}
	if err := st.heat.Add(f.Mask); err != nil {
		return err
	}
	if f.Plan != nil {
		if err := st.plan.Add(f.Plan); err != nil {
			return err
		}
	}

	base := report.NaNRow()
	base.RunID, base.Pipeline, base.Owner = st.opts.RunID, st.opts.Pipeline.ID, st.opts.Pipeline.Owner
	base.Sequence, base.Category = st.info.ID, st.info.Category
	base.NoiseType, base.NoiseLevel, base.Frame = st.opts.Noise.Type, st.opts.Noise.Level, i
	if base.NoiseType == "" {
		base.NoiseType = noise.None
	}

	fr := report.FrameResult{Frame: i, TimeMS: metrics.Milliseconds(frameTime), Moving: movingFraction(f.Mask),
		Precision: nan(), Recall: nan(), F1: nan(), IoU: nan(), PSNR: nan()}

	// Per-stage rows: time, plus denoising quality against the clean frame.
	var cleanLuma *imgx.Gray
	for k, s := range st.stages {
		row := base
		row.Stage = s.Key
		row.TimeMS = metrics.Milliseconds(outs[k].Duration)
		if s.Denoise && outs[k].View != nil {
			if cleanLuma == nil {
				cleanLuma = clean.Luma()
			}
			if err := denoiseMetrics(&row, outs[k].View, cleanLuma); err != nil {
				return err
			}
			if k == st.denoiseStep {
				st.psnrSum += row.PSNR
				st.ssimSum += row.SSIM
				st.psnrN++
				fr.PSNR = report.JSONFloat(row.PSNR)
			}
		}
		st.rows = append(st.rows, row)
	}

	final := base
	final.Stage = report.StageFinal
	final.TimeMS = metrics.Milliseconds(frameTime)
	if gt != nil {
		c, err := metrics.CompareMask(f.Mask, gt)
		if err != nil {
			return fmt.Errorf("frame %d: ground truth: %w", i, err)
		}
		st.conf = st.conf.Add(c)
		st.evalFrames++
		final.Precision, final.Recall, final.F1, final.IoU = c.Precision(), c.Recall(), c.F1(), c.IoU()
		fr.HasGT = true
		fr.Precision, fr.Recall = report.JSONFloat(final.Precision), report.JSONFloat(final.Recall)
		fr.F1, fr.IoU = report.JSONFloat(final.F1), report.JSONFloat(final.IoU)
	}
	st.rows = append(st.rows, final)
	st.frames = append(st.frames, fr)

	if st.opts.SaveFrames && st.opts.OutDir != "" {
		return st.saveFrame(i, input, f, outs, gt)
	}
	return nil
}

// denoiseMetrics fills MSE, PSNR and SSIM of a stage output against the clean luma.
func denoiseMetrics(row *report.Row, out, clean *imgx.Gray) error {
	if !imgx.SameSize(out, clean) {
		return nil // e.g. a stage that changes the size; nothing comparable
	}
	mse, err := metrics.MSE(out, clean)
	if err != nil {
		return err
	}
	ssim, err := metrics.SSIM(out, clean)
	if err != nil {
		return err
	}
	row.MSE, row.PSNR, row.SSIM = mse, metrics.PSNR(mse), ssim
	return nil
}

func (st *runState) saveFrame(i int, input *imgx.RGB, f *Frame, outs []StageOutput, gt *imgx.Gray) error {
	dir := filepath.Join(st.opts.OutDir, "frames", fmt.Sprint(i))
	if err := imgx.SavePNG(filepath.Join(dir, "input.png"), input.ToImage()); err != nil {
		return err
	}
	for k, o := range outs {
		if o.View == nil {
			continue
		}
		if err := imgx.SavePNG(filepath.Join(dir, st.stages[k].Key+".png"), Displayable(o.View).ToImage()); err != nil {
			return err
		}
	}
	if err := imgx.SavePNG(filepath.Join(dir, "mask.png"), f.Mask.ToImage()); err != nil {
		return err
	}
	if gt != nil {
		if err := imgx.SavePNG(filepath.Join(dir, "gt.png"), gt.ToImage()); err != nil {
			return err
		}
	}
	return nil
}

func (st *runState) finish(start, n int) (*Result, error) {
	s := report.Summary{
		RunID:        st.opts.RunID,
		Pipeline:     st.opts.Pipeline.ID,
		PipelineName: st.opts.Pipeline.Name,
		Owner:        st.opts.Pipeline.Owner,
		Sequence:     st.info.ID,
		Category:     st.info.Category,
		Noise:        st.opts.Noise,
		Start:        start,
		Frames:       n,
		EvalFrames:   st.evalFrames,
		Width:        st.info.Width,
		Height:       st.info.Height,
		Precision:    nan(), Recall: nan(), F1: nan(), IoU: nan(), PSNR: nan(), SSIM: nan(),
		FPS:       metrics.FPS(n, st.procTotal),
		TotalMS:   metrics.Milliseconds(st.procTotal),
		Extra:     st.p.Summaries(),
		CreatedAt: time.Now().UTC(),
	}
	if s.Noise.Type == "" {
		s.Noise.Type = noise.None
	}
	if st.evalFrames > 0 {
		s.Precision = report.JSONFloat(st.conf.Precision())
		s.Recall = report.JSONFloat(st.conf.Recall())
		s.F1 = report.JSONFloat(st.conf.F1())
		s.IoU = report.JSONFloat(st.conf.IoU())
	}
	if st.psnrN > 0 {
		s.PSNR = report.JSONFloat(st.psnrSum / float64(st.psnrN))
		s.SSIM = report.JSONFloat(st.ssimSum / float64(st.psnrN))
	}
	keys := make([]string, len(st.stages))
	for k, info := range st.stages {
		keys[k] = info.Key
		s.Stages = append(s.Stages, report.StageTiming{Key: info.Key, Method: info.Method, Category: info.Category,
			MeanMS: metrics.Milliseconds(st.stageTotal[k]) / float64(max(n, 1))})
	}
	s.HasPlan = st.plan.Frames() > 0

	res := &Result{
		Run: &report.Run{Summary: s, Pipeline: st.opts.Pipeline, StageKeys: keys, Frames: st.frames,
			SavedFrames: st.opts.SaveFrames && st.opts.OutDir != ""},
		Rows:    st.rows,
		Heatmap: st.heat.Normalized(),
	}
	if s.HasPlan {
		res.PlanHeatmap = st.plan.Normalized()
	}
	if st.opts.OutDir == "" {
		return res, nil
	}
	return res, st.write(res)
}

func (st *runState) write(res *Result) error {
	dir := st.opts.OutDir
	if err := report.WriteMetricsCSV(filepath.Join(dir, "metrics.csv"), res.Rows); err != nil {
		return err
	}
	if err := report.WriteSummaryCSV(filepath.Join(dir, "summary.csv"), []report.Summary{res.Run.Summary}); err != nil {
		return err
	}
	color, err := heatmap.Colorize(res.Heatmap, st.opts.Colormap)
	if err != nil {
		return err
	}
	if err := imgx.SavePNG(filepath.Join(dir, "heatmap.png"), color.ToImage()); err != nil {
		return err
	}
	overlay, err := heatmap.Overlay(st.firstFrame, res.Heatmap, st.opts.Colormap, 0.65, 8)
	if err != nil {
		return err
	}
	if err := imgx.SavePNG(filepath.Join(dir, "overlay.png"), overlay.ToImage()); err != nil {
		return err
	}
	if res.PlanHeatmap != nil {
		pc, err := heatmap.Colorize(res.PlanHeatmap, st.opts.Colormap)
		if err != nil {
			return err
		}
		if err := imgx.SavePNG(filepath.Join(dir, "plan_heatmap.png"), pc.ToImage()); err != nil {
			return err
		}
	}
	if err := imgx.SavePNG(filepath.Join(dir, "background.png"), st.firstFrame.ToImage()); err != nil {
		return err
	}
	// run.json last: its presence marks a complete run.
	return report.WriteRunJSON(filepath.Join(dir, "run.json"), res.Run)
}

// Displayable maps a stage output to [0, 255] for viewing: images already in
// that range are kept, others (gradient magnitudes, high-pass output) are
// linearly stretched from their min–max range.
func Displayable(g *imgx.Gray) *imgx.Gray {
	lo, hi := float32(math.Inf(1)), float32(math.Inf(-1))
	for _, v := range g.Pix {
		lo, hi = min(lo, v), max(hi, v)
	}
	if lo >= 0 && hi <= 255 {
		return g
	}
	out := imgx.NewGray(g.W, g.H)
	if hi == lo {
		return out
	}
	scale := 255 / (hi - lo)
	for i, v := range g.Pix {
		out.Pix[i] = (v - lo) * scale
	}
	return out
}

func movingFraction(mask *imgx.Gray) float64 {
	n := 0
	for _, v := range mask.Pix {
		if v >= 128 {
			n++
		}
	}
	return float64(n) / float64(len(mask.Pix))
}

func nan() report.JSONFloat { return report.JSONFloat(math.NaN()) }
