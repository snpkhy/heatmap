package heatmap

import (
	"fmt"

	"heatmap/internal/imgx"
)

// stop is a control point of a color map: the color at position t ∈ [0, 1].
type stop struct {
	t       float32
	r, g, b float32
}

// inferno approximates the perceptually uniform "inferno" map with a few
// control points: black → purple → red → orange → pale yellow. Brightness
// grows monotonically, so the map also reads correctly in grayscale.
var inferno = []stop{
	{0.00, 0, 0, 4},
	{0.25, 87, 16, 110},
	{0.50, 188, 55, 84},
	{0.75, 249, 142, 9},
	{1.00, 252, 255, 164},
}

// jet is the classic blue → cyan → green → yellow → red map. It is not
// perceptually uniform but familiar from surveillance analytics tools.
var jet = []stop{
	{0.00, 0, 0, 143},
	{0.125, 0, 0, 255},
	{0.375, 0, 255, 255},
	{0.625, 255, 255, 0},
	{0.875, 255, 0, 0},
	{1.00, 128, 0, 0},
}

// Colormap names accepted by Colorize.
const (
	Inferno = "inferno"
	Jet     = "jet"
)

func stops(name string) ([]stop, error) {
	switch name {
	case "", Inferno:
		return inferno, nil
	case Jet:
		return jet, nil
	}
	return nil, fmt.Errorf("heatmap: unknown color map %q", name)
}

// lookup returns the color at t ∈ [0, 1] by linear interpolation between the
// two surrounding control points.
func lookup(s []stop, t float32) (r, g, b float32) {
	t = max(0, min(1, t))
	for i := 1; i < len(s); i++ {
		if t <= s[i].t {
			a, c := s[i-1], s[i]
			f := (t - a.t) / (c.t - a.t)
			return a.r + f*(c.r-a.r), a.g + f*(c.g-a.g), a.b + f*(c.b-a.b)
		}
	}
	last := s[len(s)-1]
	return last.r, last.g, last.b
}

// Colorize maps a heatmap with values in [0, 255] to pseudo-colors.
func Colorize(heat *imgx.Gray, name string) (*imgx.RGB, error) {
	s, err := stops(name)
	if err != nil {
		return nil, err
	}
	// A 256-entry table: every pixel is then a single lookup.
	var lut [256][3]float32
	for i := range lut {
		lut[i][0], lut[i][1], lut[i][2] = lookup(s, float32(i)/255)
	}
	out := imgx.NewRGB(heat.W, heat.H)
	for i, v := range heat.Pix {
		c := lut[imgx.ToByte(v)]
		out.R[i], out.G[i], out.B[i] = c[0], c[1], c[2]
	}
	return out, nil
}

// Overlay blends a colorized heatmap over a frame:
//
//	out = (1 − a)·frame + a·color,   a = alpha · heat/255
//
// so quiet areas keep the original frame and the most active ones get the
// full alpha. Pixels with heat below minLevel (0–255) are left untouched.
func Overlay(frame *imgx.RGB, heat *imgx.Gray, name string, alpha, minLevel float32) (*imgx.RGB, error) {
	if frame.W != heat.W || frame.H != heat.H {
		return nil, fmt.Errorf("%w: frame %dx%d, heatmap %dx%d", imgx.ErrSizeMismatch, frame.W, frame.H, heat.W, heat.H)
	}
	if !(alpha >= 0 && alpha <= 1) {
		return nil, fmt.Errorf("heatmap: alpha %v must be in [0, 1]", alpha)
	}
	color, err := Colorize(heat, name)
	if err != nil {
		return nil, err
	}
	out := frame.Clone()
	for i, v := range heat.Pix {
		if v < minLevel || v <= 0 {
			continue
		}
		a := alpha * min(v, 255) / 255
		out.R[i] = (1-a)*out.R[i] + a*color.R[i]
		out.G[i] = (1-a)*out.G[i] + a*color.G[i]
		out.B[i] = (1-a)*out.B[i] + a*color.B[i]
	}
	return out, nil
}
