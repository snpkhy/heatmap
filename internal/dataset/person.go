package dataset

import (
	"math"
	"math/rand/v2"

	"heatmap/internal/imgx"
)

// plaza is the built-in scene with people: walkers cross a paved square in
// several directions, alone and in pairs, so the heatmap shows the paths
// they share. Figures further from the camera (higher in the frame) are
// smaller, and each casts a soft shadow that is labelled as shadow.
var plaza = scene{
	name:        "plaza",
	description: "Paved square seen from above a doorway, with eleven people walking across it in different directions",
	w:           320, h: 240, frames: 360,
	sensorNoise: 2.5,
	background:  plazaBackground,
	objects: []object{
		walker(-12, 182, 1.5, 0, 0, 226, shirtRed, 0),
		walker(-34, 192, 1.5, 0, 0, 242, shirtTeal, 1),
		walker(332, 150, -1.3, 0, 15, 282, shirtMustard, 2),
		walker(-10, 86, 1.0, 0, 30, 359, shirtWhite, 1),
		walker(46, 44, 1.1, 0.9, 50, 300, shirtBlue, 0),
		walker(300, 262, -1.2, -0.85, 90, 350, shirtGreen, 2),
		walker(204, 268, -0.15, -1.1, 120, 322, shirtPink, 1),
		walker(330, 96, -0.9, 0, 140, 359, shirtNavy, 0),
		walker(-14, 222, 1.9, 0, 170, 359, shirtOrange, 2),
		walker(334, 172, -1.6, 0, 220, 359, shirtWhite, 0),
		walker(254, 42, 0.1, 1.0, 200, 359, shirtRed, 1),
	},
}

// Clothes of the walkers. They are chosen to differ from the paving in
// brightness or in hue, like real clothes mostly do.
var (
	shirtRed     = [3]float32{196, 52, 48}
	shirtTeal    = [3]float32{28, 128, 132}
	shirtMustard = [3]float32{214, 160, 36}
	shirtWhite   = [3]float32{236, 234, 226}
	shirtBlue    = [3]float32{44, 86, 190}
	shirtGreen   = [3]float32{52, 132, 70}
	shirtPink    = [3]float32{214, 96, 140}
	shirtNavy    = [3]float32{30, 38, 78}
	shirtOrange  = [3]float32{226, 112, 36}
)

var (
	walkerPants = [][3]float32{{38, 42, 58}, {70, 62, 54}, {28, 30, 34}}
	walkerSkin  = [][3]float32{{226, 178, 142}, {172, 122, 88}, {110, 76, 56}}
)

// walkerHeight is the height in pixels of a figure standing at the bottom
// edge of the frame.
const walkerHeight = 44

// walker returns a person whose feet are at (x, y) at frame t0 and move by
// (vx, vy) pixels per frame until t1. look picks the pants and skin colours.
func walker(x, y, vx, vy float64, t0, t1 int, shirt [3]float32, look int) object {
	return object{person: true, h: walkerHeight, x0: x, y0: y, vx: vx, vy: vy, t0: t0, t1: t1,
		color: shirt, pants: walkerPants[look%len(walkerPants)], skin: walkerSkin[look%len(walkerSkin)],
		texture: 6, shadow: 0.66}
}

// plazaBackground draws square paving with darker joints, a wall with a
// doorway along the top edge and two planters near the corners.
func plazaBackground(x, y int, rng *rand.Rand) (r, g, b float32) {
	n := float32(rng.NormFloat64())
	if y < 34 { // wall
		v := 96 + n*3
		if y%8 == 0 || (x+(y/8)*14)%28 == 0 {
			v -= 16 // brick joints
		}
		if x >= 150 && x < 178 && y >= 4 {
			return 34 + n, 30 + n, 28 + n // doorway
		}
		if (x >= 40 && x < 100 || x >= 228 && x < 292) && y >= 6 && y < 24 {
			return 52 + n*2, 74 + n*2, 92 + n*2 // windows
		}
		return v * 1.05, v * 0.82, v * 0.7
	}
	if inPlanter(x, y, 26, 124) || inPlanter(x, y, 294, 124) {
		v := 58 + float32((x*5+y*3)%7)*5 + n*5
		return v * 0.5, v, v * 0.45 // shrubs
	}
	if inPlanterRim(x, y, 26, 124) || inPlanterRim(x, y, 294, 124) {
		return 150 + n*3, 146 + n*3, 138 + n*3
	}
	// Paving: 20-pixel slabs, each a little lighter or darker than its neighbours.
	slab := float32(((x/20)*7+(y/20)*13)%5) * 4
	v := 164 + slab + float32(y)*0.08 + n*3.5
	if x%20 == 0 || y%20 == 0 {
		v -= 22
	}
	return v, v * 0.97, v * 0.9
}

func inPlanter(x, y, cx, cy int) bool {
	dx, dy := x-cx, y-cy
	return dx*dx+dy*dy <= 14*14
}

func inPlanterRim(x, y, cx, cy int) bool {
	dx, dy := x-cx, y-cy
	return dx*dx+dy*dy <= 17*17
}

