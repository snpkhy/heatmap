# Stage 1: build the web UI
FROM node:24-slim AS web
WORKDIR /web
RUN corepack enable
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

# Stage 2: build the Go CLI with the UI embedded
FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
COPY --from=web /web/dist ./web/dist
RUN CGO_ENABLED=0 go build -o /out/heatmap ./cmd/heatmap

# Stage 3: runtime (CLI + FFmpeg + Python analysis)
FROM python:3.12-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ffmpeg curl unzip make \
    && rm -rf /var/lib/apt/lists/*
COPY --from=ghcr.io/astral-sh/uv:0.8.0 /uv /usr/local/bin/uv

WORKDIR /app
COPY analysis/pyproject.toml analysis/uv.lock* analysis/
RUN uv sync --project analysis --no-dev
COPY . .
COPY --from=build /out/heatmap /usr/local/bin/heatmap

ENTRYPOINT ["heatmap"]
CMD ["help"]
