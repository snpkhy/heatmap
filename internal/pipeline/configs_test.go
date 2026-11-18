package pipeline

import (
	"context"
	"math"
	"testing"

	"heatmap/internal/config"
	"heatmap/internal/dataset"
	"heatmap/internal/noise"
)

const pipelinesDir = "../../configs/pipelines"

// TestShippedPipelinesOnSyntheticScenes builds every pipeline in
// configs/pipelines and runs it on every built-in scene. Each must build
// (rule: ≥3 stages, ≥3 categories), produce a mask on every frame and be
// evaluated against ground truth. Detection quality is checked by the
// reference test cases (configs/testcases.yaml, `heatmap testcases`).
func TestShippedPipelinesOnSyntheticScenes(t *testing.T) {
	all, err := config.LoadPipelines(pipelinesDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 8 {
		t.Fatalf("%d pipelines, want 8", len(all))
	}
	for _, cfg := range all {
		for _, scene := range dataset.SyntheticNames() {
			t.Run(cfg.ID+"/"+scene, func(t *testing.T) {
				t.Parallel()
				seq, err := dataset.NewSynthetic(scene)
				if err != nil {
					t.Fatal(err)
				}
				res, err := Run(context.Background(), Options{Pipeline: cfg, Sequence: seq, Start: -1, Frames: 40,
					Noise: noise.Spec{Type: noise.None}})
				if err != nil {
					t.Fatal(err)
				}
				s := res.Run.Summary
				t.Logf("F1 %.3f  P %.3f  R %.3f  IoU %.3f  %.0f fps",
					float64(s.F1), float64(s.Precision), float64(s.Recall), float64(s.IoU), s.FPS)
				if math.IsNaN(float64(s.F1)) || s.EvalFrames != 30 {
					t.Errorf("evaluated %d frames, F1 %v", s.EvalFrames, s.F1)
				}
			})
		}
	}
}
