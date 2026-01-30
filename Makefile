# StrataSave Makefile

.PHONY: build build-linux run test clean dev seed-admin tidy css css-watch css-prod setup setup-tailwind

# Build variables
BINARY_NAME=stratasave
BUILD_DIR=bin
CMD_PATH=./cmd/stratasave

# Build the application
build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_PATH)

# Build for Linux 386 (production server)
build-linux:
	GOOS=linux GOARCH=386 go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-386 $(CMD_PATH)

# Run the application
run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

# Run in development mode (with live reload if air is installed)
dev:
	@if command -v air > /dev/null; then \
		air; \
	else \
		echo "air not installed, using go run"; \
		go run ./cmd/stratasave; \
	fi

# Run tests
test:
	go test -v ./...

# Run tests with coverage
test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

# Tidy dependencies
tidy:
	go mod tidy

# Seed admin user (requires EMAIL env var)
seed-admin:
	@if [ -z "$(EMAIL)" ]; then \
		echo "Usage: make seed-admin EMAIL=admin@example.com"; \
		exit 1; \
	fi
	./bin/stratasave seed-admin --email=$(EMAIL)

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed"; \
		go vet ./...; \
	fi

# Generate (if needed)
generate:
	go generate ./...

# Build for production
build-prod:
	CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o bin/stratasave ./cmd/stratasave

# Docker build (if Dockerfile exists)
docker-build:
	docker build -t stratasave:latest .

# Tailwind CSS variables
CSS_INPUT  = ./internal/app/resources/assets/css/src/input.css
CSS_OUTPUT = ./internal/app/resources/assets/css/tailwind.css

# Build Tailwind CSS
css:
	./tailwindcss -i $(CSS_INPUT) -o $(CSS_OUTPUT)

# Watch Tailwind CSS for changes (development)
css-watch:
	./tailwindcss -i $(CSS_INPUT) -o $(CSS_OUTPUT) --watch

# Build minified Tailwind CSS (production)
css-prod:
	./tailwindcss -i $(CSS_INPUT) -o $(CSS_OUTPUT) --minify

# Download Tailwind CSS standalone CLI for current platform
setup-tailwind:
	@echo "Detecting platform..."
	@OS=$$(uname -s | tr '[:upper:]' '[:lower:]'); \
	ARCH=$$(uname -m); \
	if [ "$$OS" = "darwin" ]; then \
		if [ "$$ARCH" = "arm64" ]; then \
			URL="https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-macos-arm64"; \
		else \
			URL="https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-macos-x64"; \
		fi; \
	elif [ "$$OS" = "linux" ]; then \
		if [ "$$ARCH" = "aarch64" ] || [ "$$ARCH" = "arm64" ]; then \
			URL="https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-arm64"; \
		else \
			URL="https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-x64"; \
		fi; \
	else \
		echo "Unsupported platform: $$OS/$$ARCH"; exit 1; \
	fi; \
	echo "Downloading Tailwind CSS from $$URL..."; \
	curl -sL "$$URL" -o tailwindcss && chmod +x tailwindcss
	@echo "Tailwind CSS installed successfully"

# Set up development environment (run after git clone)
setup: setup-tailwind
	@echo "Development environment setup complete"
	@echo ""
	@echo "Next steps:"
	@echo "  1. Copy config.example.toml to config.toml"
	@echo "  2. Update config.toml with your settings"
	@echo "  3. Run 'make css' to build Tailwind CSS"
	@echo "  4. Run 'make dev' to start the development server"

# Show help
help:
	@echo "StrataSave Makefile targets:"
	@echo ""
	@echo "Build & Run:"
	@echo "  build       - Build the application"
	@echo "  build-linux - Build for Linux 386 (production server)"
	@echo "  run         - Build and run the application"
	@echo "  dev         - Run in development mode"
	@echo ""
	@echo "Testing:"
	@echo "  test        - Run tests"
	@echo "  test-cover  - Run tests with coverage"
	@echo "  fmt         - Format code"
	@echo "  lint        - Lint code"
	@echo ""
	@echo "CSS:"
	@echo "  css         - Build Tailwind CSS"
	@echo "  css-watch   - Watch and rebuild Tailwind CSS"
	@echo "  css-prod    - Build minified Tailwind CSS"
	@echo ""
	@echo "Setup:"
	@echo "  setup           - Set up development environment"
	@echo "  setup-tailwind  - Download Tailwind CSS standalone CLI"
	@echo ""
	@echo "Other:"
	@echo "  clean       - Clean build artifacts"
	@echo "  tidy        - Tidy dependencies"
	@echo "  seed-admin  - Seed admin user (EMAIL=... required)"
	@echo "  build-prod  - Build for production"
	@echo "  help        - Show this help"
