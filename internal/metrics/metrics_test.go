package metrics

import (
	"errors"
	"math"
	"testing"
	"time"

	"heatmap/internal/imgx"
)

const eps = 1e-6

func gray(t *testing.T, w, h int, pix ...float32) *imgx.Gray {
	t.Helper()
	g, err := imgx.GrayFromSlice(w, h, pix)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func near(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func TestMSE(t *testing.T) {
	// differences: 0, −2, 3, 0 → MSE = (0 + 4 + 9 + 0) / 4 = 3.25
	a := gray(t, 2, 2, 0, 10, 20, 30)
	b := gray(t, 2, 2, 0, 12, 17, 30)
	got, err := MSE(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if !near(got, 3.25, eps) {
		t.Errorf("MSE = %v, want 3.25", got)
	}

	// Only pixels 0 and 2 count: (0 + 9) / 2 = 4.5
	got, _ = MSEMasked(a, b, []bool{true, false, true, false})
	if !near(got, 4.5, eps) {
		t.Errorf("MSEMasked = %v, want 4.5", got)
	}

	got, _ = MSEMasked(a, b, []bool{false, false, false, false})
	if !math.IsNaN(got) {
		t.Errorf("MSEMasked with no valid pixels = %v, want NaN", got)
	}
}

func TestMSEErrors(t *testing.T) {
	_, err := MSE(imgx.NewGray(2, 2), imgx.NewGray(1, 2))
	if !errors.Is(err, imgx.ErrSizeMismatch) {
		t.Errorf("got %v, want ErrSizeMismatch", err)
	}
	_, err = MSEMasked(imgx.NewGray(2, 2), imgx.NewGray(2, 2), []bool{true})
	if err == nil {
		t.Error("expected an error for a wrong-length valid mask")
	}
}

func TestPSNR(t *testing.T) {
	tests := []struct {
		mse, want float64
	}{
		{1, 48.1308036},    // 10·log10(65025 / 1)
		{3.25, 43.0119700}, // 10·log10(65025 / 3.25) = 10·log10(20007.69)
		{65025, 0},         // MSE = MAX² → 0 dB
	}
	for _, tt := range tests {
		if got := PSNR(tt.mse); !near(got, tt.want, 1e-6) {
			t.Errorf("PSNR(%v) = %v, want %v", tt.mse, got, tt.want)
		}
	}
	if got := PSNR(0); !math.IsInf(got, 1) {
		t.Errorf("PSNR(0) = %v, want +Inf", got)
	}
}

func TestGaussianKernel(t *testing.T) {
	k := gaussianKernel(11, 1.5)
	var sum float64
	for _, v := range k {
		sum += v
	}
	if !near(sum, 1, 1e-12) {
		t.Errorf("kernel sums to %v, want 1", sum)
	}
	// Centre (5,5) is the largest weight; kernel is symmetric.
	if k[5*11+5] <= k[5*11+4] || !near(k[5*11+4], k[5*11+6], 1e-15) || !near(k[0], k[120], 1e-15) {
		t.Error("kernel is not centred or not symmetric")
	}
}

func TestSSIM(t *testing.T) {
	t.Run("identical images give 1", func(t *testing.T) {
		a := gray(t, 3, 3, 0, 10, 20, 30, 40, 50, 60, 70, 80)
		got, err := SSIM(a, a.Clone())
		if err != nil {
			t.Fatal(err)
		}
		if !near(got, 1, eps) {
			t.Errorf("SSIM = %v, want 1", got)
		}
	})

	t.Run("flat 0 vs flat 10", func(t *testing.T) {
		// Both flat → σx = σy = σxy = 0, so the structure term is C2/C2 = 1:
		// SSIM = C1 / (0² + 10² + C1) = 6.5025 / 106.5025 = 0.0610549
		a := imgx.NewGray(5, 5)
		b := imgx.NewGray(5, 5)
		for i := range b.Pix {
			b.Pix[i] = 10
		}
		got, _ := SSIM(a, b)
		if !near(got, 0.0610549, 1e-6) {
			t.Errorf("SSIM = %v, want 0.0610549", got)
		}
	})

	t.Run("1x1 image smaller than the window", func(t *testing.T) {
		// Same formula as above with a single pixel.
		got, _ := SSIM(gray(t, 1, 1, 0), gray(t, 1, 1, 10))
		if !near(got, 0.0610549, 1e-6) {
			t.Errorf("SSIM = %v, want 0.0610549", got)
		}
	})

	t.Run("2x1 inverted pixels", func(t *testing.T) {
		// x = [0, 20], y = [20, 0]. Each window sees both pixels with raw weights
		// 1 (centre) and e^(−1/4.5) = 0.800737 (neighbour), renormalised to
		// p = 0.555328 and q = 0.444672. For pixel 0:
		//   μx = 20q = 8.893439, μy = 20p = 11.106561
		//   σx² = σy² = 400pq = 98.775523, σxy = −μxμy = −98.775523
		//   SSIM = (2·98.7755 + C1)(−2·98.7755 + C2) / ((79.0933 + 123.3557 + C1)(2·98.7755 + C2))
		//        = −0.530198
		// Pixel 1 is the mirror image and gives the same value.
		got, _ := SSIM(gray(t, 2, 1, 0, 20), gray(t, 2, 1, 20, 0))
		if !near(got, -0.5301979, 1e-6) {
			t.Errorf("SSIM = %v, want -0.5301979", got)
		}
	})

	t.Run("symmetric", func(t *testing.T) {
		a := gray(t, 3, 2, 5, 80, 30, 200, 10, 90)
		b := gray(t, 3, 2, 0, 60, 35, 180, 40, 70)
		ab, _ := SSIM(a, b)
		ba, _ := SSIM(b, a)
		if !near(ab, ba, 1e-12) {
			t.Errorf("SSIM(a,b) = %v, SSIM(b,a) = %v", ab, ba)
		}
	})

	t.Run("masked pixels are ignored", func(t *testing.T) {
		// Pixel 1 differs, but it is excluded from both the centres and the windows,
		// so only identical pixels remain → SSIM = 1.
		a := gray(t, 3, 1, 10, 0, 30)
		b := gray(t, 3, 1, 10, 255, 30)
		got, _ := SSIMMasked(a, b, []bool{true, false, true})
		if !near(got, 1, eps) {
			t.Errorf("SSIMMasked = %v, want 1", got)
		}
	})
}

func TestCompareMask(t *testing.T) {
	// pixel: 0      1     2    3       4         5
	// gt:    motion motion bg  shadow  outside   unknown
	// pred:  fg     bg     fg  bg      fg        bg
	// →      TP     FN     FP  TN      ignored   ignored
	gt := gray(t, 6, 1, 255, 255, 0, 50, 85, 170)
	pred := gray(t, 6, 1, 255, 0, 255, 0, 255, 0)
	c, err := CompareMask(pred, gt)
	if err != nil {
		t.Fatal(err)
	}
	if c != (Confusion{TP: 1, FP: 1, FN: 1, TN: 1}) {
		t.Fatalf("confusion = %+v", c)
	}
	// P = 1/2, R = 1/2, F1 = 2/(2+1+1) = 0.5, IoU = 1/(1+1+1) = 1/3
	checks := []struct {
		name      string
		got, want float64
	}{
		{"precision", c.Precision(), 0.5},
		{"recall", c.Recall(), 0.5},
		{"f1", c.F1(), 0.5},
		{"iou", c.IoU(), 1.0 / 3},
	}
	for _, ch := range checks {
		if !near(ch.got, ch.want, eps) {
			t.Errorf("%s = %v, want %v", ch.name, ch.got, ch.want)
		}
	}
}

func TestConfusionEdgeCases(t *testing.T) {
	empty := Confusion{TN: 10}
	for name, v := range map[string]float64{
		"precision": empty.Precision(), "recall": empty.Recall(), "f1": empty.F1(), "iou": empty.IoU(),
	} {
		if !math.IsNaN(v) {
			t.Errorf("%s of an empty prediction and empty GT = %v, want NaN", name, v)
		}
	}

	sum := Confusion{TP: 1, FP: 2}.Add(Confusion{TP: 3, FN: 4, TN: 5})
	if sum != (Confusion{TP: 4, FP: 2, FN: 4, TN: 5}) {
		t.Errorf("Add = %+v", sum)
	}
}

func TestGTHelpers(t *testing.T) {
	gt := gray(t, 5, 1, 0, 50, 85, 170, 255)
	valid := GTValid(gt)
	wantValid := []bool{true, true, false, false, true}
	bin := GTBinary(gt)
	wantBin := []float32{0, 0, 0, 0, 255}
	for i := range valid {
		if valid[i] != wantValid[i] {
			t.Errorf("GTValid[%d] = %v, want %v", i, valid[i], wantValid[i])
		}
		if bin.Pix[i] != wantBin[i] {
			t.Errorf("GTBinary[%d] = %v, want %v", i, bin.Pix[i], wantBin[i])
		}
	}
}

func TestTiming(t *testing.T) {
	if got := FPS(50, 2*time.Second); !near(got, 25, eps) {
		t.Errorf("FPS = %v, want 25", got)
	}
	if got := FPS(10, 0); !math.IsNaN(got) {
		t.Errorf("FPS with zero duration = %v, want NaN", got)
	}
	if got := Milliseconds(1500 * time.Microsecond); !near(got, 1.5, eps) {
		t.Errorf("Milliseconds = %v, want 1.5", got)
	}
}
