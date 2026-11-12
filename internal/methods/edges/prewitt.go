package edges

import (
	"math"

	"heatmap/internal/imgx"
)

// PrewittXY returns the horizontal and vertical Prewitt derivatives:
//
//	      | −1 0 1 |              | −1 −1 −1 |
//	Gx = ⅓| −1 0 1 | ∗ I,   Gy = ⅓|  0  0  0 | ∗ I
//	      | −1 0 1 |              |  1  1  1 |
//
// The ⅓ factor averages the three rows (columns), so an ideal black-to-white
// step gives exactly 255. Borders replicate the nearest pixel.
func PrewittXY(g *imgx.Gray) (gx, gy *imgx.Gray) {
	gx = imgx.NewGray(g.W, g.H)
	gy = imgx.NewGray(g.W, g.H)
	for y := range g.H {
		up := max(y-1, 0) * g.W
		mid := y * g.W
		down := min(y+1, g.H-1) * g.W
		for x := range g.W {
			l := max(x-1, 0)
			r := min(x+1, g.W-1)

			a, b, c := g.Pix[up+l], g.Pix[up+x], g.Pix[up+r]
			d, f := g.Pix[mid+l], g.Pix[mid+r]
			p, q, s := g.Pix[down+l], g.Pix[down+x], g.Pix[down+r]

			gx.Pix[mid+x] = ((c + f + s) - (a + d + p)) / 3
			gy.Pix[mid+x] = ((p + q + s) - (a + b + c)) / 3
		}
	}
	return gx, gy
}

// Prewitt returns the Prewitt gradient magnitude √(Gx² + Gy²) (see PrewittXY).
// Values are not clamped: a diagonal step can reach 255·√2.
//
// The gradient ignores a constant added to the whole image, so it reacts to
// object contours rather than to slow changes of overall brightness.
func Prewitt(g *imgx.Gray) *imgx.Gray {
	gx, gy := PrewittXY(g)
	out := imgx.NewGray(g.W, g.H)
	for i := range out.Pix {
		out.Pix[i] = float32(math.Hypot(float64(gx.Pix[i]), float64(gy.Pix[i])))
	}
	return out
}
