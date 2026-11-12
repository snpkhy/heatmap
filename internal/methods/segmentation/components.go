package segmentation

import (
	"fmt"

	"heatmap/internal/imgx"
)

// Connectivity says which neighbours of a pixel belong to the same component.
type Connectivity int

const (
	// Conn4 joins pixels that share an edge (left, right, up, down).
	Conn4 Connectivity = 4
	// Conn8 also joins pixels that touch only by a corner.
	Conn8 Connectivity = 8
)

// maskThresh is the level from which a mask pixel counts as foreground,
// the same as in the metrics package.
const maskThresh = 128

// Component describes one connected group of foreground pixels.
type Component struct {
	Label                  int     // 1-based, in raster order of the first pixel
	Area                   int     // number of pixels
	MinX, MinY, MaxX, MaxY int     // bounding box, inclusive
	CX, CY                 float64 // centroid
}

// Label finds the connected components of a binary mask (foreground ≥ 128)
// with the classic two-pass algorithm:
//
//  1. Scan the image row by row. A foreground pixel takes the smallest label
//     among its already visited neighbours (left and the row above), or a new
//     label if it has none. When neighbours carry different labels, the labels
//     are recorded as equivalent in a union–find structure.
//  2. Scan again, replacing every provisional label with the representative
//     of its equivalence class, renumbered 1, 2, 3, … in raster order.
//
// It returns the label of every pixel (0 for background) and the components
// in label order.
func Label(mask *imgx.Gray, conn Connectivity) ([]int32, []Component, error) {
	if conn != Conn4 && conn != Conn8 {
		return nil, nil, fmt.Errorf("segmentation: connectivity %d must be 4 or 8", conn)
	}
	labels := make([]int32, len(mask.Pix))
	parent := firstPass(mask, conn, labels)

	// Second pass: resolve equivalences and renumber consecutively.
	final := make([]int32, len(parent))
	var comps []Component
	for i, l := range labels {
		if l == 0 {
			continue
		}
		root := find(parent, l)
		if final[root] == 0 {
			x, y := i%mask.W, i/mask.W
			comps = append(comps, Component{Label: len(comps) + 1, MinX: x, MinY: y, MaxX: x, MaxY: y})
			final[root] = int32(len(comps))
		}
		labels[i] = final[root]
		comps[final[root]-1].add(i%mask.W, i/mask.W)
	}
	for i := range comps {
		comps[i].CX /= float64(comps[i].Area)
		comps[i].CY /= float64(comps[i].Area)
	}
	return labels, comps, nil
}

// firstPass assigns provisional labels and returns the union–find parents
// (index 0 is unused, so that label 0 can mean background).
func firstPass(mask *imgx.Gray, conn Connectivity, labels []int32) []int32 {
	parent := []int32{0}
	w := mask.W
	for y := range mask.H {
		for x := range w {
			i := y*w + x
			if mask.Pix[i] < maskThresh {
				continue
			}
			var cur int32
			join := func(n int32) {
				switch {
				case n == 0:
				case cur == 0:
					cur = n
				default:
					cur = union(parent, cur, n)
				}
			}
			if x > 0 {
				join(labels[i-1])
			}
			if y > 0 {
				join(labels[i-w])
				if conn == Conn8 {
					if x > 0 {
						join(labels[i-w-1])
					}
					if x < w-1 {
						join(labels[i-w+1])
					}
				}
			}
			if cur == 0 {
				cur = int32(len(parent))
				parent = append(parent, cur)
			}
			labels[i] = cur
		}
	}
	return parent
}

func (c *Component) add(x, y int) {
	c.Area++
	c.MinX, c.MaxX = min(c.MinX, x), max(c.MaxX, x)
	c.MinY, c.MaxY = min(c.MinY, y), max(c.MaxY, y)
	c.CX += float64(x)
	c.CY += float64(y)
}

// find returns the representative of label l, halving the path on the way.
func find(parent []int32, l int32) int32 {
	for parent[l] != l {
		parent[l] = parent[parent[l]]
		l = parent[l]
	}
	return l
}

// union merges the classes of a and b and returns the smaller representative,
// so representatives keep following raster order.
func union(parent []int32, a, b int32) int32 {
	ra, rb := find(parent, a), find(parent, b)
	if ra > rb {
		ra, rb = rb, ra
	}
	parent[rb] = ra
	return ra
}

// FilterByArea keeps only the connected components whose area in pixels lies
// in [minArea, maxArea] and returns the cleaned mask (0 or 255) together with
// the kept components. maxArea ≤ 0 means "no upper limit".
//
// Small components are typically noise (snow, rain, sensor grain); very large
// ones usually mean a global change such as a camera jump or a light switch.
func FilterByArea(mask *imgx.Gray, conn Connectivity, minArea, maxArea int) (*imgx.Gray, []Component, error) {
	labels, comps, err := Label(mask, conn)
	if err != nil {
		return nil, nil, err
	}
	keep := make([]bool, len(comps)+1)
	var kept []Component
	for _, c := range comps {
		if c.Area >= minArea && (maxArea <= 0 || c.Area <= maxArea) {
			keep[c.Label] = true
			kept = append(kept, c)
		}
	}
	out := imgx.NewGray(mask.W, mask.H)
	for i, l := range labels {
		if keep[l] {
			out.Pix[i] = MaskForeground
		}
	}
	return out, kept, nil
}
