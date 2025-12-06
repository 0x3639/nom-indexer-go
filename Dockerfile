# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install git and build dependencies for CGO (required for secp256k1)
RUN apk add --no-cache git gcc musl-dev

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Update go.mod for Go 1.24 and build the binary (CGO required for secp256k1)
RUN go mod tidy && CGO_ENABLED=1 GOOS=linux go build -o /app/indexer ./cmd/indexer

# Runtime stage
FROM alpine:3.19

WORKDIR /app

# Install ca-certificates for HTTPS connections
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user for security
RUN addgroup -g 1000 indexer && adduser -D -u 1000 -G indexer indexer

# Copy the binary from builder
COPY --from=builder /app/indexer /app/indexer

# Copy migrations
COPY --from=builder /app/migrations /app/migrations

# Set ownership to non-root user
RUN chown -R indexer:indexer /app

# Set environment variables
ENV MIGRATIONS_PATH=/app/migrations

# Switch to non-root user
USER indexer

# Run the indexer
CMD ["/app/indexer"]
