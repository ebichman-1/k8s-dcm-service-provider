# K8s Service Provider Makefile
# This Makefile provides build automation for the K8s Service Provider microservice

# Variables
APP_NAME := k8s-service-provider
BINARY_NAME := $(APP_NAME)
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.1.0")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")

# Go variables
GO_VERSION := 1.21
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)
CGO_ENABLED := 0

# Container variables
IMAGE_NAME := $(APP_NAME)
IMAGE_TAG := $(VERSION)
CONTAINER_NAME := $(APP_NAME)

# Directories
BUILD_DIR := ./bin
DIST_DIR := ./dist
COVERAGE_DIR := ./coverage

# Go build flags
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.gitCommit=$(GIT_COMMIT) -w -s"
BUILD_FLAGS := -v $(LDFLAGS)

# Default target
.DEFAULT_GOAL := help

.PHONY: help
help: ## Display this help message
	@echo "K8s Service Provider Build Commands"
	@echo "=================================="
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development
.PHONY: dev
dev: ## Run the application in development mode with hot reload
	@echo "Starting development server..."
	go run cmd/server/main.go

.PHONY: deps
deps: ## Download and install dependencies
	@echo "Downloading dependencies..."
	go mod download
	go mod verify

.PHONY: deps-update
deps-update: ## Update dependencies to latest versions
	@echo "Updating dependencies..."
	go get -u ./...
	go mod tidy

.PHONY: deps-clean
deps-clean: ## Clean module cache
	@echo "Cleaning module cache..."
	go clean -modcache

##@ Build
.PHONY: build
build: deps ## Build the application binary
	@echo "Building $(BINARY_NAME) for $(GOOS)/$(GOARCH)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) cmd/server/main.go
	@echo "Binary built: $(BUILD_DIR)/$(BINARY_NAME)"

.PHONY: build-linux
build-linux: ## Build the application binary for Linux
	@echo "Building $(BINARY_NAME) for linux/amd64..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=amd64 go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 cmd/server/main.go

.PHONY: build-windows
build-windows: ## Build the application binary for Windows
	@echo "Building $(BINARY_NAME) for windows/amd64..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=$(CGO_ENABLED) GOOS=windows GOARCH=amd64 go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe cmd/server/main.go

.PHONY: build-darwin
build-darwin: ## Build the application binary for macOS
	@echo "Building $(BINARY_NAME) for darwin/amd64..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=$(CGO_ENABLED) GOOS=darwin GOARCH=amd64 go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 cmd/server/main.go

.PHONY: build-all
build-all: build-linux build-windows build-darwin ## Build binaries for all platforms

##@ Testing
.PHONY: test
test: ## Run all tests
	@echo "Running tests..."
	go test -v -race ./...

.PHONY: test-short
test-short: ## Run tests with short flag
	@echo "Running short tests..."
	go test -v -short ./...

.PHONY: test-unit
test-unit: ## Run unit tests only
	@echo "Running unit tests..."
	go test -v -race ./internal/...

.PHONY: test-integration
test-integration: ## Run integration tests only
	@echo "Running integration tests..."
	go test -v -race ./test/...

.PHONY: test-coverage
test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	@mkdir -p $(COVERAGE_DIR)
	go test -v -race -coverprofile=$(COVERAGE_DIR)/coverage.out ./...
	go tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	@echo "Coverage report generated: $(COVERAGE_DIR)/coverage.html"

.PHONY: test-coverage-view
test-coverage-view: test-coverage ## Run tests with coverage and open report in browser
	@echo "Opening coverage report..."
	open $(COVERAGE_DIR)/coverage.html

.PHONY: benchmark
benchmark: ## Run benchmarks
	@echo "Running benchmarks..."
	go test -v -bench=. -benchmem ./...

##@ Code Quality
.PHONY: fmt
fmt: ## Format Go code
	@echo "Formatting code..."
	go fmt ./...

.PHONY: lint
lint: ## Run linter using pre-commit
	@echo "Executing pre-commit for all files"
	@if command -v pre-commit >/dev/null 2>&1; then \
		pre-commit run --all-files; \
	else \
		echo "pre-commit not installed. Install with: pip install pre-commit"; \
	fi
	@echo "pre-commit executed."

.PHONY: vet
vet: ## Run go vet
	@echo "Running go vet..."
	go vet ./...

.PHONY: sec
sec: ## Run security scanner
	@echo "Running security scanner..."
	@if command -v gosec >/dev/null 2>&1; then \
		gosec ./...; \
	else \
		echo "gosec not installed. Install with: go install github.com/securego/gosec/v2/cmd/gosec@latest"; \
	fi

