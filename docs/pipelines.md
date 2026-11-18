# Pipelines

Each of the 4 team members implements 2 pipelines from the pool below. Every pipeline has at least 3 stages from at least 3 distinct categories, and no two members share a pipeline.

## Pool

| ID | Stages (category) | What it tests |
|---|---|---|
| p1 | Gaussian filter (filtering) → running-average background model + Otsu threshold (segmentation) → opening/closing (morphology) | baseline |
| p2 | RGB→YCbCr (color) → CLAHE (histogram) → frame differencing + region growing (segmentation) | night and dark scenes |
| p3 | Median filter (filtering) → Sobel gradient difference (edges) → closing (morphology) | illumination changes |
| p4 | Box filter via integral image (filtering) → block-wise LBP (texture) → block threshold segmentation (segmentation) | shadows |
| p5 | FFT frequency filtering (fourier) → frame differencing with threshold (segmentation) → perspective warp to a floor plan (geometry) | top-down heatmap |
| p6 | Harris corners (keypoints) → block matching between frames (tracking/segmentation) → motion direction histogram (stats) | motion directions |
| p7 | RGB→HSV + chromaticity normalization (color) → histogram equalization (histogram) → per-pixel Gaussian background model (segmentation) | shadows and lighting changes |
| p8 | Prewitt gradient magnitude (edges) → block statistics: mean and variance change (stats) → connected components with area filter (segmentation) | small noise, bad weather |

p7 and p8 were added on top of the original pool of six so that 4 members × 2 pipelines = 8 distinct pipelines. Check all methods against labs 5–6 before finalizing.

## Owners

| Pipelines | Owner |
|---|---|
| p1, p2 | Illia |
| p3, p4 | Yaroslav |
| p5, p6 | Vladyslav |
| p7, p8 | Volodya |

## Configurations

Each pipeline is a YAML file in `configs/pipelines/`. `heatmap run` and the web UI build it from the method registry (see [methods.md](methods.md)) and refuse to run it unless it has at least 3 stages from at least 3 distinct categories. The diagrams below show the shipped parameters.

### p1 — Baseline background subtraction

Owner: Illia. Gaussian blur, running-average background with an Otsu threshold, then opening and closing.

```mermaid
flowchart LR
  in(["frame (RGB)"])
  s1["gaussian<br/><i>filtering</i><br/><small>k=5, sigma=1</small>"]
  in --> s1
  s2["running_avg_otsu<br/><i>segmentation</i><br/><small>alpha=0.05, fg_alpha=0.005, min_threshold=15</small>"]
  s1 --> s2
  s3["opening<br/><i>morphology</i><br/><small>k=3</small>"]
  s2 --> s3
  s4["closing<br/><i>morphology</i><br/><small>k=5</small>"]
  s3 --> s4
  out(["mask → heatmap, metrics"])
  s4 --> out
```

### p2 — Night and dark scenes

Owner: Illia. Luma from YCbCr, CLAHE to lift dark regions, frame differencing grown into regions.

```mermaid
flowchart LR
  in(["frame (RGB)"])
  s1["rgb_to_ycbcr<br/><i>color</i>"]
  in --> s1
  s2["clahe<br/><i>histogram</i><br/><small>clip_limit=3, grid_x=4, grid_y=4</small>"]
  s1 --> s2
  s3["frame_diff_region_growing<br/><i>segmentation</i>"]
  s2 --> s3
  out(["mask → heatmap, metrics"])
  s3 --> out
```

### p3 — Illumination-robust edges

Owner: Yaroslav. Median filter, change of Sobel gradients against a background, closing.

```mermaid
flowchart LR
  in(["frame (RGB)"])
  s1["median<br/><i>filtering</i><br/><small>k=3</small>"]
  in --> s1
  s2["sobel_diff<br/><i>edges</i><br/><small>alpha=0.05, threshold=60</small>"]
  s1 --> s2
  s3["closing<br/><i>morphology</i><br/><small>k=5</small>"]
  s2 --> s3
  out(["mask → heatmap, metrics"])
  s3 --> out
```

### p4 — Shadow-robust texture

Owner: Yaroslav. Box filter, block-wise LBP texture change, block threshold.

```mermaid
flowchart LR
  in(["frame (RGB)"])
  s1["box<br/><i>filtering</i><br/><small>k=3</small>"]
  in --> s1
  s2["block_lbp<br/><i>texture</i><br/><small>alpha=0.05, block=8, margin=0.05</small>"]
  s1 --> s2
  s3["block_threshold<br/><i>segmentation</i><br/><small>block=8, threshold=100</small>"]
  s2 --> s3
  out(["mask → heatmap, metrics"])
  s3 --> out
```

### p5 — Top-down heatmap

