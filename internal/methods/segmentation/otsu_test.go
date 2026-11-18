package segmentation

import (
	"testing"

	"heatmap/internal/imgx"
)

func TestOtsuBimodal(t *testing.T) {
	// A bimodal image with two equal peaks at 40 and 200.
	// Otsu must pick a threshold in the valley between them (around 120).
	w, h := 10, 10
	src := imgx.NewGray(w, h)
	for i := 0; i < len(src.Pix)/2; i++ {
		src.Pix[i] = 40
	}
	for i := len(src.Pix) / 2; i < len(src.Pix); i++ {
		src.Pix[i] = 200
	}

	thresh := Otsu(src)
	if thresh < 40 || thresh > 200 {
		t.Errorf("Otsu threshold %f should be between 40 and 200", thresh)
	}

	mask, gotT, err := OtsuThreshold(src)
	if err != nil {
		t.Fatal(err)
	}
	if gotT != thresh {
		t.Errorf("got threshold %f, want %f", gotT, thresh)
	}

	// First half should be background (0), second half foreground (255)
	for i := 0; i < len(mask.Pix)/2; i++ {
		if mask.Pix[i] != MaskBackground {
			t.Fatalf("pixel %d: got %f, want %d", i, mask.Pix[i], MaskBackground)
		}
	}
	for i := len(mask.Pix) / 2; i < len(mask.Pix); i++ {
		if mask.Pix[i] != MaskForeground {
			t.Fatalf("pixel %d: got %f, want %d", i, mask.Pix[i], MaskForeground)
		}
	}
}

func TestOtsuConstantImage(t *testing.T) {
	src := imgx.NewGray(5, 5)
	for i := range src.Pix {
		src.Pix[i] = 100
	}
	thresh := Otsu(src)
	if thresh < 0 || thresh > 255 {
		t.Errorf("unexpected threshold %f for constant image", thresh)
	}
}
