# Build stage
FROM golang:1.23-alpine AS builder

# Build arguments
ARG ENV=dev

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the applications
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server cmd/server/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o consumer cmd/consumer/main.go

# Server stage
FROM alpine:latest AS server

# Install ca-certificates for HTTPS calls
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1001 -S appuser && \
    adduser -u 1001 -S appuser -G appuser

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/server .

# Copy all config files
COPY --from=builder /app/config/ ./config/

# Change ownership to non-root user
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/v1/health || exit 1

# Set environment from build arg
ARG ENV=dev
ENV ENV=$ENV

# Run the application
CMD ["./server"]

# Consumer stage
FROM alpine:latest AS consumer

# Install ca-certificates for HTTPS calls
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1001 -S appuser && \
    adduser -u 1001 -S appuser -G appuser

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/consumer .

# Copy all config files
COPY --from=builder /app/config/ ./config/

# Change ownership to non-root user
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Set environment from build arg
ARG ENV=dev
ENV ENV=$ENV

# Run the consumer
CMD ["./consumer"]