package segmentation

import (
	"strings"
	"testing"

	"heatmap/internal/imgx"
)

// maskFrom builds a mask from rows of text: '#' is foreground, anything else background.
func maskFrom(t *testing.T, rows ...string) *imgx.Gray {
	t.Helper()
	m := imgx.NewGray(len(rows[0]), len(rows))
	for y, row := range rows {
		if len(row) != m.W {
			t.Fatalf("row %d has %d cells, want %d", y, len(row), m.W)
		}
		for x, ch := range row {
			if ch == '#' {
				m.Set(x, y, 255)
			}
		}
	}
	return m
}

// render is the inverse of maskFrom, for readable failure messages.
func render(m *imgx.Gray) string {
	var sb strings.Builder
	for y := range m.H {
		for x := range m.W {
			if m.At(x, y) >= 128 {
				sb.WriteByte('#')
			} else {
				sb.WriteByte('.')
			}
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

func TestLabelConnectivity(t *testing.T) {
	// Two blobs that touch only by a corner: one component with 8-connectivity, two with 4.
	m := maskFrom(t,
		"##...",
		"##...",
		"..##.",
		"..##.",
	)
	_, c8, err := Label(m, Conn8)
	if err != nil {
		t.Fatal(err)
	}
	if len(c8) != 1 || c8[0].Area != 8 {
		t.Errorf("Conn8: %+v, want one component of area 8", c8)
	}
	_, c4, _ := Label(m, Conn4)
	if len(c4) != 2 || c4[0].Area != 4 || c4[1].Area != 4 {
		t.Errorf("Conn4: %+v, want two components of area 4", c4)
	}
}

func TestLabelMergesEquivalentLabels(t *testing.T) {
	// A "U" shape: the two arms get different provisional labels in the first
	// pass and are joined only on the bottom row.
	m := maskFrom(t,
		"#...#",
		"#...#",
		"#####",
	)
	labels, comps, err := Label(m, Conn4)
	if err != nil {
		t.Fatal(err)
	}
	if len(comps) != 1 {
		t.Fatalf("got %d components, want 1", len(comps))
	}
	for i, v := range m.Pix {
		want := int32(0)
		if v >= 128 {
			want = 1
		}
		if labels[i] != want {
			t.Fatalf("label[%d] = %d, want %d", i, labels[i], want)
		}
	}
	c := comps[0]
	if c.Area != 9 || c.MinX != 0 || c.MinY != 0 || c.MaxX != 4 || c.MaxY != 2 {
		t.Errorf("component %+v, want area 9 and box (0,0)-(4,2)", c)
	}
	// Centroid: x is symmetric around 2; y = (2·0 + 2·1 + 5·2) / 9 = 12/9.
	if c.CX != 2 || c.CY < 1.333 || c.CY > 1.334 {
		t.Errorf("centroid (%v, %v), want (2, 1.333)", c.CX, c.CY)
	}
}

func TestLabelRasterOrderAndStats(t *testing.T) {
	m := maskFrom(t,
		"..#..",
		".....",
		"##..#",
		"##..#",
	)
	labels, comps, _ := Label(m, Conn8)
	if len(comps) != 3 {
		t.Fatalf("got %d components, want 3", len(comps))
	}
	wantAreas := []int{1, 4, 2}
	for i, c := range comps {
		if c.Label != i+1 || c.Area != wantAreas[i] {
			t.Errorf("component %d: %+v, want label %d area %d", i, c, i+1, wantAreas[i])
		}
	}
	if labels[2] != 1 || labels[2*5] != 2 || labels[2*5+4] != 3 {
		t.Errorf("labels not in raster order: %v", labels)
	}
}

func TestLabelEmptyAndFull(t *testing.T) {
	_, comps, _ := Label(imgx.NewGray(4, 3), Conn8)
	if len(comps) != 0 {
		t.Errorf("empty mask has %d components", len(comps))
	}
	full := maskFrom(t, "###", "###")
	_, comps, _ = Label(full, Conn4)
	if len(comps) != 1 || comps[0].Area != 6 {
		t.Errorf("full mask: %+v, want one component of area 6", comps)
	}
}

func TestLabelBadConnectivity(t *testing.T) {
	if _, _, err := Label(imgx.NewGray(2, 2), 6); err == nil {
		t.Error("connectivity 6 was accepted")
	}
}

func TestFilterByArea(t *testing.T) {
	m := maskFrom(t,
		"#....###",
		".....###",
		"..#.....",
		"........",
		"####....",
	)
	// Areas: 1 (top-left), 6 (block), 1 (lone pixel), 4 (bar).
	tests := []struct {
		name     string
		min, max int
		want     []string
	}{
		{"drop specks", 2, 0, []string{
			".....###",
			".....###",
			"........",
			"........",
			"####....",
		}},
		{"drop specks and the largest", 2, 5, []string{
			"........",
			"........",
			"........",
			"........",
			"####....",
		}},
		{"keep everything", 1, 0, []string{
			"#....###",
			".....###",
			"..#.....",
			"........",
			"####....",
		}},
		{"keep nothing", 100, 0, []string{
			"........",
			"........",
			"........",
			"........",
			"........",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, kept, err := FilterByArea(m, Conn4, tt.min, tt.max)
			if err != nil {
				t.Fatal(err)
			}
			want := render(maskFrom(t, tt.want...))
			if got := render(out); got != want {
				t.Errorf("got\n%swant\n%s", got, want)
			}
			area := 0
			for _, c := range kept {
				area += c.Area
			}
			if area != strings.Count(want, "#") {
				t.Errorf("kept components cover %d pixels, mask has %d", area, strings.Count(want, "#"))
			}
		})
	}
}
