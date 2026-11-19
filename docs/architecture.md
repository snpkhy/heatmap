# Architecture

## Data flow

```
dataset ──► noise ──► pipeline (stages from methods/*) ──► mask ──┬──► metrics (vs ground truth)
                                                                   ├──► heatmap (camera / floor plan)
                                                                   └──► report ──► results/<run_id>/
                                                                                        │
                                                      analysis/ (Python plots) ◄────────┤
                                                      web UI (via HTTP API)    ◄────────┘
```

All image processing lives in Go. The CLI (`cmd/heatmap`) and the web UI are two front-ends over the same `pipeline.Run`.

## Packages

| Package | Role |
|---|---|
| `imgx` | own `Gray`/`RGB` float32 images, PNG/JPEG/BMP I/O |
| `methods/<category>` | the hand-written methods; no dependency on the rest |
| `pipeline` | `Frame`, `Stage`, method registry and adapters, `Build`, `Run` |
| `dataset` | `Sequence` implementations: CDnet, frame folders, synthetic scenes |
| `noise` | seeded Gaussian and salt-and-pepper noise |
| `heatmap` | activity accumulator with decay, colour maps, overlay |
| `metrics` | MSE, PSNR, SSIM, confusion counts against CDnet labels, timing |
| `report` | `metrics.csv`, `summary.csv`, `run.json` |
| `config` | strict YAML for pipelines, experiments, test cases |
| `experiment` | worker pool over many runs, test case checks, benchmark |
| `server` | HTTP API with Server-Sent Events, embedded UI |

## The pipeline model

A pipeline is a list of stages. Every frame becomes a `pipeline.Frame` that flows through them:

| Field | Written by | Read by |
|---|---|---|
| `RGB` | runner (after noise) | color stages |
| `Image` | runner (luma), filtering, histogram, edges, texture, fourier | most stages |
| `Channels` | color stages, `equalize` (with `channel`) | `gaussian_bg` |
| `Mask` | segmentation, `sobel_diff`, `block_change`, `block_matching` | morphology, `components`, `perspective_warp`, metrics, heatmap |
| `Plan` | `perspective_warp` | floor-plan heatmap |
| `Corners` | `harris` | `block_matching` (in the next frame) |
| `Vectors` | `block_matching` | `direction_histogram` |

Each stage returns an image of its output for the frame viewer. Stateful stages (background models, trackers, LBP background) keep state between frames; `Reset` clears it, and every run builds its own pipeline, so parallel runs never share state.

Methods register in `internal/pipeline/stages.go` with a name, a category and a constructor. Constructors read their parameters through `Params` getters that take the default and a help text, so `heatmap list`, `/api/methods` and [methods.md](methods.md) all come from that one place. `Build` rejects unknown methods and parameters, invalid values and pipelines with fewer than 3 stages or 3 categories, before any frame is processed.

Parallelism is across runs (pipelines × sequences × noise levels), not across the frames of one run, because most stages depend on the previous frames.

## Metrics

- Masks are compared with CDnet labels: 255 motion, 50 shadow (background), 0 static; 85 (outside ROI) and 170 (unknown) are ignored. Confusion counts are summed over all evaluated frames before precision/recall/F1/IoU are computed.
- Stages marked *denoise* (filtering, fourier) are compared with the luma of the clean frame (MSE, PSNR, SSIM) to measure how much of the synthetic noise they remove.
- FPS counts processing time only (all stages), not decoding, metrics or file output.

## Web UI

`heatmap serve` starts an HTTP server (127.0.0.1:8080 by default) that exposes a JSON API and serves the UI.

| Part | Choice |
|---|---|
| Framework | React + TypeScript, built with Vite |
| Components | MUI (Material UI), dark theme by default, light optional |
| Charts | MUI X Charts |
| Package manager | pnpm |
| Lint/format | Biome |
| Tests | Vitest |

```
internal/server/   HTTP handlers and JSON API (standard net/http)
web/               Vite project
web/dist/          production build output
web/embed.go       //go:embed all:dist — the built UI is embedded into the Go binary
```

- Development: `go run ./cmd/heatmap serve` on :8080 and `make web-dev` for the Vite dev server on :5173, which proxies `/api` to Go.
- Production: `make build` builds the frontend first, then Go; the result is a single `heatmap` binary.
- `web/dist/.gitkeep` is committed so `go build` works even before the UI has been built.
- All detection happens in Go. The UI starts runs and displays what the API returns; the one thing it computes is the heat layer of the viewer, which it draws on a canvas from the saved motion masks (`web/src/heat/`), so trails, the heatmap at any moment and the palettes can change without another run.
- Fonts (Inter, Space Grotesk, JetBrains Mono, and Geologica for Ukrainian headings, since Space Grotesk has no Cyrillic) are bundled through Fontsource, so the UI works offline.
- The interface is in English and Ukrainian. All its words live in `web/src/i18n.tsx`; names the server sends in English (detection modes, built-in scenes) are translated there by id, and unknown ones are shown as sent.
- Routing uses the URL hash (`#/runs/<id>`), so no router dependency and no server-side routes are needed.

API:

| Endpoint | Purpose |
|---|---|
| `GET /api/health` | version |
| `GET /api/methods`, `/api/pipelines`, `/api/sequences` | what can be run |
| `GET /api/sequences/preview?id=&frame=` | one frame of a sequence as PNG |
| `POST /api/runs` | start a run (pipeline + sequence + noise + frames), returns `run_id` |
| `GET /api/runs`, `GET /api/runs/{id}` | list and details (runs on disk are included after a restart); for a run of this server the details also give the scene and the first frame, so the viewer can follow it live |
| `GET /api/runs/{id}/events` | live progress (Server-Sent Events) |
| `GET /api/runs/{id}/files/{file...}` | heatmaps, stage images, CSV files of a run |
| `DELETE /api/runs/{id}` | cancel an active run or delete a finished one |

Requests can only name sequences inside the data directory and pipelines from the pipelines directory; run files are opened through `os.Root`, so paths cannot escape a run directory.
