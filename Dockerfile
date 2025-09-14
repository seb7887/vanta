# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary with optimizations
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown

RUN CGO_ENABLED=0 GOOS=linux go build \
    -a -installsuffix cgo \
    -ldflags="-w -s -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildTime=${BUILD_TIME}" \
    -o vanta ./cmd/mocker

# Final stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1001 -S vanta && \
    adduser -u 1001 -S vanta -G vanta

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/vanta .

# Copy configuration examples
COPY --from=builder /app/examples ./examples

# Change ownership
RUN chown -R vanta:vanta /app

# Switch to non-root user
USER vanta

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ./vanta start --config /app/examples/docker-config.yaml --health-check || exit 1

# Set entrypoint
ENTRYPOINT ["./vanta"]
CMD ["start", "--config", "/app/examples/docker-config.yaml"]