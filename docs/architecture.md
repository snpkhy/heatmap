# Architecture

> Full write-up comes in stage 12. This file records the decisions made so far.

## Data flow

```
dataset ──► pipeline (stages from methods/*) ──► heatmap ──► metrics ──► report ──► results/<run_id>/
                                                                                         │
                                                       analysis/ (Python plots) ◄────────┤
                                                       web UI (via HTTP API)    ◄────────┘
```

All image processing lives in Go. The CLI and the web UI are two front-ends over the same `pipeline.Runner`.

## Web UI

`heatmap serve` starts an HTTP server on localhost that exposes a JSON API and serves the UI.

| Part | Choice |
|---|---|
| Framework | React + TypeScript, built with Vite |
| Components | MUI (Material UI), light and dark theme |
| Charts | MUI X Charts |
| Package manager | pnpm |
| Lint/format | Biome |
| Tests | Vitest |

Layout:

```
internal/server/   HTTP handlers and JSON API (standard net/http)
web/               Vite project
web/dist/          production build output
web/embed.go       //go:embed all:dist — the built UI is embedded into the Go binary
```

- Development: run the Go API on :8080 (`heatmap serve`, once implemented) and `make web-dev` for the Vite dev server on :5173, which proxies `/api` to Go.
- Production: `make build` builds the frontend first, then Go; the result is a single `heatmap` binary.
- `web/dist/.gitkeep` is committed so `go build` works even before the UI has been built.
- The UI never processes images. It only starts runs and displays what the API returns.

Planned API:

| Endpoint | Purpose |
|---|---|
| `GET /api/methods`, `/api/pipelines`, `/api/sequences` | what can be run |
| `POST /api/runs` | start a run (pipeline + sequence + noise), returns `run_id` |
| `GET /api/runs/{id}/events` | live progress (Server-Sent Events) |
| `GET /api/runs/{id}/...` | stage frames, masks, heatmap, metrics |

Screens: run launcher, frame viewer (original / stages / mask / ground truth / heatmap), metrics, run comparison, and optionally live webcam mode (stage 13).

Timeline: `serve` skeleton with the launcher and viewer after stage 4, the heatmap view after stage 6, metrics and comparison after stage 9.
