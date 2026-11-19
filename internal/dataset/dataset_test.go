package dataset

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"heatmap/internal/imgx"
)

func TestSyntheticScenes(t *testing.T) {
	names := SyntheticNames()
	if len(names) != 4 {
		t.Fatalf("scenes %v, want 4", names)
	}
	for _, name := range names {
		s, err := NewSynthetic(name)
		if err != nil {
			t.Fatal(err)
		}
		info := s.Info()
		if info.ID != "synthetic/"+name || !info.HasGT || info.EvalStart != syntheticWarmup || info.EvalEnd != info.Frames-1 {
			t.Errorf("%s: info %+v", name, info)
		}
		if gt, err := s.GroundTruth(0); gt != nil || err != nil {
			t.Errorf("%s: warm-up frame must have no ground truth", name)
		}
		// Somewhere in the middle every scene has moving pixels.
		gt, err := s.GroundTruth(info.Frames / 2)
		if err != nil || gt == nil {
			t.Fatalf("%s: %v", name, err)
		}
		var motion int
		for _, v := range gt.Pix {
			if v == gtMotion {
				motion++
			}
		}
		if motion < 100 {
			t.Errorf("%s: only %d moving pixels in the middle frame", name, motion)
		}
		f, err := s.Frame(info.Frames / 2)
		if err != nil || f.W != info.Width || f.H != info.Height {
			t.Fatalf("%s: frame %v %v", name, f, err)
		}
		if _, err := s.Frame(info.Frames); err == nil {
			t.Errorf("%s: frame past the end must fail", name)
		}
		if _, err := s.GroundTruth(-1); err == nil {
			t.Errorf("%s: negative frame must fail", name)
		}
	}
	if _, err := NewSynthetic("ocean"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown scene: err = %v", err)
	}
}

func TestSyntheticIsDeterministicAndMoves(t *testing.T) {
	a, _ := NewSynthetic("street")
	b, _ := NewSynthetic("street")
	f1, _ := a.Frame(50)
	f2, _ := b.Frame(50)
	f3, _ := a.Frame(51)
	same, moved := true, false
	for i := range f1.R {
		same = same && f1.R[i] == f2.R[i] && f1.B[i] == f2.B[i]
		moved = moved || f1.R[i] != f3.R[i]
	}
	if !same {
		t.Error("the same frame must render identically")
	}
	if !moved {
		t.Error("consecutive frames must differ")
	}
}

func TestSyntheticShadowLabels(t *testing.T) {
	s, _ := NewSynthetic("shadow")
	gt, _ := s.GroundTruth(60)
	f, _ := s.Frame(60)
	bg, _ := s.Frame(5) // object not yet in the scene
	var shadow int
	for i, v := range gt.Pix {
		if v == gtShadow {
			shadow++
			if f.R[i] > bg.R[i]*0.75 {
				t.Fatalf("shadow pixel %d is not darker: %v vs %v", i, f.R[i], bg.R[i])
			}
		}
	}
	if shadow < 100 {
		t.Errorf("%d shadow pixels, want a visible shadow", shadow)
	}
}

