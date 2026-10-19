# heatmap

Motion detection and activity heatmaps for surveillance camera video sequences using **classical image processing** — no AI/ML; every algorithm is implemented by hand in Go.

> Status: stage 1 — repository scaffold. Commands such as `run` and `experiment` are added stage by stage.

## Requirements

- Go 1.27+
- [uv](https://docs.astral.sh/uv/) (manages Python 3.11+ and the `analysis/` dependencies)
- Node.js 24+ and [pnpm](https://pnpm.io/) (web UI only)
- FFmpeg (only for splitting your own videos into frames)
- or just Docker

## Quick start

```bash
make setup   # Go/Python/web dependencies and golangci-lint
make data    # download CDnet into data/cdnet
make run     # run a pipeline (PIPELINE=configs/pipelines/p1.yaml INPUT=...)
make test    # Go and Python tests
make lint    # golangci-lint, ruff and biome
make web-dev # web UI dev server on http://localhost:5173
make help    # all targets
```

With Docker:

```bash
make docker
docker run --rm -v "$PWD/data:/app/data" -v "$PWD/results:/app/results" heatmap help
```

## Layout

```
cmd/heatmap/        CLI
internal/
  imgx/             own image types and I/O
  config/           YAML configs
  dataset/          CDnet and frame-folder loaders
  noise/            seeded synthetic noise
  pipeline/         Stage interface, method registry, runner
  methods/<cat>/    processing methods, one package per category
  heatmap/          heatmap accumulation and rendering
  metrics/          MSE, PSNR, SSIM, F1, IoU, FPS
  report/           CSV/JSON output
configs/            pipelines, experiments, test cases
analysis/           Python: plots and tables from results/
web/                web UI: React + TypeScript + Vite + MUI (pnpm)
scripts/            data download, video frame extraction
docs/               architecture, methods, specification
testdata/           tiny images for tests
```

## Team

| Member | GitHub | Pipelines |
|---|---|---|
| Member 1 | | |
| Member 2 | | |
| Member 3 | | |
| Member 4 | | |

AI tool usage is declared in [META-USAGE.md](META-USAGE.md).
