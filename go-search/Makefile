# Rice Search - Go Edition
# Makefile for build, test, and development tasks

# Variables
BINARY_NAME := rice-search
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

# Go settings
GO := go
GOFLAGS := -v
GOTESTFLAGS := -race -cover

# Directories
CMD_DIR := ./cmd/rice-search
BUILD_DIR := ./build
MODELS_DIR := ./models

# Default target
.DEFAULT_GOAL := help

##@ Development

.PHONY: build
build: ## Build the binary
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)

.PHONY: build-all
build-all: ## Build for all platforms
	GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_DIR)
	GOOS=linux GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(CMD_DIR)
	GOOS=darwin GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_DIR)
	GOOS=darwin GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(CMD_DIR)
	GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(CMD_DIR)

.PHONY: run
run: ## Run the server
	$(GO) run $(CMD_DIR) serve

.PHONY: dev
dev: ## Run with hot reload (requires air)
	air -c .air.toml

##@ Testing

.PHONY: test
test: ## Run all tests
	$(GO) test $(GOTESTFLAGS) ./...

.PHONY: test-short
test-short: ## Run short tests only
	$(GO) test -short ./...

.PHONY: test-cover
test-cover: ## Run tests with coverage report
	$(GO) test $(GOTESTFLAGS) -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

.PHONY: test-integration
test-integration: ## Run integration tests (requires Qdrant)
	$(GO) test -tags=integration -v ./...

.PHONY: bench
bench: ## Run benchmarks
	$(GO) test -bench=. -benchmem ./...

##@ Code Quality

.PHONY: lint
lint: ## Run linter
	golangci-lint run ./...

.PHONY: lint-fix
lint-fix: ## Run linter and fix issues
	golangci-lint run --fix ./...

.PHONY: fmt
fmt: ## Format code
	$(GO) fmt ./...
	gofumpt -l -w .

.PHONY: vet
vet: ## Run go vet
	$(GO) vet ./...

.PHONY: check
check: fmt vet lint test ## Run all checks (format, vet, lint, test)

##@ Dependencies

.PHONY: deps
deps: ## Download dependencies
	$(GO) mod download

.PHONY: deps-update
deps-update: ## Update dependencies
	$(GO) get -u ./...
	$(GO) mod tidy

.PHONY: tidy
tidy: ## Tidy go.mod
	$(GO) mod tidy

##@ Models

.PHONY: models-download
models-download: ## Download ML models
	./scripts/download-models.sh

.PHONY: models-verify
models-verify: ## Verify model checksums
	./scripts/verify-models.sh

##@ Docker

.PHONY: docker-build
docker-build: ## Build Docker image
	docker build -t rice-search:$(VERSION) -f deployments/docker/Dockerfile .

.PHONY: docker-run
docker-run: ## Run in Docker
	docker run -p 8080:8080 rice-search:$(VERSION)

.PHONY: compose-up
compose-up: ## Start with docker-compose (persistent data)
	docker-compose -f deployments/docker-compose.yml up -d

.PHONY: compose-down
compose-down: ## Stop docker-compose
	docker-compose -f deployments/docker-compose.yml down

.PHONY: compose-logs
compose-logs: ## View docker-compose logs
	docker-compose -f deployments/docker-compose.yml logs -f

##@ Development (Ephemeral)

.PHONY: dev-up
dev-up: ## Start dev environment (ephemeral data, Qdrant UI)
	docker-compose -f deployments/docker-compose.dev.yml up -d

.PHONY: dev-down
dev-down: ## Stop dev environment (data is lost!)
	docker-compose -f deployments/docker-compose.dev.yml down

.PHONY: dev-logs
dev-logs: ## View dev logs
	docker-compose -f deployments/docker-compose.dev.yml logs -f

.PHONY: dev-restart
dev-restart: dev-down dev-up ## Restart dev (fresh data)

##@ Code Generation

.PHONY: generate
generate: ## Run go generate
	$(GO) generate ./...

.PHONY: templ
templ: ## Generate templ templates
	templ generate

##@ Utilities

.PHONY: clean
clean: ## Clean build artifacts
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

.PHONY: install-tools
install-tools: ## Install development tools
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install mvdan.cc/gofumpt@latest
	go install github.com/air-verse/air@latest
	go install github.com/a-h/templ/cmd/templ@latest

.PHONY: help
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
