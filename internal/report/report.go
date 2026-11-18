package report

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"heatmap/internal/noise"
)

// Stage names used in metrics rows besides the pipeline's own stage keys.
const (
	// StageFinal rows hold the mask metrics and the total time of a frame.
	StageFinal = "final"
)

// MetricsColumns is the header of metrics.csv. analysis/common.py relies on it.
var MetricsColumns = []string{
	"run_id", "pipeline", "owner", "sequence", "category", "noise_type", "noise_level",
	"frame", "stage", "mse", "psnr", "ssim", "precision", "recall", "f1", "iou", "time_ms",
}

// Row is one line of metrics.csv: one stage of one frame, or the "final"
// row of a frame. Metrics that do not apply are NaN and written as empty cells.
type Row struct {
	RunID, Pipeline, Owner, Sequence, Category string
	NoiseType                                  string
	NoiseLevel                                 float64
	Frame                                      int
	Stage                                      string
	MSE, PSNR, SSIM                            float64
	Precision, Recall, F1, IoU                 float64
	TimeMS                                     float64
}

// NaNRow returns a row with every metric set to NaN.
func NaNRow() Row {
	n := math.NaN()
	return Row{MSE: n, PSNR: n, SSIM: n, Precision: n, Recall: n, F1: n, IoU: n, TimeMS: n}
}

func num(v float64) string {
	if math.IsNaN(v) {
		return ""
	}
	if math.IsInf(v, 1) {
		return "inf"
	}
	return strconv.FormatFloat(v, 'g', 6, 64)
}

func (r Row) record() []string {
	return []string{
		r.RunID, r.Pipeline, r.Owner, r.Sequence, r.Category, r.NoiseType, num(r.NoiseLevel),
		strconv.Itoa(r.Frame), r.Stage, num(r.MSE), num(r.PSNR), num(r.SSIM),
		num(r.Precision), num(r.Recall), num(r.F1), num(r.IoU), num(r.TimeMS),
	}
}

// WriteMetricsCSV writes rows to path with the MetricsColumns header.
func WriteMetricsCSV(path string, rows []Row) error {
	return writeCSV(path, MetricsColumns, func(w *csv.Writer) error {
		for _, r := range rows {
			if err := w.Write(r.record()); err != nil {
				return err
			}
		}
		return nil
	})
}

// StageTiming is the mean time of one stage per frame.
type StageTiming struct {
	Key      string  `json:"key"`
	Method   string  `json:"method"`
	Category string  `json:"category"`
	MeanMS   float64 `json:"mean_ms"`
}

// Summary is the result of one run: one pipeline on one sequence with one
// noise setting. Mask metrics are computed from the confusion counts summed
// over all evaluated frames (the CDnet convention), not averaged per frame.
type Summary struct {
	RunID        string     `json:"run_id"`
	Pipeline     string     `json:"pipeline"`
	PipelineName string     `json:"pipeline_name"`
	Owner        string     `json:"owner,omitempty"`
	Sequence     string     `json:"sequence"`
	Category     string     `json:"category"`
	Noise        noise.Spec `json:"noise"`
	Start        int        `json:"start"`
	Frames       int        `json:"frames"`
	EvalFrames   int        `json:"eval_frames"`
	Width        int        `json:"width"`
	Height       int        `json:"height"`
	// Mask metrics (NaN → null when no frame had ground truth).
	Precision JSONFloat `json:"precision"`
	Recall    JSONFloat `json:"recall"`
	F1        JSONFloat `json:"f1"`
	IoU       JSONFloat `json:"iou"`
	// Denoising quality of the first denoising stage against the clean frame.
	PSNR JSONFloat `json:"psnr"`
	SSIM JSONFloat `json:"ssim"`
	// Speed.
	FPS     float64       `json:"fps"`
	TotalMS float64       `json:"total_ms"`
	Stages  []StageTiming `json:"stages"`
	// Extra holds run-level results of stages (e.g. direction histograms).
	Extra     map[string]any `json:"extra,omitempty"`
	HasPlan   bool           `json:"has_plan"`
	CreatedAt time.Time      `json:"created_at"`
}

