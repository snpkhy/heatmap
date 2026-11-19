# AI Tool Usage Declaration

The repository structure, infrastructure (Makefile, CI, Dockerfile, linter configs) and parts of the code were created with **Claude Code** (Anthropic). Every generated piece was reviewed by team members as described below.

> AI was used only as a development tool. The image processing methods themselves use no AI/ML.

| Tool | Key prompts | Generated parts | Reviewed by | How it was checked |
|---|---|---|---|---|
| Claude Code | "Scaffold the repository from the brief" | folder layout, `Makefile`, `.golangci.yml`, CI, `Dockerfile`, doc templates | … | review, `make test lint` |
| Claude Code | "Implement the pipelines p5 and p6", "make everything work end to end, including the UI" | methods for p5/p6 (`fourier`, `geometry`, `keypoints`, frame differencing, direction histogram); `pipeline`, `dataset`, `noise`, `heatmap`, `config`, `report`, `experiment`, `server` packages; CLI commands; `configs/`; `analysis/plots.py`, `compare.py`; web UI pages; docs | … | `go test -race`, `golangci-lint`, `heatmap testcases`, pytest, vitest, manual runs of the CLI and the UI |
| Claude Code | "Make the app beautiful: show how people walk and the heat they leave" | redesigned web UI (scene gallery, live viewer with heat trails, history, comparison; `web/src/heat/`), `synthetic/plaza` scene, live-run details in the API, per-sequence `download_cdnet.sh` | … | vitest, biome, `go test -race`, manual runs of the UI on synthetic and CDnet footage |
