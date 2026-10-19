BIN        := bin/heatmap
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS    := -X main.version=$(VERSION)
PIPELINE   ?= configs/pipelines/p1.yaml
INPUT      ?= data/cdnet/baseline/highway
EXPERIMENT ?= configs/experiments/main.yaml
GOLANGCI_LINT_VERSION ?= v2.15.0

.PHONY: help setup build data run experiment testcases bench analysis \
        test test-go test-py lint lint-go lint-py fmt docker clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  %-12s %s\n",$$1,$$2}'

setup: ## Install Go and Python dependencies and linters
	go mod download
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	uv sync --project analysis

build: ## Build the CLI into bin/heatmap
	go build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/heatmap

data: ## Download CDnet sequences into data/cdnet
	./scripts/download_cdnet.sh

run: build ## Run one pipeline (PIPELINE=..., INPUT=...)
	$(BIN) run --pipeline $(PIPELINE) --input $(INPUT) --out results

experiment: build ## Run the full experiment (EXPERIMENT=...)
	$(BIN) experiment --config $(EXPERIMENT)

testcases: build ## Check reference test cases
	$(BIN) testcases --config configs/testcases.yaml

bench: build ## Run Go benchmarks
	go test -run '^$$' -bench . -benchmem ./...

analysis: ## Build plots and tables from results/
	uv run --project analysis python analysis/compare.py

test: test-go test-py ## Run all tests

test-go:
	go test -race ./...

test-py:
	uv run --project analysis pytest analysis

lint: lint-go lint-py ## Run all linters (must report zero issues)

lint-go:
	golangci-lint run ./...

lint-py:
	uv run --project analysis ruff check analysis
	uv run --project analysis ruff format --check analysis

fmt: ## Format code
	golangci-lint fmt ./...
	uv run --project analysis ruff format analysis

docker: ## Build the Docker image
	docker build -t heatmap .

clean: ## Remove build artifacts
	rm -rf bin