// SummaryColumns is the header of summary.csv.
var SummaryColumns = []string{
	"run_id", "pipeline", "owner", "sequence", "category", "noise_type", "noise_level",
	"frames", "eval_frames", "precision", "recall", "f1", "iou", "psnr", "ssim", "fps",
}

// WriteSummaryCSV writes one row per run.
func WriteSummaryCSV(path string, sums []Summary) error {
	return writeCSV(path, SummaryColumns, func(w *csv.Writer) error {
		for _, s := range sums {
			rec := []string{
				s.RunID, s.Pipeline, s.Owner, s.Sequence, s.Category, s.Noise.Type, num(s.Noise.Level),
				strconv.Itoa(s.Frames), strconv.Itoa(s.EvalFrames),
				num(float64(s.Precision)), num(float64(s.Recall)), num(float64(s.F1)), num(float64(s.IoU)),
				num(float64(s.PSNR)), num(float64(s.SSIM)), num(s.FPS),
			}
			if err := w.Write(rec); err != nil {
				return err
			}
		}
		return nil
	})
}

// FrameResult holds the per-frame numbers shown in the web UI.
type FrameResult struct {
	Frame     int       `json:"frame"`
	HasGT     bool      `json:"has_gt"`
	Precision JSONFloat `json:"precision"`
	Recall    JSONFloat `json:"recall"`
	F1        JSONFloat `json:"f1"`
	IoU       JSONFloat `json:"iou"`
	PSNR      JSONFloat `json:"psnr"`
	TimeMS    float64   `json:"time_ms"`
	// Moving is the fraction of pixels marked as motion.
	Moving float64 `json:"moving"`
}

// Run is the content of run.json: everything needed to show or redo a run.
type Run struct {
	Summary Summary `json:"summary"`
	// Pipeline is the pipeline config that was run.
	Pipeline any `json:"pipeline"`
	// StageKeys are the stage keys in order, also the names of saved stage images.
	StageKeys []string      `json:"stage_keys"`
	Frames    []FrameResult `json:"frames"`
	// SavedFrames tells whether per-frame images exist under frames/.
	SavedFrames bool `json:"saved_frames"`
}

// WriteRunJSON writes run.json.
func WriteRunJSON(path string, r *Run) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("report: %w", err)
	}
	return writeFile(path, data)
}

// ReadRunJSON reads run.json.
func ReadRunJSON(path string) (*Run, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("report: %w", err)
	}
	var r Run
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("report: %s: %w", path, err)
	}
	return &r, nil
}

// JSONFloat is a float64 that encodes NaN and infinities as null, which
// plain encoding/json refuses.
type JSONFloat float64

// MarshalJSON implements json.Marshaler.
func (f JSONFloat) MarshalJSON() ([]byte, error) {
	v := float64(f)
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return []byte("null"), nil
	}
	return strconv.AppendFloat(nil, v, 'g', -1, 64), nil
}

// UnmarshalJSON implements json.Unmarshaler; null becomes NaN.
func (f *JSONFloat) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*f = JSONFloat(math.NaN())
		return nil
	}
	v, err := strconv.ParseFloat(string(b), 64)
	if err != nil {
		return err
	}
	*f = JSONFloat(v)
	return nil
}

func writeCSV(path string, header []string, body func(*csv.Writer) error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("report: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("report: %w", err)
	}
	w := csv.NewWriter(f)
	err = w.Write(header)
	if err == nil {
		err = body(w)
	}
	w.Flush()
	if err == nil {
		err = w.Error()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("report: %s: %w", path, err)
	}
	return nil
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("report: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("report: %w", err)
	}
	return nil
}
