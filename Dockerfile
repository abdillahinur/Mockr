# Multi-stage Docker build for Mockr
# Stage 1: Build the Go binary
FROM golang:alpine AS builder

# Install git and ca-certificates (needed for go modules)
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o mockr ./cmd/mockr

# Stage 2: Create minimal runtime image
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Create non-root user for security
RUN adduser -D -H -s /bin/sh mockr

# Create app directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/mockr /usr/local/bin/mockr

# Make binary executable and change ownership
RUN chmod +x /usr/local/bin/mockr && \
    chown mockr:mockr /usr/local/bin/mockr

# Copy default config file and change ownership
COPY examples/mockr.json /app/mockr.json
RUN chown mockr:mockr /app/mockr.json

# Change ownership of app directory
RUN chown -R mockr:mockr /app

# Switch to non-root user
USER mockr

# Expose default port
EXPOSE 3000

# Default command
CMD ["/usr/local/bin/mockr", "start", "/app/mockr.json"]

# Usage with volume mount:
# docker run -p 3000:3000 -v /path/to/your/config.json:/app/mockr.json mockr
#
# For enhanced security, consider running with:
# docker run --read-only --tmpfs /tmp -p 3000:3000 -v /path/to/your/config.json:/app/mockr.json mockr