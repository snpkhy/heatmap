package imgx

import (
	"errors"
	"math"
	"path/filepath"
	"testing"
)

const testdata = "../../testdata/images"

func TestGrayAtClamped(t *testing.T) {
	// 2x2 image:
	//   1 2
	//   3 4
	g, err := GrayFromSlice(2, 2, []float32{1, 2, 3, 4})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		x, y int
		want float32
	}{
		{0, 0, 1},
		{1, 1, 4},
		{-1, 0, 1}, // left of (0,0) → border pixel 1
		{5, 0, 2},  // right of (1,0) → 2
		{0, -3, 1}, // above (0,0) → 1
		{-1, 9, 3}, // bottom-left corner → 3
		{9, 9, 4},  // bottom-right corner → 4
	}
	for _, tt := range tests {
		if got := g.AtClamped(tt.x, tt.y); got != tt.want {
			t.Errorf("AtClamped(%d,%d) = %v, want %v", tt.x, tt.y, got, tt.want)
		}
	}
}

func TestGrayFromSliceRejectsWrongLength(t *testing.T) {
	if _, err := GrayFromSlice(2, 2, []float32{1, 2, 3}); err == nil {
		t.Fatal("expected an error for 3 pixels in a 2x2 image")
	}
}

func TestCloneIsIndependent(t *testing.T) {
	g := NewGray(1, 1)
	c := g.Clone()
	c.Set(0, 0, 7)
	if g.At(0, 0) != 0 {
		t.Fatal("modifying the clone changed the original")
	}
}

func TestClampAndToByte(t *testing.T) {
	tests := []struct {
		in   float32
		want uint8
	}{
		{-5, 0}, {0, 0}, {0.4, 0}, {0.5, 1}, {254.6, 255}, {300, 255},
	}
	for _, tt := range tests {
		if got := ToByte(tt.in); got != tt.want {
			t.Errorf("ToByte(%v) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestLuma(t *testing.T) {
	// Pure red (255,0,0): Y = 0.299·255 = 76.245
	// Pure white:         Y = (0.299+0.587+0.114)·255 = 255
	c := NewRGB(2, 1)
	c.Set(0, 0, 255, 0, 0)
	c.Set(1, 0, 255, 255, 255)
	y := c.Luma()
	want := []float32{76.245, 255}
	for i, w := range want {
		if math.Abs(float64(y.Pix[i]-w)) > 1e-3 {
			t.Errorf("Luma pixel %d = %v, want %v", i, y.Pix[i], w)
		}
	}
}

func TestCheckSameSize(t *testing.T) {
	err := CheckSameSize(NewGray(2, 2), NewGray(3, 2))
	if !errors.Is(err, ErrSizeMismatch) {
		t.Fatalf("got %v, want ErrSizeMismatch", err)
	}
	if err := CheckSameSize(NewGray(2, 2), NewGray(2, 2)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadValidImages(t *testing.T) {
	t.Run("grayscale PNG keeps exact values", func(t *testing.T) {
		// gray_3x3.png holds 0, 10, 20, ..., 80 row by row.
		g, err := LoadGray(filepath.Join(testdata, "gray_3x3.png"))
		if err != nil {
			t.Fatal(err)
		}
		if g.W != 3 || g.H != 3 {
			t.Fatalf("size %dx%d, want 3x3", g.W, g.H)
		}
		for i, v := range g.Pix {
			if v != float32(i*10) {
				t.Errorf("pixel %d = %v, want %d", i, v, i*10)
			}
		}
	})

	t.Run("1x1 RGB PNG", func(t *testing.T) {
		c, err := Load(filepath.Join(testdata, "rgb_1x1.png"))
		if err != nil {
			t.Fatal(err)
		}
		if r, g, b := c.At(0, 0); r != 10 || g != 20 || b != 30 {
			t.Errorf("pixel = (%v,%v,%v), want (10,20,30)", r, g, b)
		}
		if c.IsGrayscale() {
			t.Error("a colored pixel was reported as grayscale")
		}
	})

	t.Run("BMP", func(t *testing.T) {
		c, err := Load(filepath.Join(testdata, "rgb_2x1.bmp"))
		if err != nil {
			t.Fatal(err)
		}
		if r, _, b := c.At(0, 0); r != 255 || b != 0 {
			t.Errorf("first pixel should be red, got r=%v b=%v", r, b)
		}
		if r, _, b := c.At(1, 0); r != 0 || b != 255 {
			t.Errorf("second pixel should be blue, got r=%v b=%v", r, b)
		}
	})

	t.Run("JPEG", func(t *testing.T) {
		// Flat gray 128 saved with quality 95; JPEG is lossy, so allow ±2.
		g, err := LoadGray(filepath.Join(testdata, "flat_8x8.jpg"))
		if err != nil {
			t.Fatal(err)
		}
		for i, v := range g.Pix {
			if v < 126 || v > 130 {
				t.Fatalf("pixel %d = %v, want about 128", i, v)
			}
		}
	})
}

func TestLoadBrokenFiles(t *testing.T) {
	tests := []struct {
		file string
		want error
	}{
		{"empty.png", ErrEmptyFile},
		{"corrupt.png", ErrCorrupt},
		{"not_an_image.png", ErrUnsupportedFormat},
		{"unsupported.gif", ErrUnsupportedFormat},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			_, err := Load(filepath.Join(testdata, tt.file))
			if !errors.Is(err, tt.want) {
				t.Fatalf("got %v, want %v", err, tt.want)
			}
		})
	}

	t.Run("missing file", func(t *testing.T) {
		if _, err := Load(filepath.Join(testdata, "does_not_exist.png")); err == nil {
			t.Fatal("expected an error")
		}
	})
}

func TestSavePNGRoundTrip(t *testing.T) {
	// Values are rounded and clamped on save: -3 → 0, 12.6 → 13, 300 → 255.
	g, _ := GrayFromSlice(3, 1, []float32{-3, 12.6, 300})
	path := filepath.Join(t.TempDir(), "sub", "out.png")
	if err := SavePNG(path, g.ToImage()); err != nil {
		t.Fatal(err)
	}
	back, err := LoadGray(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []float32{0, 13, 255}
	for i, w := range want {
		if back.Pix[i] != w {
			t.Errorf("pixel %d = %v, want %v", i, back.Pix[i], w)
		}
	}
}

func TestRGBRoundTrip(t *testing.T) {
	c := NewRGB(2, 1)
	c.Set(0, 0, 1, 2, 3)
	c.Set(1, 0, 250, 128, 0)
	path := filepath.Join(t.TempDir(), "rgb.png")
	if err := SavePNG(path, c.ToImage()); err != nil {
		t.Fatal(err)
	}
	back, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	for i := range c.R {
		if back.R[i] != c.R[i] || back.G[i] != c.G[i] || back.B[i] != c.B[i] {
			t.Errorf("pixel %d differs after round trip", i)
		}
	}
}
