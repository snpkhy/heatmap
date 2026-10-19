BIN        := bin/heatmap
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS    := -X main.version=$(VERSION)
PIPELINE   ?= configs/pipelines/p1.yaml
INPUT      ?= data/cdnet/baseline/highway
EXPERIMENT ?= configs/experiments/main.yaml
GOLANGCI_LINT_VERSION ?= v2.15.0

.PHONY: help setup build build-go web-build web-dev data run experiment testcases bench analysis \
        test test-go test-py test-web lint lint-go lint-py lint-web fmt docker clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  %-12s %s\n",$$1,$$2}'

setup: ## Install Go and Python dependencies and linters
	go mod download
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	uv sync --project analysis
	pnpm -C web install

build: web-build build-go ## Build the web UI and the CLI into bin/heatmap

build-go: ## Build only the Go CLI (uses the last web build)
	go build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/heatmap

web-build: ## Build the web UI into web/dist (embedded into the binary)
	pnpm -C web build

web-dev: ## Run the Vite dev server on :5173 (proxies /api to :8080)
	pnpm -C web dev

data: ## Download CDnet sequences into data/cdnet
	./scripts/download_cdnet.sh

run: build-go ## Run one pipeline (PIPELINE=..., INPUT=...)
	$(BIN) run --pipeline $(PIPELINE) --input $(INPUT) --out results

experiment: build-go ## Run the full experiment (EXPERIMENT=...)
	$(BIN) experiment --config $(EXPERIMENT)

testcases: build-go ## Check reference test cases
	$(BIN) testcases --config configs/testcases.yaml

bench: ## Run Go benchmarks
	go test -run '^$$' -bench . -benchmem ./...

analysis: ## Build plots and tables from results/
	uv run --project analysis python analysis/compare.py

test: test-go test-py test-web ## Run all tests

test-go:
	go test -race ./...

test-py:
	uv run --project analysis pytest analysis

test-web:
	pnpm -C web test

lint: lint-go lint-py lint-web ## Run all linters (must report zero issues)

lint-go:
	golangci-lint run ./...

lint-py:
	uv run --project analysis ruff check analysis
	uv run --project analysis ruff format --check analysis

lint-web:
	pnpm -C web lint

fmt: ## Format code
	golangci-lint fmt ./...
	uv run --project analysis ruff format analysis
	pnpm -C web format

docker: ## Build the Docker image
	docker build -t heatmap .

clean: ## Remove build artifacts
	rm -rf bin
	find web/dist -mindepth 1 ! -name .gitkeep -delete
