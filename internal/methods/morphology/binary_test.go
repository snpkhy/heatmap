package morphology

import (
	"strings"
	"testing"

	"heatmap/internal/imgx"
)

// parseMask builds a mask from rows of '#' (foreground) and '.' (background).
func parseMask(t *testing.T, rows ...string) *imgx.Gray {
	t.Helper()
	g := imgx.NewGray(len(rows[0]), len(rows))
	for y, row := range rows {
		for x, c := range row {
			if c == '#' {
				g.Set(x, y, maskForeground)
			}
		}
	}
	return g
}

// formatMask is the inverse of parseMask, for readable failure messages.
func formatMask(g *imgx.Gray) string {
	var b strings.Builder
	for y := 0; y < g.H; y++ {
		b.WriteByte('\n')
		for x := 0; x < g.W; x++ {
			if g.At(x, y) >= maskThresh {
				b.WriteByte('#')
			} else {
				b.WriteByte('.')
			}
		}
	}
	return b.String()
}

func assertMask(t *testing.T, got, want *imgx.Gray) {
	t.Helper()
	if formatMask(got) != formatMask(want) {
		t.Errorf("got:%s\nwant:%s", formatMask(got), formatMask(want))
	}
}

func TestDilate(t *testing.T) {
	src := parseMask(t,
		".....",
		".....",
		"..#..",
		".....",
		".....",
	)
	got, err := Dilate(src, 3)
	if err != nil {
		t.Fatal(err)
	}
	assertMask(t, got, parseMask(t,
		".....",
		".###.",
		".###.",
		".###.",
		".....",
	))
}

func TestErode(t *testing.T) {
	src := parseMask(t,
		".....",
		".###.",
		".###.",
		".###.",
		".....",
	)
	got, err := Erode(src, 3)
	if err != nil {
		t.Fatal(err)
	}
	assertMask(t, got, parseMask(t,
		".....",
		".....",
		"..#..",
		".....",
		".....",
	))
}

func TestErodeKeepsObjectsAtBorder(t *testing.T) {
	// A full mask stays full: pixels outside the image do not count as background.
	src := parseMask(t, "###", "###")
	got, err := Erode(src, 3)
	if err != nil {
		t.Fatal(err)
	}
	assertMask(t, got, src)
}

func TestClosingFillsHole(t *testing.T) {
	// Two pixels of margin so the dilated object does not reach the border.
	src := parseMask(t,
		".........",
		".........",
		"..#####..",
		"..##.##..",
		"..#####..",
		".........",
		".........",
	)
	got, err := Closing(src, 3)
	if err != nil {
		t.Fatal(err)
	}
	assertMask(t, got, parseMask(t,
		".........",
		".........",
		"..#####..",
		"..#####..",
		"..#####..",
		".........",
		".........",
	))
}

func TestClosingJoinsFragments(t *testing.T) {
	// Two edge fragments one pixel apart become one object.
	src := parseMask(t,
		".........",
		".........",
		"..##.##..",
		".........",
		".........",
	)
	got, err := Closing(src, 3)
	if err != nil {
		t.Fatal(err)
	}
	assertMask(t, got, parseMask(t,
		".........",
		".........",
		"..#####..",
		".........",
		".........",
	))
}

func TestMorphologyRejectsBadWindow(t *testing.T) {
	src := imgx.NewGray(2, 2)
	for _, k := range []int{0, -1, 2} {
		if _, err := Dilate(src, k); err == nil {
			t.Errorf("Dilate k=%d: expected an error", k)
		}
		if _, err := Erode(src, k); err == nil {
			t.Errorf("Erode k=%d: expected an error", k)
		}
		if _, err := Closing(src, k); err == nil {
			t.Errorf("Closing k=%d: expected an error", k)
		}
	}
}

func BenchmarkClosing3x3(b *testing.B) {
	src := imgx.NewGray(320, 240)
	for i := range src.Pix {
		if i%7 == 0 {
			src.Pix[i] = maskForeground
		}
	}
	b.ResetTimer()
	for b.Loop() {
		if _, err := Closing(src, 3); err != nil {
			b.Fatal(err)
		}
	}
}
