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

Stage diagrams (mermaid) with parameters will be added in stage 12.