// Parts of a figure, in drawing order.
const (
	partNone = iota
	partLegs
	partBody
	partArm
	partHead
)

// figure is a walker at one frame: where it stands, how tall it is there and
// how far through a stride it is.
type figure struct {
	fx, fy, h float64 // feet position and height in pixels
	swing     float64 // −1…1: how far the leading leg is ahead
	side      float64 // 0…1: how much of the stride is across the image
}

// figureAt returns the walker's pose at frame t, and whether it is in the scene.
func (o object) figureAt(t, sceneH int) (figure, bool) {
	if t < o.t0 || t > o.t1 {
		return figure{}, false
	}
	dt := float64(t - o.t0)
	fx, fy := o.x0+o.vx*dt, o.y0+o.vy*dt
	// Perspective: the camera looks down the square, so figures shrink upwards.
	h := float64(o.h) * (0.52 + 0.48*fy/float64(sceneH))
	speed := math.Hypot(o.vx, o.vy)
	side := 1.0
	if speed > 0 {
		side = math.Abs(o.vx) / speed
	}
	// One stride takes the figure about 0.9 of its height forward.
	phase := 2 * math.Pi * speed * dt / (0.9 * float64(o.h))
	return figure{fx: fx, fy: fy, h: h, swing: math.Sin(phase), side: side}, true
}

// part returns which part of the figure covers the pixel (px, py).
func (fg figure) part(px, py int) int {
	x, y := float64(px)+0.5-fg.fx, fg.fy-(float64(py)+0.5) // y grows upwards from the feet
	h := fg.h
	if y < 0 || y > h {
		return partNone
	}
	head := 0.085 * h
	if dy := y - (h - head); x*x+dy*dy <= head*head {
		return partHead
	}
	const hip, shoulder = 0.47, 0.82
	reach := 0.17 * h * fg.swing * (0.25 + 0.75*fg.side)
	switch {
	case y <= hip*h: // legs: each runs from the hip to its own foot
		down := 1 - y/(hip*h)
		leg := 0.055 * h
		for _, s := range [2]float64{1, -1} {
			if math.Abs(x-s*(0.04*h+reach*down)) <= leg {
				return partLegs
			}
		}
	case y <= shoulder*h:
		down := (shoulder*h - y) / ((shoulder - hip) * h)
		// The arms swing opposite to the legs.
		for _, s := range [2]float64{1, -1} {
			if math.Abs(x-s*(0.15*h-0.6*reach*down)) <= 0.04*h {
				return partArm
			}
		}
		if math.Abs(x) <= 0.125*h {
			return partBody
		}
	}
	return partNone
}

// bounds returns the pixel rectangle that contains the figure.
func (fg figure) bounds() (x0, y0, w, h int) {
	half := 0.36 * fg.h
	x0, y0 = int(math.Floor(fg.fx-half)), int(math.Floor(fg.fy-fg.h))
	return x0, y0, int(math.Ceil(fg.fx+half)) - x0 + 1, int(math.Ceil(fg.fy)) - y0 + 1
}

// person draws a walker and labels its pixels as motion.
func (s *Synthetic) person(o object, t int, f *imgx.RGB, gt *imgx.Gray) {
	fg, ok := o.figureAt(t, s.sc.h)
	if !ok {
		return
	}
	x0, y0, w, h := fg.bounds()
	s.fill(x0, y0, w, h, func(px, py, i int) {
		var c [3]float32
		switch fg.part(px, py) {
		case partNone:
			return
		case partLegs:
			c = o.pants
		case partBody:
			c = o.color
		case partArm:
			c = [3]float32{o.color[0] * 0.82, o.color[1] * 0.82, o.color[2] * 0.82}
		case partHead:
			c = o.skin
		}
		// A faint weave anchored to the figure, so the clothes are not flat.
		d := o.texture
		if ((px-x0)/2+(py-y0)/2)%2 == 1 {
			d = -d
		}
		f.R[i], f.G[i], f.B[i] = imgx.Clamp255(c[0]+d), imgx.Clamp255(c[1]+d), imgx.Clamp255(c[2]+d)
		gt.Pix[i] = gtMotion
	})
}

// personShadow darkens an ellipse on the ground beside the walker's feet.
func (s *Synthetic) personShadow(o object, t int, f *imgx.RGB, gt *imgx.Gray) {
	fg, ok := o.figureAt(t, s.sc.h)
	if !ok || o.shadow <= 0 {
		return
	}
	rx, ry := 0.3*fg.h, 0.075*fg.h
	cx, cy := fg.fx+0.2*fg.h, fg.fy-0.02*fg.h
	x0, y0 := int(math.Floor(cx-rx)), int(math.Floor(cy-ry))
	s.fill(x0, y0, int(2*rx)+2, int(2*ry)+2, func(px, py, i int) {
		dx, dy := (float64(px)+0.5-cx)/rx, (float64(py)+0.5-cy)/ry
		if dx*dx+dy*dy > 1 {
			return
		}
		f.R[i] *= o.shadow
		f.G[i] *= o.shadow
		f.B[i] *= o.shadow
		gt.Pix[i] = gtShadow
	})
}
