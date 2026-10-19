package imgx

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"

	// Decoders registered for image.Decode. Libraries are used for I/O only.
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/bmp" // CDnet ROI masks are BMP files
)

var (
	// ErrEmptyFile is returned when an image file has zero bytes.
	ErrEmptyFile = errors.New("imgx: empty file")
	// ErrUnsupportedFormat is returned when the file is not PNG, JPEG or BMP.
	ErrUnsupportedFormat = errors.New("imgx: unsupported image format")
	// ErrCorrupt is returned when the file looks like a supported image but cannot be decoded.
	ErrCorrupt = errors.New("imgx: corrupt image")
)

// Load reads a PNG, JPEG or BMP file into an RGB image.
// Grayscale files are loaded with equal R, G and B channels.
// Errors wrap ErrEmptyFile, ErrUnsupportedFormat or ErrCorrupt so callers
// can report a clear message and continue with the next file.
func Load(path string) (*RGB, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("imgx: read %s: %w", path, err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("%s: %w", path, ErrEmptyFile)
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		if errors.Is(err, image.ErrFormat) {
			return nil, fmt.Errorf("%s: %w", path, ErrUnsupportedFormat)
		}
		return nil, fmt.Errorf("%s: %w: %w", path, ErrCorrupt, err)
	}
	return FromImage(img), nil
}

// LoadGray reads an image file and converts it to grayscale (see RGB.Luma).
// Files that are already grayscale keep their exact values.
func LoadGray(path string) (*Gray, error) {
	c, err := Load(path)
	if err != nil {
		return nil, err
	}
	if c.IsGrayscale() {
		return c.Channel(0), nil
	}
	return c.Luma(), nil
}

// FromImage converts any image.Image into an RGB image with values in [0, 255].
// Alpha is ignored. A zero-sized image panics, as image decoders never produce one.
func FromImage(img image.Image) *RGB {
	b := img.Bounds()
	out := NewRGB(b.Dx(), b.Dy())
	for y := 0; y < out.H; y++ {
		for x := 0; x < out.W; x++ {
			// RGBA returns 16-bit values; >>8 brings them to 8 bits.
			r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			out.Set(x, y, float32(r>>8), float32(g>>8), float32(bl>>8))
		}
	}
	return out
}

// ToImage converts a Gray image to *image.Gray, rounding and clamping values.
func (g *Gray) ToImage() *image.Gray {
	out := image.NewGray(image.Rect(0, 0, g.W, g.H))
	for i, v := range g.Pix {
		out.Pix[i] = ToByte(v)
	}
	return out
}

// ToImage converts an RGB image to *image.RGBA, rounding and clamping values.
func (c *RGB) ToImage() *image.RGBA {
	out := image.NewRGBA(image.Rect(0, 0, c.W, c.H))
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			r, g, b := c.At(x, y)
			out.SetRGBA(x, y, color.RGBA{R: ToByte(r), G: ToByte(g), B: ToByte(b), A: 255})
		}
	}
	return out
}

// SavePNG writes an image to path as PNG, creating parent directories as needed.
func SavePNG(path string, img image.Image) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("imgx: create dir for %s: %w", path, err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("imgx: create %s: %w", path, err)
	}
	if err := png.Encode(f, img); err != nil {
		_ = f.Close()
		return fmt.Errorf("imgx: encode %s: %w", path, err)
	}
	return f.Close()
}
