# Technical Specification

> Template. The team fills it in and agrees on it with the instructor.

## 1. Topic
Motion detection and activity heatmap construction on surveillance camera video sequences using classical image processing methods.

## 2. Team and responsibilities
| Member | GitHub | Pipelines | Packages/files |
|---|---|---|---|
| Illia | ilavabisevic37-boop | p1, p2 | `methods/filtering/gaussian.go`, `segmentation/runningavg.go`, `otsu.go`, `region_growing.go`, `color/ycbcr.go`, `histogram/clahe.go` |
| Yaroslav | tchytchmek | p3, p4 | `methods/filtering/median.go`, `box.go`, `edges/sobel.go`, `morphology/`, `texture/`, `segmentation/blockthresh.go` |
| Vladyslav | snpkhy | p5, p6 | `methods/fourier/`, `geometry/`, `keypoints/`, `segmentation/framediff.go`, `stats/direction.go`; infrastructure, web UI |
| Volodya | vovikLXUS | p7, p8 | `methods/color/hsv.go`, `chroma.go`, `histogram/equalize.go`, `segmentation/gaussbg.go`, `components.go`, `edges/prewitt.go`, `stats/block.go` |

## 3. Dataset
- Dataset: CDnet 2014
- Categories and sequences: …
- Frame ranges: …
- Total number of frames: … (target ≥ 100)
- Synthetic noise: types, levels, seed: …

## 4. Pipelines
Each member implements 2 pipelines, each with ≥ 3 stages from ≥ 3 distinct categories.

Pool of pipelines: see [pipelines.md](pipelines.md).

| ID | Stages (category) | Owner |
|---|---|---|
| p1 | see pipelines.md | Illia |
| p2 | see pipelines.md | Illia |
| p3 | see pipelines.md | Yaroslav |
| p4 | see pipelines.md | Yaroslav |
| p5 | see pipelines.md | Vladyslav |
| p6 | see pipelines.md | Vladyslav |
| p7 | see pipelines.md | Volodya |
| p8 | see pipelines.md | Volodya |

Methods from labs 5–6 that must not be reused: …

## 5. Complexity level
…

## 6. User interface
Local web UI (`heatmap serve`): React + TypeScript + Vite + MUI, pnpm. Owner: Vladyslav (snpkhy).

## 7. Test cases and metric thresholds
Reference cases live in `configs/testcases.yaml` and run with `heatmap testcases` (also in CI). They use the built-in synthetic scenes, whose ground truth is exact and whose noise is seeded, so results are reproducible. Each case sets minimum F1 / precision / recall / IoU / PSNR / FPS about 10–15 % below the measured value.

| ID | Pipeline | Data | Expectation |
|---|---|---|---|
| TC-01 | p1 | synthetic/street | F1 ≥ 0.85, precision ≥ 0.9, ≥ 10 fps |
| TC-02 | p1 | street + Gaussian σ=10 | F1 ≥ 0.85, PSNR ≥ 28 dB |
| TC-03 | p2 | synthetic/night | F1 ≥ 0.40 |
| TC-04 | p3 | synthetic/street | F1 ≥ 0.60, recall ≥ 0.9 |
| TC-05 | p3 | street + salt & pepper 2 % | F1 ≥ 0.60, PSNR ≥ 28 dB |
| TC-06 | p4 | synthetic/street | F1 ≥ 0.70 |
| TC-07 | p4 | synthetic/shadow | F1 ≥ 0.55 |
| TC-08 | p5 | street + Gaussian σ=10 | F1 ≥ 0.45, PSNR ≥ 26 dB |
| TC-09 | p6 | synthetic/street | F1 ≥ 0.65, recall ≥ 0.9 |
| TC-10 | p7 | synthetic/shadow | F1 ≥ 0.90, precision ≥ 0.90 (shadow suppressed) |
| TC-11 | p7 | street + Gaussian σ=10 | F1 ≥ 0.85 |
| TC-12 | p8 | street + salt & pepper 2 % | F1 ≥ 0.65 |
| TC-13 | p8 | synthetic/night | recall ≥ 0.95 |
