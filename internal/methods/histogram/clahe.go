package histogram

import (
	"fmt"
	"math"

	"heatmap/internal/imgx"
)

// CLAHEConfig holds parameters for Contrast Limited Adaptive Histogram Equalization.
type CLAHEConfig struct {
	// GridX and GridY specify the number of contextual tiles across width and height.
	// Typically 8×8. Must be >= 1.
	GridX, GridY int

	// ClipLimit controls contrast limiting. A value of 1.0 disables enhancement beyond uniform.
	// Typical values are in [2.0, 4.0]. Higher values allow higher contrast stretch,
	// but amplify noise in flat regions.
	ClipLimit float64
}

// DefaultCLAHEConfig returns standard defaults: 8×8 grid with clip limit 2.5.
func DefaultCLAHEConfig() CLAHEConfig {
	return CLAHEConfig{
		GridX:     8,
		GridY:     8,
		ClipLimit: 2.5,
	}
}

func (c CLAHEConfig) validate() error {
	if c.GridX < 1 || c.GridY < 1 {
		return fmt.Errorf("histogram: CLAHE grid dimensions must be at least 1, got %dx%d", c.GridX, c.GridY)
	}
	if c.ClipLimit < 1.0 {
		return fmt.Errorf("histogram: CLAHE clip limit must be >= 1.0, got %v", c.ClipLimit)
	}
	return nil
}

// CLAHE applies Contrast Limited Adaptive Histogram Equalization to a grayscale image.
// It enhances local details and expands contrast in low-light/night scenes without
// over-amplifying noise in homogeneous areas.
//
// The image is divided into a GridX × GridY grid of tiles. For each tile,
// a clipped histogram is computed and converted to an equalization LUT.
// Final pixel values are computed via bilinear interpolation of the 4 nearest tile LUTs,
// eliminating block boundaries.
func CLAHE(src *imgx.Gray, cfg CLAHEConfig) (*imgx.Gray, error) {
	if src == nil {
		return nil, fmt.Errorf("histogram: src image cannot be nil")
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	gx, gy := cfg.GridX, cfg.GridY
	if src.W < gx {
		gx = src.W
	}
	if src.H < gy {
		gy = src.H
	}

	tileW := float64(src.W) / float64(gx)
	tileH := float64(src.H) / float64(gy)

	// Precompute equalization LUTs for each tile [gy][gx][Levels]
	luts := make([][][Levels]float32, gy)
	for ty := 0; ty < gy; ty++ {
		luts[ty] = make([][Levels]float32, gx)
		y0 := int(float64(ty) * tileH)
		y1 := int(float64(ty+1) * tileH)
		if ty == gy-1 {
			y1 = src.H
		}

		for tx := 0; tx < gx; tx++ {
			x0 := int(float64(tx) * tileW)
			x1 := int(float64(tx+1) * tileW)
			if tx == gx-1 {
				x1 = src.W
			}

			// Build local tile histogram
			var hist [Levels]int
			tilePixels := (x1 - x0) * (y1 - y0)
			for y := y0; y < y1; y++ {
				for x := x0; x < x1; x++ {
					hist[imgx.ToByte(src.At(x, y))]++
				}
			}

			// Clip histogram and redistribute excess
			clipVal := int(math.Max(1.0, cfg.ClipLimit*float64(tilePixels)/float64(Levels)))
			excess := 0
			for i := 0; i < Levels; i++ {
				if hist[i] > clipVal {
					excess += hist[i] - clipVal
					hist[i] = clipVal
				}
			}

			bonus := excess / Levels
			remainder := excess % Levels
			for i := 0; i < Levels; i++ {
				hist[i] += bonus
				if i < remainder {
					hist[i]++
				}
			}

			// Generate equalization mapping LUT
			luts[ty][tx] = EqualizationLUT(hist)
		}
	}

	dst := imgx.NewGray(src.W, src.H)

	// Bilinear interpolation between tile centers
	for y := 0; y < src.H; y++ {
		// y relative to tile centers: center of tile ty is (ty + 0.5) * tileH
		fy := (float64(y) - 0.5*tileH) / tileH
		ty0 := int(math.Floor(fy))
		ty1 := ty0 + 1
		yt := fy - float64(ty0)

		if ty0 < 0 {
			ty0, ty1 = 0, 0
			yt = 0
		} else if ty1 >= gy {
			ty0, ty1 = gy-1, gy-1
			yt = 0
		}

		for x := 0; x < src.W; x++ {
			fx := (float64(x) - 0.5*tileW) / tileW
			tx0 := int(math.Floor(fx))
			tx1 := tx0 + 1
			xt := fx - float64(tx0)

			if tx0 < 0 {
				tx0, tx1 = 0, 0
				xt = 0
			} else if tx1 >= gx {
				tx0, tx1 = gx-1, gx-1
				xt = 0
			}

			val := imgx.ToByte(src.At(x, y))

			v00 := float64(luts[ty0][tx0][val])
			v10 := float64(luts[ty0][tx1][val])
			v01 := float64(luts[ty1][tx0][val])
			v11 := float64(luts[ty1][tx1][val])

			top := (1.0-xt)*v00 + xt*v10
			bottom := (1.0-xt)*v01 + xt*v11
			interpolated := (1.0-yt)*top + yt*bottom

			dst.Set(x, y, float32(interpolated))
		}
	}

	return dst, nil
}
