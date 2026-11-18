# Methods

Every processing method is implemented by hand in Go under `internal/methods/<category>/` and registered as a pipeline stage in `internal/pipeline/stages.go`. This page lists all registered stages with their parameters and defaults; `heatmap list` prints the same from the binary, and the web UI shows the stages of each pipeline.

A stage reads and writes the `Frame` that flows through a pipeline (see [architecture.md](architecture.md)): `Image` (working single-channel image), `Channels` (colour features), `Mask` (motion mask), `Plan` (mask on a floor plan), `Corners` and `Vectors`.

Stages marked *denoise* are compared with the clean frame (MSE, PSNR, SSIM) when synthetic noise is added.

## color

### `chroma_value`

Chromaticity r, g plus HSV value V (p7); Channels = r, g, V and Image = V.

| Parameter | Type | Default | Meaning |
|---|---|---|---|
| `min_sum` | float | `30` | R+G+B below which chromaticity is neutral |

### `rgb_to_hsv`

HSV; Channels = H (scaled to 0–255), S (0–255), V and Image = V.

### `rgb_to_ycbcr`

BT.601 YCbCr; Channels = Y, Cb, Cr and Image = Y (luma).

## edges

### `prewitt`

Prewitt gradient magnitude of Image (p8).

### `sobel`

Sobel gradient magnitude of Image.

### `sobel_diff`

Motion as a change of Sobel gradients against a background model (p3).

| Parameter | Type | Default | Meaning |
|---|---|---|---|
| `threshold` | float | `60` | smallest gradient change that counts as motion |
| `alpha` | float | `0.05` | background learning rate |

## filtering

### `box` *(denoise)*

Mean of the k×k neighbourhood via an integral image (constant cost per pixel).

| Parameter | Type | Default | Meaning |
|---|---|---|---|
| `k` | int | `3` | window size, odd |

### `gaussian` *(denoise)*

Gaussian blur with a k×k kernel.

| Parameter | Type | Default | Meaning |
|---|---|---|---|
| `k` | int | `5` | kernel size, odd |
| `sigma` | float | `1` | standard deviation in pixels |

### `median` *(denoise)*

Median of the k×k neighbourhood; removes impulse noise, keeps edges.

| Parameter | Type | Default | Meaning |
|---|---|---|---|
| `k` | int | `3` | window size, odd |

## fourier

### `fft_filter` *(denoise)*

Frequency-domain filter of Image via the FFT (p5).

| Parameter | Type | Default | Meaning |
|---|---|---|---|
| `shape` | string | `gaussian` | gaussian, butterworth or ideal |
| `cutoff` | float | `0.1` | cut-off in cycles per pixel, (0, 0.5] |
| `order` | int | `2` | Butterworth order |
| `high_pass` | bool | `False` | keep high frequencies instead |

## geometry

### `perspective_warp`

Warp Mask onto a top-down floor plan; Plan = warped mask (p5).

| Parameter | Type | Default | Meaning |
|---|---|---|---|
| `src` | points | `[[0.3, 0.35], [0.7, 0.35], [1, 1], [0, 1]]` | floor quadrilateral in the image, as fractions of width/height, clockwise from top-left |
| `width` | int | `160` | plan width in pixels |
| `height` | int | `200` | plan height in pixels |
| `interp` | string | `nearest` | nearest or bilinear |

## histogram

### `clahe`

Contrast limited adaptive histogram equalization of Image.

| Parameter | Type | Default | Meaning |
|---|---|---|---|
| `grid_x` | int | `8` | tiles across |
| `grid_y` | int | `8` | tiles down |
| `clip_limit` | float | `2.5` | contrast limit, >= 1 |

### `equalize`

Global histogram equalization of Image, or of one of the Channels.

| Parameter | Type | Default | Meaning |
|---|---|---|---|
| `channel` | int | `-1` | index into Channels to equalize, -1 for Image |
| `keep_raw` | bool | `False` | append the unequalized channel to Channels (for shadow ratios) |

## keypoints

### `harris`

Harris corners of Image; writes Corners, view marks them (p6).

| Parameter | Type | Default | Meaning |
|---|---|---|---|
| `k` | float | `0.04` | Harris sensitivity |
| `sigma` | float | `1` | window standard deviation |
| `quality` | float | `0.01` | fraction of the strongest response |
| `min_response` | float | `10000` | absolute response floor |
| `min_distance` | int | `5` | minimum distance between corners |
| `max_corners` | int | `500` | cap on corners, 0 = no limit |

## morphology

### `closing`

Dilation then erosion of Mask: fills holes, joins fragments.

| Parameter | Type | Default | Meaning |
|---|---|---|---|
| `k` | int | `3` | structuring element size, odd |

### `dilate`

Binary dilation of Mask.

| Parameter | Type | Default | Meaning |
|---|---|---|---|
| `k` | int | `3` | structuring element size, odd |

### `erode`

Binary erosion of Mask.