// writeCDnet creates a tiny CDnet-like sequence of n frames with temporal ROI [roi0, roi1].
func writeCDnet(t *testing.T, dir string, n, roi0, roi1 int) {
	t.Helper()
	for i := 1; i <= n; i++ {
		f := imgx.NewRGB(6, 4)
		f.Set(0, 0, float32(10*i), 0, 0)
		if err := imgx.SavePNG(filepath.Join(dir, "input", fmt.Sprintf("in%06d.png", i)), f.ToImage()); err != nil {
			t.Fatal(err)
		}
		gt := imgx.NewGray(6, 4)
		gt.Set(1, 1, 255)
		if err := imgx.SavePNG(filepath.Join(dir, "groundtruth", fmt.Sprintf("gt%06d.png", i)), gt.ToImage()); err != nil {
			t.Fatal(err)
		}
	}
	roi := fmt.Sprintf("%d %d\n", roi0, roi1)
	if err := os.WriteFile(filepath.Join(dir, "temporalROI.txt"), []byte(roi), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCDnet(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "cdnet", "baseline", "tiny")
	writeCDnet(t, dir, 5, 3, 5)

	s, err := Open(root, "cdnet/baseline/tiny")
	if err != nil {
		t.Fatal(err)
	}
	info := s.Info()
	if info.Frames != 5 || info.Category != "baseline" || info.Name != "tiny" ||
		!info.HasGT || info.EvalStart != 2 || info.EvalEnd != 4 || info.Width != 6 || info.Height != 4 {
		t.Errorf("info %+v", info)
	}
	f, err := s.Frame(3)
	if err != nil {
		t.Fatal(err)
	}
	if r, _, _ := f.At(0, 0); r != 40 {
		t.Errorf("frame 3 marker %v, want 40 (fourth file)", r)
	}
	if gt, err := s.GroundTruth(1); gt != nil || err != nil {
		t.Error("frame before the temporal ROI must have no ground truth")
	}
	gt, err := s.GroundTruth(2)
	if err != nil || gt == nil || gt.At(1, 1) != 255 {
		t.Errorf("ground truth of frame 2: %v %v", gt, err)
	}
	if _, err := s.Frame(5); err == nil {
		t.Error("frame past the end must fail")
	}

	// The same directory opened by path: only OpenInput accepts paths.
	byPath, err := OpenInput(root, dir)
	if err != nil || byPath.Info().Source != SourceCDnet {
		t.Errorf("open by path: %v %v", byPath, err)
	}
	if _, err := Open(root, dir); !errors.Is(err, ErrNotFound) {
		t.Errorf("Open must reject filesystem paths, err = %v", err)
	}
	if s, err := OpenInput(root, "synthetic/street"); err != nil || s.Info().Source != SourceSynthetic {
		t.Errorf("OpenInput with an id: %v", err)
	}
}

func TestCDnetWithoutTemporalROI(t *testing.T) {
	dir := t.TempDir()
	writeCDnet(t, dir, 2, 1, 2)
	if err := os.Remove(filepath.Join(dir, "temporalROI.txt")); err != nil {
		t.Fatal(err)
	}
	s, err := OpenCDnet(dir, "x")
	if err != nil {
		t.Fatal(err)
	}
	if s.Info().HasGT {
		t.Error("without temporalROI.txt there is no usable ground truth")
	}
	if gt, err := s.GroundTruth(0); gt != nil || err != nil {
		t.Error("GroundTruth must be nil")
	}
}

func TestCDnetTemporalROIPastTheEnd(t *testing.T) {
	dir := t.TempDir()
	writeCDnet(t, dir, 3, 7, 9)
	s, err := OpenCDnet(dir, "x")
	if err != nil {
		t.Fatal(err)
	}
	if info := s.Info(); info.HasGT || info.EvalStart != -1 || info.EvalEnd != -1 {
		t.Errorf("ROI beyond the frames must mean no ground truth: %+v", info)
	}
}

func TestFolder(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "frames", "clip")
	for i, name := range []string{"b.png", "a.png", "c.bmp.txt"} {
		if filepath.Ext(name) == ".txt" {
			if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		g := imgx.NewGray(3, 2)
		g.Pix[0] = float32(100 + i)
		if err := imgx.SavePNG(filepath.Join(dir, name), g.ToImage()); err != nil {
			t.Fatal(err)
		}
	}
	s, err := Open(root, "frames/clip")
	if err != nil {
		t.Fatal(err)
	}
	info := s.Info()
	if info.Frames != 2 || info.HasGT || info.Source != SourceFrames || info.EvalStart != -1 {
		t.Errorf("info %+v", info)
	}
	f, _ := s.Frame(0)
	if r, _, _ := f.At(0, 0); r != 101 {
		t.Errorf("frames must be sorted by name: first is %v, want 101 (a.png)", r)
	}
	if gt, err := s.GroundTruth(1); gt != nil || err != nil {
		t.Error("a folder has no ground truth")
	}
	if _, err := s.GroundTruth(2); err == nil {
		t.Error("out of range must fail")
	}
}

func TestOpenErrors(t *testing.T) {
	root := t.TempDir()
	for _, id := range []string{
		"", "synthetic", "synthetic/a/b", "cdnet/baseline", "frames/a/b",
		"cdnet/baseline/missing", "frames/missing", filepath.Join(root, "nope"),
	} {
		if _, err := Open(root, id); err == nil {
			t.Errorf("Open(%q): want error", id)
		}
	}
	if _, err := Open(root, "frames/../../etc"); err == nil {
		t.Error(".. must be rejected")
	}
	empty := filepath.Join(root, "empty")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenPath(empty); !errors.Is(err, ErrNotFound) {
		t.Errorf("empty folder: err = %v", err)
	}
}

func TestDiscover(t *testing.T) {
	root := t.TempDir()
	writeCDnet(t, filepath.Join(root, "cdnet", "shadow", "tiny"), 2, 1, 2)
	if err := os.MkdirAll(filepath.Join(root, "cdnet", "shadow", "junk"), 0o755); err != nil {
		t.Fatal(err)
	}
	clip := filepath.Join(root, "frames", "clip")
	if err := imgx.SavePNG(filepath.Join(clip, "0001.png"), imgx.NewGray(2, 2).ToImage()); err != nil {
		t.Fatal(err)
	}
	infos, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, in := range infos {
		ids = append(ids, in.ID)
	}
	want := []string{"synthetic/night", "synthetic/plaza", "synthetic/shadow", "synthetic/street", "cdnet/shadow/tiny", "frames/clip"}
	if fmt.Sprint(ids) != fmt.Sprint(want) {
		t.Errorf("ids %v, want %v", ids, want)
	}

	onlySynthetic, err := Discover(filepath.Join(root, "missing"))
	if err != nil || len(onlySynthetic) != 4 {
		t.Errorf("missing data dir: %d sequences, %v", len(onlySynthetic), err)
	}
}

func TestPlazaWalkers(t *testing.T) {
	s, err := NewSynthetic("plaza")
	if err != nil {
		t.Fatal(err)
	}
	gt, _ := s.GroundTruth(200)
	var motion, shadow int
	for _, v := range gt.Pix {
		switch v {
		case gtMotion:
			motion++
		case gtShadow:
			shadow++
		}
	}
	// Several people are in the square at frame 200, each with a shadow.
	if motion < 1500 || shadow < 150 {
		t.Errorf("frame 200: %d moving and %d shadow pixels", motion, shadow)
	}

	near, ok := walker(100, 220, 1, 0, 0, 10, shirtRed, 0).figureAt(0, s.sc.h)
	far, ok2 := walker(100, 60, 1, 0, 0, 10, shirtRed, 0).figureAt(0, s.sc.h)
	if !ok || !ok2 || far.h >= near.h {
		t.Errorf("a distant figure must be smaller: near %.1f, far %.1f", near.h, far.h)
	}
	if _, ok := walker(0, 0, 1, 0, 5, 10, shirtRed, 0).figureAt(11, s.sc.h); ok {
		t.Error("a walker must be gone after its last frame")
	}

	// From top to bottom a figure is a head, a body and legs.
	x := int(near.fx)
	at := func(up float64) int { return near.part(x, int(near.fy-up*near.h)) }
	if at(0.93) != partHead || at(0.65) != partBody {
		t.Errorf("head %d, body %d", at(0.93), at(0.65))
	}
	legs := false
	x0, _, w, _ := near.bounds()
	for px := x0; px < x0+w; px++ {
		legs = legs || near.part(px, int(near.fy-0.2*near.h)) == partLegs
	}
	if !legs || near.part(x0-3, int(near.fy-0.5*near.h)) != partNone {
		t.Error("legs must be under the body and nothing outside the figure")
	}
}
