# heatmap

Motion detection and activity heatmaps for surveillance camera video sequences using **classical image processing** — no AI/ML; every algorithm is implemented by hand in Go.

Eight pipelines (p1–p8) combine 30 hand-written methods from 11 categories. They run on CDnet 2014 sequences, your own frame folders, or three built-in synthetic scenes with exact ground truth, so everything works right after cloning — no download needed.

## Requirements

- Go 1.27+
- [uv](https://docs.astral.sh/uv/) (manages Python 3.11+ and the `analysis/` dependencies)
- Node.js 24+ and [pnpm](https://pnpm.io/) (web UI only)
- FFmpeg (only for splitting your own videos into frames)
- or just Docker

## Quick start

```bash
make setup     # Go/Python/web dependencies and golangci-lint
make serve     # build the UI and the CLI, open http://localhost:8080
```

Without the UI:

```bash
go run ./cmd/heatmap list                                         # methods, pipelines, sequences
go run ./cmd/heatmap run -pipeline p7 -input synthetic/shadow     # one run → results/<run_id>/
go run ./cmd/heatmap experiment -config configs/experiments/main.yaml
go run ./cmd/heatmap testcases                                    # reference test cases
go run ./cmd/heatmap bench                                        # sequential vs parallel
make analysis                                                     # plots and tables for the latest run
```

More in the [user guide](docs/user_guide.md). All make targets: `make help`.

With Docker:

```bash
make docker
docker run --rm -p 8080:8080 -v "$PWD/data:/app/data" -v "$PWD/results:/app/results" heatmap serve -addr 0.0.0.0:8080
```

## Layout

```
cmd/heatmap/        CLI: list, run, experiment, testcases, bench, serve
internal/
  imgx/             own image types and I/O
  methods/<cat>/    processing methods, one package per category
  pipeline/         Frame, Stage, method registry, runner
  config/           YAML configs (pipelines, experiments, test cases)
  dataset/          CDnet, frame folders, synthetic scenes
  noise/            seeded synthetic noise
  heatmap/          heatmap accumulation, colour maps, overlay
  metrics/          MSE, PSNR, SSIM, precision/recall/F1/IoU, FPS
  report/           metrics.csv, summary.csv, run.json
  experiment/       parallel experiments, test cases, benchmark
  server/           HTTP API + SSE for the web UI
configs/            pipelines, experiments, test cases
analysis/           Python: plots and tables from results/
web/                web UI: React + TypeScript + Vite + MUI (pnpm)
scripts/            data download, video frame extraction
docs/               architecture, methods, pipelines, user guide, specification
testdata/           tiny images for tests
```

## Team

| Member | GitHub | Pipelines |
|---|---|---|
| Illia | ilavabisevic37-boop | p1, p2 |
| Yaroslav | tchytchmek | p3, p4 |
| Vladyslav | snpkhy | p5, p6 |
| Volodya | vovikLXUS | p7, p8 |

AI tool usage is declared in [META-USAGE.md](META-USAGE.md).
