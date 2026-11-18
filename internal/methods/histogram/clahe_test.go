package histogram

import (
	"testing"

	"heatmap/internal/imgx"
)

func TestCLAHEDarkSceneContrastStretched(t *testing.T) {
	// A dark scene with low contrast (intensities between 10 and 30)
	w, h := 64, 64
	src := imgx.NewGray(w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			src.Set(x, y, float32(10+(x%20)))
		}
	}

	cfg := DefaultCLAHEConfig()
	cfg.GridX, cfg.GridY = 4, 4
	out, err := CLAHE(src, cfg)
	if err != nil {
		t.Fatal(err)
	}

	var minVal, maxVal float32 = 255, 0
	for _, v := range out.Pix {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}

	// Dynamic range must be significantly widened compared to original [10, 29]
	originalSpan := float32(19)
	newSpan := maxVal - minVal
	if newSpan < originalSpan*2 {
		t.Errorf("expected CLAHE to stretch contrast significantly: original span %v, got %v", originalSpan, newSpan)
	}
}

func TestCLAHERejectsBadConfig(t *testing.T) {
	src := imgx.NewGray(8, 8)
	badConfigs := []CLAHEConfig{
		{GridX: 0, GridY: 8, ClipLimit: 2.0},
		{GridX: 8, GridY: 0, ClipLimit: 2.0},
		{GridX: 8, GridY: 8, ClipLimit: 0.5},
	}
	for _, cfg := range badConfigs {
		if _, err := CLAHE(src, cfg); err == nil {
			t.Errorf("expected error for bad config %+v", cfg)
		}
	}
}

func BenchmarkCLAHE(b *testing.B) {
	src := imgx.NewGray(320, 240)
	for i := range src.Pix {
		src.Pix[i] = float32((i * 7) % 60)
	}
	cfg := DefaultCLAHEConfig()
	b.ResetTimer()
	for b.Loop() {
		if _, err := CLAHE(src, cfg); err != nil {
			b.Fatal(err)
		}
	}
}
