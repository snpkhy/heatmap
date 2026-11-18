# User Guide

## 1. Install

```bash
make setup
```

This downloads the Go modules, installs golangci-lint, creates the Python environment for `analysis/` with uv and installs the web dependencies with pnpm. Only Go is needed for the CLI; Node.js 24+ and pnpm are needed to build the web UI.

## 2. Data

Three **synthetic scenes** are built in and need no download: `synthetic/street`, `synthetic/shadow`, `synthetic/night`. They have exact ground truth; the first 10 frames have none, so background models can settle.

**CDnet 2014**: `make data` (or `scripts/download_cdnet.sh [category/sequence ...]`) downloads the archive once into `data/cdnet/` and unpacks the selected sequences to `data/cdnet/<category>/<sequence>/`. Set `CDNET_URL` to use a mirror, or put `dataset2014.zip` into `data/cdnet/` yourself. Ground truth is used only inside each sequence's temporal ROI.

**Your own video**: split it into frames and put them under `data/frames/<name>/`:

```bash
scripts/extract_frames.sh my_video.mp4 data/frames/my_video 10   # 10 fps
```

Folders of frames have no ground truth: you get heatmaps and speed, but no precision/recall.

`heatmap list` shows every sequence it finds. Sequence ids are `synthetic/<name>`, `cdnet/<category>/<sequence>` and `frames/<name>`; the CLI also accepts a directory path.

## 3. Web UI

```bash
make serve        # or: go run ./cmd/heatmap serve
```

Open <http://localhost:8080>.

- **New run**: choose a pipeline (its stages are shown with their categories), a sequence (with a preview frame), optional synthetic noise and the number of frames, then **Start run**.
- **Run page**: live progress with the current input and mask while it runs. When done:
  - tiles with F1, precision, recall, IoU, speed, denoising PSNR/SSIM and, for p6, the main motion direction;
  - **Frames**: input, the output of every stage, the motion mask and the ground truth for any frame, with playback;
  - **Heatmap**: the activity heatmap over the scene, on its own and, for p5, on the top-down floor plan;
  - **Metrics**: per-frame F1/precision/recall, time per stage, and the direction histogram for p6.
- **Runs**: every run (including those made with `heatmap run`), with cancel/delete. Tick two or more to **Compare** them.

For UI development run `go run ./cmd/heatmap serve` and `make web-dev`; Vite serves the UI on :5173 and proxies `/api` to :8080.

The server listens on 127.0.0.1 by default. Runs started from the UI save every stage image of every frame, so they are capped at 500 frames (`-max-frames`); two run at a time (`-parallel`).

## 4. Command line

| Command | What it does |
|---|---|
| `heatmap list [-json]` | methods with categories and parameters, pipelines, sequences |
| `heatmap run -pipeline p1 -input synthetic/street` | one run; flags: `-noise gaussian:10` / `salt_pepper:0.02`, `-seed`, `-start`, `-frames`, `-save-frames`, `-decay`, `-colormap inferno\|jet`, `-out` |
| `heatmap experiment -config configs/experiments/main.yaml` | pipelines × sequences × noise in parallel (`-workers`) |
| `heatmap testcases [-json]` | runs `configs/testcases.yaml`, exit code 1 if a threshold fails |
| `heatmap bench -input synthetic/street -frames 100` | every pipeline sequentially, then in parallel; FPS and speedup |
| `heatmap serve [-addr 127.0.0.1:8080]` | web UI and HTTP API |

`-start -1` (the default) starts 50 frames before the ground truth begins, so the run is evaluated on frames with labels.

## 5. Outputs

Each run writes `results/<run_id>/`:

| File | Content |
|---|---|
| `run.json` | summary, pipeline config, per-frame metrics |
| `metrics.csv` | one row per frame and stage (time; MSE/PSNR/SSIM for denoising stages) plus a `final` row per frame with precision/recall/F1/IoU |
| `summary.csv` | one row with the run's totals |
| `heatmap.png`, `overlay.png` | normalized activity heatmap and the heatmap over the first frame |
| `plan_heatmap.png` | heatmap on the floor plan (pipelines with a geometry stage) |
| `frames/<n>/…` | with `-save-frames`: input, each stage, mask, ground truth |

An experiment writes one such directory per run plus combined `metrics.csv`, `summary.csv` and `experiment.json` under `results/<name>-<time>/`.

Mask metrics follow CDnet: confusion counts are summed over all evaluated frames; ground-truth pixels marked 85 (outside ROI) and 170 (unknown) are ignored, shadows (50) count as background.

## 6. Analysis

```bash
make analysis                      # latest run or experiment in results/
make analysis RUN=results/main-20261119-004913
```

`analysis/plots.py` writes `plots/f1_by_sequence.png`, `f1_by_noise.png`, `fps.png` and `f1_per_frame.png`; `analysis/compare.py` writes `comparison.md` and `comparison.csv`.

## 7. Writing a pipeline

Add `configs/pipelines/<id>.yaml`:

```yaml
id: my1
name: My pipeline
owner: me
stages:
  - method: median
    params: {k: 3}
  - method: frame_diff
    params: {threshold: 20}
  - method: closing
    params: {k: 5}
```

It needs at least 3 stages from at least 3 categories, and some stage must produce a motion mask. Unknown methods, unknown parameters and invalid values are reported before the first frame. Parameters and defaults of every method: [methods.md](methods.md) or `heatmap list`.

## 8. Development

```bash
make test    # Go (with -race), Python, web
make lint    # golangci-lint, ruff, biome — must report zero issues
go run ./cmd/heatmap testcases
```
