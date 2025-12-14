# NixFleet Dashboard v2 - Multi-stage Go Build
#
# Build: docker build --build-arg VERSION=2.0.0 --build-arg GIT_COMMIT=$(git rev-parse HEAD) -t nixfleet:v2 .
# Run:   docker run -p 8000:8000 -v nixfleet_data:/data nixfleet:v2
#
# This uses modernc.org/sqlite (pure Go, no CGO) so we can use scratch/alpine

# ============================================
# Stage 1: Build
# ============================================
FROM golang:1.24-alpine AS builder

# Install build dependencies (git for go mod, ca-certs for HTTPS)
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Copy go mod files first (better caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build arguments for version injection
ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_TIME=unknown

# Generate templ templates if needed (they should be pre-generated, but just in case)
# RUN go install github.com/a-h/templ/cmd/templ@latest && templ generate

# Build the dashboard binary
# CGO_ENABLED=0 for static binary (modernc.org/sqlite is pure Go)
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w \
      -X github.com/markus-barta/nixfleet/v2/internal/dashboard.Version=${VERSION} \
      -X github.com/markus-barta/nixfleet/v2/internal/dashboard.GitCommit=${GIT_COMMIT} \
      -X github.com/markus-barta/nixfleet/v2/internal/dashboard.BuildTime=${BUILD_TIME}" \
    -o /nixfleet-dashboard \
    ./cmd/nixfleet-dashboard

# ============================================
# Stage 2: Runtime
# ============================================
FROM alpine:3.20

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    && rm -rf /var/cache/apk/*

# Create non-root user
RUN addgroup -g 1000 nixfleet && \
    adduser -u 1000 -G nixfleet -s /bin/sh -D nixfleet

# Create data directory
RUN mkdir -p /data && chown nixfleet:nixfleet /data

# Copy binary from builder
COPY --from=builder /nixfleet-dashboard /usr/local/bin/nixfleet-dashboard

# Switch to non-root user
USER nixfleet

# Expose internal port (Traefik connects here, NOT exposed to host)
EXPOSE 8000

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget -q --spider http://localhost:8000/login || exit 1

# Set default environment
ENV NIXFLEET_LISTEN=":8000" \
    NIXFLEET_DATA_DIR="/data"

# Run the dashboard
CMD ["nixfleet-dashboard"]

