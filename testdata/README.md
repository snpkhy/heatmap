# Test data

Tiny files used by unit tests and edge-case checks.

| File | Purpose |
|---|---|
| `images/gray_3x3.png` | 8-bit grayscale, pixels 0, 10, …, 80 row by row |
| `images/rgb_1x1.png` | single RGB pixel (10, 20, 30) |
| `images/rgb_2x1.bmp` | red then blue pixel, BMP format (as CDnet `ROI.bmp`) |
| `images/flat_8x8.jpg` | flat gray 128, JPEG quality 95 |
| `images/empty.png` | zero-byte file |
| `images/corrupt.png` | valid PNG truncated in half |
| `images/not_an_image.png` | text file with a `.png` name |
| `images/unsupported.gif` | GIF, which the project does not accept |
