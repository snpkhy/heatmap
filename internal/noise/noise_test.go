package noise

import (
	"math"
	"testing"

	"heatmap/internal/imgx"
)

func flatRGB(w, h int, v float32) *imgx.RGB {
	c := imgx.NewRGB(w, h)
	for i := range c.R {
		c.R[i], c.G[i], c.B[i] = v, v, v
	}
	return c
}

func TestParseAndString(t *testing.T) {
	for in, want := range map[string]Spec{
		"":                 {Type: None},
		"none":             {Type: None},
		"gaussian:10":      {Type: Gaussian, Level: 10},
		"salt_pepper:0.05": {Type: SaltPepper, Level: 0.05},
	} {
		got, err := Parse(in)
		if err != nil || got != want {
			t.Errorf("Parse(%q) = %+v, %v; want %+v", in, got, err, want)
		}
		if back, _ := Parse(got.String()); back != got {
			t.Errorf("String round trip of %+v gave %+v", got, back)
		}
	}
	for _, bad := range []string{"gaussian", "gaussian:x", "gaussian:-1", "salt_pepper:2", "blur:3", "gaussian:NaN"} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("Parse(%q): want error", bad)
		}
	}
}

func TestNoneLeavesFrame(t *testing.T) {
	c := flatRGB(4, 4, 100)
	for _, s := range []Spec{{Type: None}, {}, {Type: Gaussian, Level: 0}} {
		if got := s.Apply(c, 3); got != c {
			t.Errorf("%+v must return the frame unchanged", s)
		}
	}
}

func TestGaussianStatisticsAndClamping(t *testing.T) {
	c := flatRGB(100, 100, 128)
	got := Spec{Type: Gaussian, Level: 10, Seed: 1}.Apply(c, 0)
	var sum, sum2 float64
	for _, v := range got.R {
		sum += float64(v)
		sum2 += float64(v) * float64(v)
	}
	n := float64(len(got.R))
	mean := sum / n
	std := math.Sqrt(sum2/n - mean*mean)
	if math.Abs(mean-128) > 0.5 || math.Abs(std-10) > 0.5 {
		t.Errorf("mean %.2f std %.2f, want 128 and 10", mean, std)
	}
	if c.R[0] != 128 {
		t.Error("the source frame must not change")
	}
	bright := Spec{Type: Gaussian, Level: 80, Seed: 1}.Apply(flatRGB(50, 50, 250), 0)
	for _, v := range bright.G {
		if v < 0 || v > 255 {
			t.Fatalf("value %v outside [0, 255]", v)
		}
	}
}

func TestSaltPepperAmount(t *testing.T) {
	c := flatRGB(100, 50, 128)
	got := Spec{Type: SaltPepper, Level: 0.1, Seed: 7}.Apply(c, 2)
	var salt, pepper, other int
	for i, v := range got.R {
		if got.G[i] != v || got.B[i] != v {
			t.Fatal("salt-and-pepper must hit all channels of a pixel")
		}
		switch v {
		case 255:
			salt++
		case 0:
			pepper++
		case 128:
			other++
		}
	}
	if salt+pepper != 500 {
		t.Errorf("%d corrupted pixels, want exactly 500", salt+pepper)
	}
	if salt < 200 || pepper < 200 {
		t.Errorf("salt %d, pepper %d: want roughly equal", salt, pepper)
	}
}

func TestReproducibleAndOrderIndependent(t *testing.T) {
	c := flatRGB(20, 20, 100)
	s := Spec{Type: Gaussian, Level: 5, Seed: 42}
	a5 := s.Apply(c, 5)
	_ = s.Apply(c, 1) // processing other frames in between changes nothing
	b5 := s.Apply(c, 5)
	a6 := s.Apply(c, 6)
	same, differ := true, false
	for i := range a5.R {
		same = same && a5.R[i] == b5.R[i]
		differ = differ || a5.R[i] != a6.R[i]
	}
	if !same {
		t.Error("the same frame and seed must give the same noise")
	}
	if !differ {
		t.Error("different frames must get different noise")
	}
	other := Spec{Type: Gaussian, Level: 5, Seed: 43}.Apply(c, 5)
	if other.R[0] == a5.R[0] && other.R[1] == a5.R[1] && other.R[2] == a5.R[2] {
		t.Error("a different seed must give different noise")
	}
}
