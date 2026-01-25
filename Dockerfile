# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache curl ca-certificates tzdata

WORKDIR /app

# Download Tailwind CSS standalone CLI
RUN ARCH=$(uname -m) && \
    if [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then \
        URL="https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-arm64"; \
    else \
        URL="https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-x64"; \
    fi && \
    curl -sL "$URL" -o /usr/local/bin/tailwindcss && \
    chmod +x /usr/local/bin/tailwindcss

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build Tailwind CSS
RUN tailwindcss -i ./internal/app/resources/assets/css/src/input.css \
    -o ./internal/app/resources/assets/css/tailwind.css --minify

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/stratasave ./cmd/stratasave

# Final stage
FROM alpine:3.21

# Install ca-certificates for HTTPS and tzdata for timezone support
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user for security
RUN adduser -D -g '' appuser

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/stratasave .

# Copy resources (templates, assets)
COPY --from=builder /app/internal/app/resources ./internal/app/resources

# Change ownership to non-root user
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose the default port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
ENTRYPOINT ["./stratasave"]