.PHONY: check-aep
check-aep: ## Run AEP Spectral linting on OpenAPI specs
	@echo "Running AEP Spectral linter on OpenAPI specs..."
	@if command -v npx >/dev/null 2>&1; then \
		npx --yes @stoplight/spectral-cli lint api/openapi.yaml api/namespace-openapi.yaml --ruleset .spectral.yaml; \
	else \
		echo "npx not found. Install Node.js 20+ to run Spectral."; \
		exit 1; \
	fi

.PHONY: check
check: fmt vet lint lint-api sec test ## Run all code quality checks

##@ Documentation
.PHONY: docs
docs: ## Generate API documentation
	@echo "Generating API documentation..."
	@if command -v swagger >/dev/null 2>&1; then \
		swagger generate spec -o ./api/swagger.json --scan-models; \
	else \
		echo "swagger not installed. Install with: go install github.com/go-swagger/go-swagger/cmd/swagger@latest"; \
	fi

.PHONY: docs-serve
docs-serve: ## Serve API documentation
	@echo "Serving API documentation..."
	@if command -v swagger >/dev/null 2>&1; then \
		swagger serve -F=swagger ./api/openapi.yaml; \
	else \
		echo "swagger not installed. Install with: go install github.com/go-swagger/go-swagger/cmd/swagger@latest"; \
	fi

##@ Container
.PHONY: image-build
image-build: ## Build container image
	@echo "Building container image..."
	@podman build -t $(IMAGE_NAME):$(IMAGE_TAG) .
	@echo "Container image built: $(IMAGE_NAME):$(IMAGE_TAG)"

.PHONY: image-run
image-run: ## Run container
	@echo "Running container..."
	@echo "Using kubeconfig: $(if $(KUBECONFIG),$(KUBECONFIG),~/.kube/config)"
	@podman run -d --rm --name $(CONTAINER_NAME) -p 8082:8080 -p 8083:8081 \
		$(if $(DCM_NETWORK),--network $(DCM_NETWORK),) \
		-v "$(if $(KUBECONFIG),$(KUBECONFIG),~/.kube/config)":/home/appuser/.kube/config:ro \
		$(IMAGE_NAME):$(IMAGE_TAG)

.PHONY: image-stop
image-stop: ## Stop container
	@echo "Stopping container..."
	@podman stop $(CONTAINER_NAME)


##@ Release
.PHONY: release
release: check build-all ## Prepare a release
	@echo "Preparing release $(VERSION)..."
	@mkdir -p $(DIST_DIR)
	@cp $(BUILD_DIR)/* $(DIST_DIR)/
	@echo "Release $(VERSION) prepared in $(DIST_DIR)/"

.PHONY: tag
tag: ## Create and push a git tag
	@echo "Creating git tag $(VERSION)..."
	git tag -a $(VERSION) -m "Release $(VERSION)"
	git push origin $(VERSION)

##@ Maintenance
.PHONY: clean
clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	rm -rf $(DIST_DIR)
	rm -rf $(COVERAGE_DIR)
	go clean -cache

.PHONY: install-tools
install-tools: ## Install development tools
	@echo "Installing development tools..."
	pip install pre-commit
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	go install github.com/go-swagger/go-swagger/cmd/swagger@latest
	@echo "Note: Node.js 20+ required for AEP linting (npx @stoplight/spectral-cli)"

.PHONY: info
info: ## Show build information
	@echo "Build Information"
	@echo "================="
	@echo "App Name:     $(APP_NAME)"
	@echo "Version:      $(VERSION)"
	@echo "Build Time:   $(BUILD_TIME)"
	@echo "Git Commit:   $(GIT_COMMIT)"
	@echo "Go Version:   $(shell go version)"
	@echo "GOOS:         $(GOOS)"
	@echo "GOARCH:       $(GOARCH)"
	@echo "CGO Enabled:  $(CGO_ENABLED)"

##@ Environment
.PHONY: env
env: ## Show environment variables
	@echo "Environment Variables"
	@echo "===================="
	@echo "DCM_NETWORK:     $(DCM_NETWORK)"
	@echo "KUBECONFIG:      $(KUBECONFIG)"
	@echo "GO111MODULE:     $(GO111MODULE)"
	@echo "GOPROXY:         $(GOPROXY)"

# Include local Makefile for custom targets if it exists
-include Makefile.local
