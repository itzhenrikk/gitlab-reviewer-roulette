# Build stage
FROM golang:1.25.4-alpine3.22 AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Build arguments
ARG VERSION=dev

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the applications
RUN go build -ldflags="-s -w -X main.version=${VERSION}" -o /build/bin/server ./cmd/server
RUN go build -ldflags="-s -w -X main.version=${VERSION}" -o /build/bin/migrate ./cmd/migrate
RUN go build -ldflags="-s -w -X main.version=${VERSION}" -o /build/bin/init ./cmd/init

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create app user
RUN addgroup -g 1000 app && \
  adduser -D -u 1000 -G app app

# Set working directory
WORKDIR /app

# Copy binaries from builder
COPY --from=builder /build/bin/server /app/server
COPY --from=builder /build/bin/migrate /app/migrate
COPY --from=builder /build/bin/init /app/init

# Copy migrations
COPY --from=builder /build/migrations /app/migrations

# Create directory for config
RUN mkdir -p /app/config

# Change ownership
RUN chown -R app:app /app

# Switch to app user
USER app

# Expose ports
EXPOSE 8080 9090

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Set entrypoint to allow running different binaries
# This allows: docker run image /app/migrate up
# Or: docker run image /app/init --config /app/config.yaml
ENTRYPOINT []

# Default command runs the server
CMD ["/app/server"]