| Parameter | Type | Default | Meaning |
|---|---|---|---|
| `k` | int | `3` | structuring element size, odd |

### `opening`

Erosion then dilation of Mask: removes specks.

| Parameter | Type | Default | Meaning |
|---|---|---|---|
| `k` | int | `3` | structuring element size, odd |

## segmentation

### `block_matching`

Match the previous frame's corners into this frame (SAD); writes Vectors and a Mask of moving corners (p6).

| Parameter | Type | Default | Meaning |
|---|---|---|---|
| `block` | int | `9` | matched window size, odd |
| `search` | int | `8` | largest displacement tried |
| `max_cost` | float | `20` | largest mean absolute difference accepted |
| `min_motion` | float | `1` | displacement from which a corner is moving |
| `mask_radius` | int | `4` | disc radius drawn per moving corner |

### `block_threshold`

A block is foreground when its mean Image value is above threshold (p4).

| Parameter | Type | Default | Meaning |
|---|---|---|---|
| `block` | int | `16` | block size in pixels |
| `threshold` | float | `100` | mean value above which a block is foreground |

### `components`

Keep connected components of Mask with area in [min_area, max_area] (p8).

| Parameter | Type | Default | Meaning |
|---|---|---|---|
| `connectivity` | int | `8` | 4 or 8 |
| `min_area` | int | `20` | smallest kept component in pixels |
| `max_area` | int | `0` | largest kept component, 0 = no limit |

### `frame_diff`

|I(t) - I(t-1)| > threshold (p5).

| Parameter | Type | Default | Meaning |
|---|---|---|---|
| `threshold` | float | `25` | smallest change that counts as motion |

### `frame_diff_region_growing`

Frame difference seeds grown into connected regions (p2).

| Parameter | Type | Default | Meaning |
|---|---|---|---|
| `seed_threshold` | float | `30` | difference that starts a region |
| `grow_threshold` | float | `12` | difference that lets a region grow |
| `max_intensity_diff` | float | `25` | intensity similarity limit, <= 0 off |
| `connectivity` | int | `8` | 4 or 8 |

### `gaussian_bg`

Per-pixel Gaussian background model on Channels (or Image), optional shadow suppression (p7).

| Parameter | Type | Default | Meaning |
|---|---|---|---|
| `alpha` | float | `0.02` | background learning rate |
| `fg_alpha` | float | `0.002` | learning rate under foreground |
| `k` | float | `3.5` | threshold in standard deviations |
| `init_var` | float | `64` | initial variance |
| `min_var` | float | `16` | variance floor |
| `warmup` | int | `20` | frames of cumulative averaging |
| `shadow_channel` | int | `-1` | brightness channel for shadows, -1 off |
| `ratio_channel` | int | `-1` | passive channel for the shadow ratio, -1 none |
| `shadow_low` | float | `0.5` | darkest shadow relative to background |
| `shadow_k` | float | `3.5` | color tolerance of shadows in std devs |
| `mark_shadows` | bool | `False` | write 50 for shadow pixels |

### `otsu`

Global Otsu threshold of Image.

### `running_avg_otsu`

Running-average background model with an Otsu threshold on the difference (p1).

| Parameter | Type | Default | Meaning |
|---|---|---|---|
| `alpha` | float | `0.05` | background learning rate |
| `fg_alpha` | float | `0.005` | learning rate under foreground |
| `min_threshold` | float | `15` | lowest allowed threshold |

## stats

### `block_change`

Motion from changes of block mean and variance, adaptive thresholds (p8).

| Parameter | Type | Default | Meaning |
|---|---|---|---|
| `block` | int | `8` | block size in pixels |
| `mean_thresh` | float | `1` | floor of the mean threshold |
| `std_thresh` | float | `1.5` | floor of the std threshold |
| `noise_factor` | float | `3` | threshold in units of typical fluctuation, 0 off |
| `noise_step` | float | `0.02` | fluctuation tracking step |
| `step` | float | `0.05` | reference tracking step |
| `warmup` | int | `10` | frames of cumulative averaging |
| `persist` | int | `2` | frames a block must stay changed |

### `direction_histogram`

Histogram of motion directions over the run (p6); view draws the vectors.

| Parameter | Type | Default | Meaning |
|---|---|---|---|
| `bins` | int | `8` | number of direction bins |
| `min_magnitude` | float | `1` | ignore shorter vectors |
| `weighted` | bool | `False` | weight vectors by length |

## texture

### `block_lbp`

Per-block LBP histogram distance to a background model; Image = change map 0–255 (p4).

| Parameter | Type | Default | Meaning |
|---|---|---|---|
| `block` | int | `16` | block size in pixels |
| `margin` | float | `0.05` | relative LBP noise margin |
| `alpha` | float | `0.05` | background learning rate |

### `lbp`

Local binary pattern code of every pixel.

| Parameter | Type | Default | Meaning |
|---|---|---|---|
| `margin` | float | `0.05` | relative noise margin |
