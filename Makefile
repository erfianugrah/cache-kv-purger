# Project variables
BINARY_NAME := cache-kv-purger
SHORT_NAME := cf
MAIN_PACKAGE := ./cmd/cache-kv-purger
BUILD_DIR := ./build
DIST_DIR := ./dist
GO := go
GOFLAGS := -v
LDFLAGS := -s -w

# Version information
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Full ldflags
FULL_LDFLAGS := $(LDFLAGS) -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(BUILD_DATE)

# Go environment
export CGO_ENABLED=0

# Platform detection
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)

# Default target
.DEFAULT_GOAL := build

# Help target
.PHONY: help
help:
	@echo "Makefile for $(BINARY_NAME)"
	@echo ""
	@echo "Usage:"
	@echo "  make build       Build the binary for current platform"
	@echo "  make build-all   Build binaries for all platforms"
	@echo "  make install     Build and install the binary as 'cf'"
	@echo "  make test        Run tests"
	@echo "  make lint        Run linter"
	@echo "  make clean       Clean build artifacts"
	@echo "  make release     Create a new release with goreleaser"
	@echo "  make snapshot    Create a snapshot release with goreleaser"
	@echo ""
	@echo "Build options:"
	@echo "  make build-linux   Build for Linux (amd64)"
	@echo "  make build-darwin  Build for macOS (amd64)"
	@echo "  make build-windows Build for Windows (amd64)"

# Build for current platform
.PHONY: build
build:
	@echo "Building $(BINARY_NAME) for $(GOOS)/$(GOARCH)..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -ldflags "$(FULL_LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE)
	@echo "Binary built: $(BUILD_DIR)/$(BINARY_NAME)"

# Install binary as 'cf'
.PHONY: install
install: build
	@echo "Installing $(BINARY_NAME) as $(SHORT_NAME)..."
	@sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/$(SHORT_NAME)
	@sudo chmod +x /usr/local/bin/$(SHORT_NAME)
	@echo "Installed successfully! You can now use '$(SHORT_NAME)' command."

# Cross-platform builds
.PHONY: build-linux
build-linux:
	@echo "Building for Linux amd64..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags "$(FULL_LDFLAGS)" \
		-o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PACKAGE)

.PHONY: build-darwin
build-darwin:
	@echo "Building for Darwin (macOS) amd64..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags "$(FULL_LDFLAGS)" \
		-o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PACKAGE)

.PHONY: build-windows
build-windows:
	@echo "Building for Windows amd64..."
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags "$(FULL_LDFLAGS)" \
		-o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PACKAGE)

# Build all platforms
.PHONY: build-all
build-all: build-linux build-darwin build-windows
	@echo "All binaries built in $(BUILD_DIR)/"

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	$(GO) test -v ./...

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GO) test -v -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run linter
.PHONY: lint
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed. Installing..."; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin v1.55.2; \
		$$(go env GOPATH)/bin/golangci-lint run ./...; \
	fi

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...

# Tidy dependencies
.PHONY: tidy
tidy:
	@echo "Tidying dependencies..."
	$(GO) mod tidy

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR) $(DIST_DIR)
	@rm -f coverage.out coverage.html
	@echo "Clean complete."

# Create a release with goreleaser
.PHONY: release
release:
	@echo "Creating release with goreleaser..."
	@if command -v goreleaser >/dev/null 2>&1; then \
		goreleaser release --clean; \
	else \
		echo "goreleaser not installed. Please install it first."; \
		echo "Visit: https://goreleaser.com/install/"; \
		exit 1; \
	fi

# Create a snapshot release with goreleaser
.PHONY: snapshot
snapshot:
	@echo "Creating snapshot release with goreleaser..."
	@if command -v goreleaser >/dev/null 2>&1; then \
		goreleaser release --snapshot --clean; \
	else \
		echo "goreleaser not installed. Please install it first."; \
		echo "Visit: https://goreleaser.com/install/"; \
		exit 1; \
	fi

# Development build (quick build without optimization)
.PHONY: dev
dev:
	@echo "Building development version..."
	$(GO) build -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE)

# Verify dependencies
.PHONY: verify
verify:
	@echo "Verifying dependencies..."
	$(GO) mod verify

# Download dependencies
.PHONY: deps
deps:
	@echo "Downloading dependencies..."
	$(GO) mod download

# Run the application
.PHONY: run
run: build
	@echo "Running $(BINARY_NAME)..."
	$(BUILD_DIR)/$(BINARY_NAME)

# Check for outdated dependencies
.PHONY: check-deps
check-deps:
	@echo "Checking for outdated dependencies..."
	@go list -u -m all

# CI target - runs all checks
.PHONY: ci
ci: deps verify fmt lint test build
	@echo "CI checks completed successfully!"