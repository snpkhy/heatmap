package segmentation

import (
	"testing"

	"heatmap/internal/imgx"
)

func TestRunningAvgBGMotionDetection(t *testing.T) {
	w, h := 64, 48
	bg := imgx.NewGray(w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			bg.Set(x, y, float32(50+(x+y)%30))
		}
	}

	model, err := NewRunningAvgBG(DefaultRunningAvgBGConfig())
	if err != nil {
		t.Fatal(err)
	}

	// First frame: initialises model, returns zero mask
	m1, err := model.Apply(bg)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range m1.Pix {
		if v != MaskBackground {
			t.Fatal("first frame mask must be all background")
		}
	}

	// Second frame: introduce a moving object with significant contrast
	f2 := bg.Clone()
	for y := 10; y < 25; y++ {
		for x := 20; x < 35; x++ {
			f2.Set(x, y, 220)
		}
	}

	m2, err := model.Apply(f2)
	if err != nil {
		t.Fatal(err)
	}

	// Inside the object: must be foreground
	for y := 12; y < 23; y++ {
		for x := 22; x < 33; x++ {
			if m2.At(x, y) != MaskForeground {
				t.Errorf("expected foreground at (%d, %d), got %v", x, y, m2.At(x, y))
			}
		}
	}

	// Well outside the object: must be background
	if m2.At(5, 5) != MaskBackground || m2.At(50, 40) != MaskBackground {
		t.Errorf("background pixel misclassified as foreground")
	}
}

func TestRunningAvgBGRejectsBadConfig(t *testing.T) {
	badConfigs := []RunningAvgBGConfig{
		{Alpha: 0, FGAlpha: 0, MinThreshold: 10},
		{Alpha: 1.5, FGAlpha: 0, MinThreshold: 10},
		{Alpha: 0.1, FGAlpha: 0.2, MinThreshold: 10},
		{Alpha: 0.1, FGAlpha: 0.05, MinThreshold: -1},
	}
	for _, cfg := range badConfigs {
		if _, err := NewRunningAvgBG(cfg); err == nil {
			t.Errorf("expected error for bad config %+v", cfg)
		}
	}
}
