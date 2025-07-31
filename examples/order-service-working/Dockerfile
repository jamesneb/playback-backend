# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy source code
COPY . .

# Download/update dependencies and build
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o order-service .

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/order-service .

# Expose port
EXPOSE 8081

# Command to run
CMD ["./order-service"]