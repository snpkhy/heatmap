# Stage 1: build the Go CLI
FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/heatmap ./cmd/heatmap

# Stage 2: runtime (CLI + FFmpeg + Python analysis)
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
