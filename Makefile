.PHONY: build clean test lint install

# Build both binaries
build:
	@echo "Building tusk CLI..."
	go build -o tusk ./cmd/tusk
	@echo "Building tuskd daemon..."
	go build -o tuskd ./cmd/tuskd
	@echo "Build complete!"

# Clean build artifacts
clean:
	rm -f tusk tuskd
	rm -f coverage.out coverage.html

# Run tests
test:
	go test -v -cover ./...

# Run tests with coverage
test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Lint code
lint:
	go fmt ./...
	go vet ./...

# Install to $GOPATH/bin
install: build
	go install ./cmd/tusk
	go install ./cmd/tuskd

# Format code
fmt:
	go fmt ./...

# Tidy dependencies
tidy:
	go mod tidy

# Run all checks
check: fmt lint test

# Help
help:
	@echo "Available targets:"
	@echo "  build      - Build both binaries"
	@echo "  clean      - Remove build artifacts"
	@echo "  test       - Run tests"
	@echo "  test-cover - Run tests with coverage report"
	@echo "  lint       - Run go vet and format"
	@echo "  install    - Build and install to PATH"
	@echo "  fmt        - Format code"
	@echo "  tidy       - Tidy dependencies"
	@echo "  check      - Run fmt, lint, and test"
	@echo "  help       - Show this help"