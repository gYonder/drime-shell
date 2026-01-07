.PHONY: all build test test-race test-cover lint fmt check clean install release-dry release

# Default target
all: fmt lint test build

# Build the binary
build:
	@echo "Building drime-shell..."
	go build -o drime-shell ./cmd/drime

# Run all tests
test:
	@echo "Running tests..."
	go test ./... -v

# Run tests with race detector
test-race:
	@echo "Running tests with race detector..."
	go test ./... -race -v

# Run tests with coverage
test-cover:
	@echo "Running tests with coverage..."
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Run golangci-lint (includes vet and more)
lint:
	@echo "Running linter..."
	golangci-lint run

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Check (fmt + lint + test) - for CI
check: fmt lint test

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -f drime-shell
	rm -f coverage.out coverage.html

# Install to GOPATH/bin
install:
	@echo "Installing drime-shell..."
	go install ./cmd/drime

# Release (dry run) - requires goreleaser
release-dry:
	@echo "Running goreleaser (dry run)..."
	goreleaser release --snapshot --clean

# Release (real) - requires GITHUB_TOKEN
release:
	@echo "Running goreleaser..."
	goreleaser release --clean


# Run the application
run: build
	./drime

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

# Install development tools
tools:
	@echo "Installing development tools..."
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $$(go env GOPATH)/bin v2.7.2

# Help
help:
	@echo "Available targets:"
	@echo "  all        - fmt, lint, test, build (default)"
	@echo "  build      - Build the binary"
	@echo "  test       - Run tests"
	@echo "  test-race  - Run tests with race detector"
	@echo "  test-cover - Run tests with coverage report"
	@echo "  lint       - Run golangci-lint (includes vet)"
	@echo "  fmt        - Format code with go fmt"
	@echo "  check      - Run all checks (for CI)"
	@echo "  clean      - Remove build artifacts"
	@echo "  install    - Install to GOPATH/bin"
	@echo "  run        - Build and run"
	@echo "  deps       - Download and tidy dependencies"
	@echo "  tools      - Install development tools"