Owner: Vladyslav. FFT low-pass, frame differencing, perspective warp of the mask onto a floor plan.

```mermaid
flowchart LR
  in(["frame (RGB)"])
  s1["fft_filter<br/><i>fourier</i><br/><small>cutoff=0.1, shape=gaussian</small>"]
  in --> s1
  s2["frame_diff<br/><i>segmentation</i><br/><small>threshold=8</small>"]
  s1 --> s2
  s3["perspective_warp<br/><i>geometry</i><br/><small>height=200, src=[[0.25, 0.3], [0.75, 0.3], [1, 1], [0, 1]], width=160</small>"]
  s2 --> s3
  out(["mask → heatmap, metrics"])
  s3 --> out
```

### p6 — Motion directions

Owner: Vladyslav. Harris corners, block matching between frames, motion direction histogram.

```mermaid
flowchart LR
  in(["frame (RGB)"])
  s1["harris<br/><i>keypoints</i>"]
  in --> s1
  s2["block_matching<br/><i>segmentation</i><br/><small>block=9, mask_radius=5, max_cost=20, min_motion=1, search=6</small>"]
  s1 --> s2
  s3["direction_histogram<br/><i>stats</i><br/><small>bins=8, min_magnitude=1</small>"]
  s2 --> s3
  out(["mask → heatmap, metrics"])
  s3 --> out
```

### p7 — Shadows and lighting changes

Owner: Volodya. Chromaticity + HSV value, equalized value, per-pixel Gaussian background with shadow suppression.

```mermaid
flowchart LR
  in(["frame (RGB)"])
  s1["chroma_value<br/><i>color</i><br/><small>min_sum=30</small>"]
  in --> s1
  s2["equalize<br/><i>histogram</i><br/><small>channel=2, keep_raw=True</small>"]
  s1 --> s2
  s3["gaussian_bg<br/><i>segmentation</i><br/><small>k=3.5, ratio_channel=3, shadow_channel=2, shadow_k=6, shadow_low=0.4</small>"]
  s2 --> s3
  out(["mask → heatmap, metrics"])
  s3 --> out
```

### p8 — Small noise and bad weather

Owner: Volodya. Prewitt gradients, block mean/variance change detector, connected components with an area filter.

```mermaid
flowchart LR
  in(["frame (RGB)"])
  s1["prewitt<br/><i>edges</i>"]
  in --> s1
  s2["block_change<br/><i>stats</i><br/><small>block=8, persist=2</small>"]
  s1 --> s2
  s3["components<br/><i>segmentation</i><br/><small>connectivity=8, min_area=64</small>"]
  s2 --> s3
  out(["mask → heatmap, metrics"])
  s3 --> out
```

## Results on the built-in synthetic scenes

F1 on clean frames (`heatmap experiment -config configs/experiments/main.yaml`; frames with ground truth only). The scenes are generated by `internal/dataset/synthetic.go` with exact ground truth: `street` (car and pedestrian on textured asphalt), `shadow` (an object with a hard shadow labelled 50, i.e. background) and `night` (two dim objects in a dark, low-contrast scene).

| Pipeline | street | shadow | night | What it shows |
|---|---|---|---|---|
| p1 | 0.937 | 0.657 | 0.994 | strong baseline, but takes the shadow for motion (precision 0.49 on `shadow`) |
| p2 | 0.416 | 0.473 | 0.472 | frame differencing sees object edges, not their interiors |
| p3 | 0.684 | 0.504 | 0.496 | gradient change; high recall, blobby masks after closing |
| p4 | 0.783 | 0.626 | 0.334 | texture (LBP) ignores the shadow better than intensity, weak on flat dark scenes |
| p5 | 0.508 | 0.379 | 0.148 | frame differencing after an FFT low-pass; its point is the top-down heatmap |
| p6 | 0.721 | 0.493 | 0.205 | sparse corner motion; dominant direction 0° (the car drives right) |
| p7 | 0.961 | **0.960** | 0.444 | the only pipeline that suppresses the shadow (precision 0.94) |
| p8 | 0.615 | 0.444 | 0.641 | block statistics: near-perfect recall, coarse 8×8 masks |

With noise the filtering stages matter. Under `salt_pepper:0.02` on `street`, p3 (median filter) keeps F1 0.676 and p8 (area filter) even rises to 0.730, while p2, p5 and p6 fall below 0.2; on `night` p8 collapses too (0.129) and on `shadow` p3 drops to 0.197. Under `gaussian:10`, p1, p5, p6 and p8 barely change, p7 loses a little on `street` (0.961 → 0.915) but fails on `night` (0.003), and p2 fails almost completely (≈ 0.05). `configs/testcases.yaml` turns the key observations into checked thresholds (`heatmap testcases`, run in CI).

