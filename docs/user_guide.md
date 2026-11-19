# User Guide

## 1. Install

```bash
make setup
```

This downloads the Go modules, installs golangci-lint, creates the Python environment for `analysis/` with uv and installs the web dependencies with pnpm. Only Go is needed for the CLI; Node.js 24+ and pnpm are needed to build the web UI.

## 2. Data

Four **synthetic scenes** are built in and need no download: `synthetic/plaza` (people walking across a square), `synthetic/street`, `synthetic/shadow` and `synthetic/night`. They have exact ground truth; the first 10 frames have none, so background models can settle.

**CDnet 2014**: `make data` (or `scripts/download_cdnet.sh [category/sequence ...]`) downloads the selected sequences, one archive each, and unpacks them to `data/cdnet/<category>/<sequence>/`. The archives are kept in `data/cdnet/.archives`, so nothing is downloaded twice. Set `CDNET_URL` to use a mirror. For footage of people start with `scripts/download_cdnet.sh baseline/pedestrians shadow/peopleInShade shadow/busStation` (about 65 MB). Ground truth is used only inside each sequence's temporal ROI.

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

Open <http://localhost:8080>. The interface is in English or Ukrainian: switch with **EN / УК** in the top bar (the choice is remembered; a Ukrainian browser starts in Ukrainian).

- **Scenes**: every sequence the server finds, real footage first. Hover a card to preview its footage. Above the cards choose the detection mode (a pipeline, listed by its name), the length in frames and optional synthetic noise, then press **Analyze**. A scene that was already analysed with the settings on screen shows its heat and offers **Watch** instead; change the mode or the noise and it offers **Analyze** again. The settings are remembered.
- **Viewer**: opens as soon as the analysis starts and follows it frame by frame; when it is done the footage plays in a loop. The **Detection mode** box at the top shows the same frames through another mode: it opens that analysis, or runs it first.
  - **Trails**: every moving person leaves heat that fades (the *Trail length* slider sets how fast). **Heatmap**: the heat adds up from the first frame to the current one. **Video only**: the footage alone.
  - *Thermal view* hides the footage and shows the heat on black; *Outline* draws a line around everything that moves in the current frame; three palettes; *Intensity* sets the strength of the heat layer.
  - The timeline shows how much was moving in every frame; drag it to seek. Space plays and pauses, ← and → step one frame, the speed button cycles 0.5×–4×.
  - Beside the picture: the share of the picture in motion now, the share of the scene that has seen movement so far, the busiest frame (with a jump link) and, once done, F1/precision/recall, speed, denoising PSNR/SSIM, the main motion direction for p6 and the floor-plan heatmap for p5.
  - **How it was detected**: the frame on screen after every stage, the motion mask and the ground truth. **Charts**: per-frame F1/precision/recall, time per stage, and the direction histogram for p6.
- **History**: every run (including those made with `heatmap run`), with cancel/delete. Tick two or more to **Compare** their heatmaps and metrics. Runs made on the command line without `-save-frames` have no frames to play; the viewer shows their final heatmap.

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
