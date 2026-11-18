package imgx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInBounds(t *testing.T) {
	g := NewGray(3, 2)
	for _, c := range []struct {
		x, y int
		want bool
	}{
		{0, 0, true}, {2, 1, true}, {3, 0, false}, {0, 2, false}, {-1, 0, false}, {0, -1, false},
	} {
		if got := g.InBounds(c.x, c.y); got != c.want {
			t.Errorf("InBounds(%d,%d) = %v, want %v", c.x, c.y, got, c.want)
		}
	}
}

func TestGrayClampInPlace(t *testing.T) {
	g, err := GrayFromSlice(4, 1, []float32{-10, 0, 128.5, 300})
	if err != nil {
		t.Fatal(err)
	}
	if out := g.Clamp(); out != g {
		t.Error("Clamp must return the same image")
	}
	want := []float32{0, 0, 128.5, 255}
	for i, w := range want {
		if g.Pix[i] != w {
			t.Errorf("pixel %d = %v, want %v", i, g.Pix[i], w)
		}
	}
}

func TestNewImagePanicsOnInvalidSize(t *testing.T) {
	for i, f := range []func(){
		func() { NewGray(0, 1) },
		func() { NewGray(1, -1) },
		func() { NewRGB(-2, 2) },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("case %d: want panic for a non-positive size", i)
				}
			}()
			f()
		}()
	}
}

func TestRGBCloneIsIndependent(t *testing.T) {
	c := NewRGB(2, 1)
	c.Set(0, 0, 10, 20, 30)
	d := c.Clone()
	d.Set(0, 0, 1, 2, 3)
	if r, g, b := c.At(0, 0); r != 10 || g != 20 || b != 30 {
		t.Errorf("original changed to (%v,%v,%v)", r, g, b)
	}
	if d.W != 2 || d.H != 1 {
		t.Errorf("clone size %dx%d", d.W, d.H)
	}
}

func TestGrayToRGB(t *testing.T) {
	g, _ := GrayFromSlice(2, 1, []float32{7, 200})
	c := GrayToRGB(g)
	if !c.IsGrayscale() {
		t.Error("GrayToRGB must give equal channels")
	}
	if r, _, _ := c.At(1, 0); r != 200 {
		t.Errorf("pixel (1,0) = %v, want 200", r)
	}
	g.Pix[0] = 99 // the result must not share memory with the input
	if r, _, _ := c.At(0, 0); r != 7 {
		t.Error("GrayToRGB must copy the pixels")
	}
}

func TestLoadGrayConvertsColor(t *testing.T) {
	// A coloured image is converted with the BT.601 weights.
	c := NewRGB(1, 1)
	c.Set(0, 0, 200, 100, 50)
	path := filepath.Join(t.TempDir(), "c.png")
	if err := SavePNG(path, c.ToImage()); err != nil {
		t.Fatal(err)
	}
	g, err := LoadGray(path)
	if err != nil {
		t.Fatal(err)
	}
	want := float32(0.299*200 + 0.587*100 + 0.114*50)
	if d := g.Pix[0] - want; d > 1e-3 || d < -1e-3 {
		t.Errorf("luma %v, want %v", g.Pix[0], want)
	}
	if _, err := LoadGray(filepath.Join(t.TempDir(), "missing.png")); err == nil {
		t.Error("missing file: want error")
	}
}

func TestSavePNGCreatesDirsAndReportsErrors(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "a", "b", "out.png")
	if err := SavePNG(nested, NewGray(2, 2).ToImage()); err != nil {
		t.Fatalf("nested path: %v", err)
	}
	if _, err := os.Stat(nested); err != nil {
		t.Fatal(err)
	}

	// A regular file where a directory is needed makes MkdirAll fail.
	blocker := filepath.Join(dir, "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SavePNG(filepath.Join(blocker, "out.png"), NewGray(1, 1).ToImage()); err == nil {
		t.Error("parent is a file: want error")
	}
	// The target itself is a directory: Create fails.
	if err := SavePNG(dir, NewGray(1, 1).ToImage()); err == nil {
		t.Error("target is a directory: want error")
	}
}
