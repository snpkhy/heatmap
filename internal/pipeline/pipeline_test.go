package pipeline

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"heatmap/internal/config"
	"heatmap/internal/dataset"
	"heatmap/internal/imgx"
	"heatmap/internal/noise"
	"heatmap/internal/report"
)

func TestParams(t *testing.T) {
	p := NewParams(map[string]any{
		"i": 5, "whole": 7.0, "f": 1.5, "fi": 2, "b": true, "s": "x",
		"pts": []any{[]any{0, 1.5}, []any{2.0, 3}},
	})
	if p.Int("i", 0, "") != 5 || p.Int("whole", 0, "") != 7 || p.Int("missing", 9, "") != 9 {
		t.Error("Int")
	}
	if p.Float("f", 0, "") != 1.5 || p.Float("fi", 0, "") != 2 || p.Float32("missing", 0.5, "") != 0.5 {
		t.Error("Float")
	}
	if !p.Bool("b", false, "") || p.String("s", "", "") != "x" {
		t.Error("Bool/String")
	}
	if pts := p.Points("pts", nil, ""); len(pts) != 2 || pts[0] != [2]float64{0, 1.5} || pts[1] != [2]float64{2, 3} {
		t.Errorf("Points %v", pts)
	}
	if err := p.Err(); err != nil {
		t.Errorf("unexpected error %v", err)
	}
	if len(p.Specs()) != 9 || p.Specs()[0].Name != "i" || p.Specs()[0].Type != "int" {
		t.Errorf("specs %+v", p.Specs())
	}

	bad := NewParams(map[string]any{
		"i": 1.5, "f": "x", "nan": math.NaN(), "b": 1, "s": 2, "pts": "x", "pts2": []any{[]any{1}},
		"pts3": []any{[]any{"a", 1}}, "typo": 1,
	})
	bad.Int("i", 0, "")
	bad.Float("f", 0, "")
	bad.Float("nan", 0, "")
	bad.Bool("b", false, "")
	bad.String("s", "", "")
	bad.Points("pts", nil, "")
	bad.Points("pts2", nil, "")
	bad.Points("pts3", nil, "")
	bad.Errorf("custom %d", 1)
	err := bad.Err()
	if err == nil {
		t.Fatal("want errors")
	}
	for _, want := range []string{`"i"`, `"f"`, `"nan"`, `"b"`, `"s"`, `"pts"`, `"pts2"`, `"pts3"`, "custom 1", "unknown parameter(s) typo"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
}

func TestRegistry(t *testing.T) {
	ms := Methods()
	if len(ms) < 25 {
		t.Fatalf("only %d methods registered", len(ms))
	}
	for i := 1; i < len(ms); i++ {
		a, b := ms[i-1], ms[i]
		if a.Category > b.Category || (a.Category == b.Category && a.Name >= b.Name) {
			t.Fatalf("methods not sorted: %s/%s before %s/%s", a.Category, a.Name, b.Category, b.Name)
		}
	}
	if _, ok := Lookup("gaussian"); !ok {
		t.Error("gaussian must be registered")
	}
	if _, ok := Lookup("nope"); ok {
		t.Error("unknown method found")
	}
	m, _ := Lookup("gaussian")
	if ps := m.Params(); len(ps) != 2 || ps[0].Name != "k" || ps[0].Default != 5 {
		t.Errorf("gaussian params %+v", ps)
	}
	defer func() {
		if recover() == nil {
			t.Error("duplicate registration must panic")
		}
	}()
	Register(Method{Name: "gaussian"})
}

// TestEveryMethodBuildsAndRuns builds every method with default parameters
// and runs it for a few frames after stages that provide what it needs.
func TestEveryMethodBuildsAndRuns(t *testing.T) {
	seq, _ := dataset.NewSynthetic("street")
	for _, m := range Methods() {
		t.Run(m.Name, func(t *testing.T) {
			st, err := m.New(NewParams(nil))
			if err != nil {
				t.Fatalf("defaults: %v", err)
			}
			harris, _ := Lookup("harris")
			hs, _ := harris.New(NewParams(nil))
			color, _ := Lookup("chroma_value")
			cs, _ := color.New(NewParams(nil))
			for i := 40; i < 44; i++ {
				rgb, _ := seq.Frame(i)
				f := NewFrame(i, rgb)
				if _, err := cs.Process(f); err != nil { // Channels for channel-based stages
					t.Fatal(err)
				}
				f.Image = rgb.Luma()
				if _, err := hs.Process(f); err != nil { // Corners for block matching
					t.Fatal(err)
				}
				f.Mask = imgx.NewGray(rgb.W, rgb.H) // a mask for mask stages
				f.Mask.Set(50, 50, 255)
				view, err := st.Process(f)
				if err != nil {
					t.Fatalf("frame %d: %v", i, err)
				}
				if view == nil || view.W == 0 {
					t.Fatalf("frame %d: no view", i)
				}
			}
			st.Reset()
			if m.Description == "" || m.Category == "" {
				t.Error("method needs a description and a category")
			}
		})
	}
}

func TestBadParamsFailAtBuild(t *testing.T) {
	cases := map[string]map[string]any{
		"gaussian":                  {"k": 4},
		"median":                    {"k": 0},
		"box":                       {"k": -1},
		"clahe":                     {"clip_limit": 0.5},
		"running_avg_otsu":          {"alpha": 2},
		"frame_diff":                {"threshold": 0},
		"frame_diff_region_growing": {"connectivity": 5},
		"gaussian_bg":               {"k": -1},
		"block_threshold":           {"block": 0},
		"components":                {"connectivity": 3},
		"block_matching":            {"block": 4},
		"opening":                   {"k": 2},
		"sobel_diff":                {"alpha": 0},
		"block_lbp":                 {"block": 1},
		"block_change":              {"persist": 0},
		"direction_histogram":       {"bins": 1},
		"fft_filter":                {"cutoff": 0.9},
		"perspective_warp":          {"src": []any{[]any{0, 0}, []any{1, 1}, []any{2, 2}, []any{3, 3}}},
		"harris":                    {"k": 0},
		"chroma_value":              {"min_sum": -1},
		"equalize":                  {"channel": -3},
	}
	for name, params := range cases {
		cfg := &config.Pipeline{ID: "x", Stages: []config.Stage{
			{Method: name, Params: params}, {Method: "otsu"}, {Method: "closing"},
		}}
		if _, err := Build(cfg); err == nil {
			t.Errorf("%s %v: want build error", name, params)
		}
	}
	for _, extra := range []map[string]any{
		{"interp": "cubic"}, {"width": 0}, {"src": []any{[]any{0, 0}}},
	} {
		cfg := &config.Pipeline{ID: "x", Stages: []config.Stage{
			{Method: "gaussian"}, {Method: "otsu"}, {Method: "perspective_warp", Params: extra},
		}}
		if _, err := Build(cfg); err == nil {
			t.Errorf("perspective_warp %v: want build error", extra)
		}
	}
	if _, err := Build(&config.Pipeline{ID: "x", Stages: []config.Stage{
		{Method: "fft_filter", Params: map[string]any{"shape": "box"}}, {Method: "otsu"}, {Method: "closing"},
	}}); err == nil {
		t.Error("bad fft shape: want error")
	}
}

func TestBuildRuleAndErrors(t *testing.T) {
	two := &config.Pipeline{ID: "x", Stages: []config.Stage{{Method: "gaussian"}, {Method: "otsu"}}}
	if _, err := Build(two); !errors.Is(err, ErrRule) {
		t.Errorf("2 stages: err = %v", err)
	}
	sameCat := &config.Pipeline{ID: "x", Stages: []config.Stage{
		{Method: "gaussian"}, {Method: "median"}, {Method: "otsu"},
	}}
	if _, err := Build(sameCat); !errors.Is(err, ErrRule) {
		t.Errorf("2 categories: err = %v", err)
	}
	unknown := &config.Pipeline{ID: "x", Stages: []config.Stage{{Method: "warp9"}, {Method: "otsu"}, {Method: "closing"}}}
	if _, err := Build(unknown); err == nil || !strings.Contains(err.Error(), "unknown method") {
		t.Errorf("unknown method: err = %v", err)
	}
	typo := &config.Pipeline{ID: "x", Stages: []config.Stage{
		{Method: "gaussian", Params: map[string]any{"sigam": 1}}, {Method: "otsu"}, {Method: "closing"},
	}}
	if _, err := Build(typo); err == nil || !strings.Contains(err.Error(), "sigam") {
		t.Errorf("typo: err = %v", err)
	}
	if _, err := Build(&config.Pipeline{}); err == nil {
		t.Error("empty config: want error")
	}
}

func TestProcessNeedsMask(t *testing.T) {
	p, err := Build(&config.Pipeline{ID: "nomask", Stages: []config.Stage{
		{Method: "gaussian"}, {Method: "rgb_to_ycbcr"}, {Method: "equalize"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	f := NewFrame(0, imgx.NewRGB(8, 8))
	if _, err := p.Process(f); !errors.Is(err, ErrNoMask) {
		t.Errorf("err = %v, want ErrNoMask", err)
	}
	// A mask stage without an earlier mask producer fails clearly.
	q, _ := Build(&config.Pipeline{ID: "x", Stages: []config.Stage{
		{Method: "gaussian"}, {Method: "closing"}, {Method: "rgb_to_ycbcr"},
	}})
	if _, err := q.Process(NewFrame(0, imgx.NewRGB(8, 8))); err == nil || !strings.Contains(err.Error(), "mask") {
		t.Errorf("err = %v", err)
	}
	// equalize on a channel that does not exist.
	r, _ := Build(&config.Pipeline{ID: "x", Stages: []config.Stage{
		{Method: "gaussian"}, {Method: "equalize", Params: map[string]any{"channel": 2}}, {Method: "otsu"},
	}})
	if _, err := r.Process(NewFrame(0, imgx.NewRGB(8, 8))); err == nil {
		t.Error("missing channel: want error")
	}
}

func loadP(t *testing.T, id string) *config.Pipeline {
	t.Helper()
	p, err := config.FindPipeline(pipelinesDir, id)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRunWritesOutputs(t *testing.T) {
	seq, _ := dataset.NewSynthetic("street")
	out := t.TempDir()
	var progress []Progress
	res, err := Run(context.Background(), Options{
		Pipeline:   loadP(t, "p5"),
		Sequence:   seq,
		Noise:      noise.Spec{Type: noise.Gaussian, Level: 8, Seed: 1},
		Start:      -1,
		Frames:     30,
		OutDir:     out,
		RunID:      "test-run",
		SaveFrames: true,
		Progress:   func(p Progress) { progress = append(progress, p) },
	})
	if err != nil {
		t.Fatal(err)
	}
	s := res.Run.Summary
	if s.Frames != 30 || s.Start != 0 || s.EvalFrames != 20 || s.RunID != "test-run" || s.Noise.Level != 8 {
		t.Errorf("summary %+v", s)
	}
	if math.IsNaN(float64(s.F1)) || math.IsNaN(float64(s.PSNR)) || !s.HasPlan || s.FPS <= 0 || len(s.Stages) != 3 {
		t.Errorf("summary metrics %+v", s)
	}
	if len(progress) != 30 || progress[29].Done != 30 || progress[29].Total != 30 {
		t.Errorf("progress %d calls, last %+v", len(progress), progress[len(progress)-1])
	}
	// 3 stage rows + 1 final row per frame.
	if len(res.Rows) != 30*4 {
		t.Errorf("%d rows", len(res.Rows))
	}
	for _, name := range []string{"run.json", "metrics.csv", "summary.csv", "heatmap.png", "overlay.png",
		"plan_heatmap.png", "background.png", "frames/15/input.png", "frames/15/mask.png", "frames/15/gt.png",
		"frames/15/1-fft_filter.png", "frames/15/3-perspective_warp.png"} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Errorf("missing %s", name)
		}
	}
	if _, err := os.Stat(filepath.Join(out, "frames/5/gt.png")); err == nil {
		t.Error("warm-up frames have no ground truth image")
	}
	run, err := report.ReadRunJSON(filepath.Join(out, "run.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !run.SavedFrames || len(run.Frames) != 30 || len(run.StageKeys) != 3 || run.Frames[0].HasGT {
		t.Errorf("run.json %+v", run)
	}
	csv, _ := os.ReadFile(filepath.Join(out, "metrics.csv"))
	if !strings.HasPrefix(string(csv), strings.Join(report.MetricsColumns, ",")) {
		t.Error("metrics.csv header")
	}
}

func TestRunDirectionHistogramSummary(t *testing.T) {
	seq, _ := dataset.NewSynthetic("street")
	res, err := Run(context.Background(), Options{Pipeline: loadP(t, "p6"), Sequence: seq, Start: 15, Frames: 60})
	if err != nil {
		t.Fatal(err)
	}
	h, ok := res.Run.Summary.Extra["3-direction_histogram"].(map[string]any)
	if !ok {
		t.Fatalf("extra %v", res.Run.Summary.Extra)
	}
	if h["vectors"].(int) == 0 {
		t.Error("no vectors counted")
	}
	// The car drives right (0°) and is the strongest mover in the scene.
	if h["dominant_deg"] != 0.0 {
		t.Errorf("dominant direction %v, want 0", h["dominant_deg"])
	}
}

func TestRunErrorsAndCancel(t *testing.T) {
	seq, _ := dataset.NewSynthetic("night")
	p1 := loadP(t, "p1")
	if _, err := Run(context.Background(), Options{Pipeline: p1}); err == nil {
		t.Error("no sequence: want error")
	}
	if _, err := Run(context.Background(), Options{Pipeline: p1, Sequence: seq, Start: 500}); err == nil {
		t.Error("start past the end: want error")
	}
	if _, err := Run(context.Background(), Options{Pipeline: p1, Sequence: seq, Noise: noise.Spec{Type: "fog"}}); err == nil {
		t.Error("bad noise: want error")
	}
	if _, err := Run(context.Background(), Options{Pipeline: &config.Pipeline{ID: "x"}, Sequence: seq}); err == nil {
		t.Error("bad pipeline: want error")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Run(ctx, Options{Pipeline: p1, Sequence: seq}); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled: err = %v", err)
	}
	res, err := Run(context.Background(), Options{Pipeline: p1, Sequence: seq, Start: 0, Frames: 5})
	if err != nil {
		t.Fatal(err)
	}
	if res.Run.Summary.EvalFrames != 0 || !math.IsNaN(float64(res.Run.Summary.F1)) {
		t.Error("frames before ground truth must not be evaluated")
	}
}

func TestRunIDAndAutoStart(t *testing.T) {
	id := NewRunID("p1", "my seq/../x", time.Date(2026, 11, 19, 14, 30, 5, 0, time.UTC))
	if !strings.HasPrefix(id, "20261119-143005-p1-my_seq_x-") || !ValidRunID(id) {
		t.Errorf("id %q", id)
	}
	for _, bad := range []string{"", "a/b", "..", "a b", strings.Repeat("a", 200)} {
		if ValidRunID(bad) {
			t.Errorf("ValidRunID(%q) = true", bad)
		}
	}
	if AutoStart(dataset.Info{HasGT: true, EvalStart: 470}) != 420 ||
		AutoStart(dataset.Info{HasGT: true, EvalStart: 10}) != 0 || AutoStart(dataset.Info{}) != 0 {
		t.Error("AutoStart")
	}
}

func TestDisplayable(t *testing.T) {
	in, _ := imgx.GrayFromSlice(3, 1, []float32{0, 100, 255})
	if Displayable(in) != in {
		t.Error("in-range image must be returned as is")
	}
	wide, _ := imgx.GrayFromSlice(3, 1, []float32{-50, 0, 450})
	out := Displayable(wide)
	if out.Pix[0] != 0 || out.Pix[2] != 255 {
		t.Errorf("stretched %v", out.Pix)
	}
	flat, _ := imgx.GrayFromSlice(2, 1, []float32{-5, -5})
	if f := Displayable(flat); f.Pix[0] != 0 {
		t.Error("flat out-of-range image must map to 0")
	}
}
