# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd/api

# Final stage
FROM alpine:3.19

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata ffmpeg

# Copy the binary from builder
COPY --from=builder /app/main .

# Copy migrations if needed
COPY --from=builder /app/migrations ./migrations

# Copy .env file if exists
COPY --from=builder /app/.env ./.env

# Expose port
EXPOSE 5555

# Run the application
CMD ["./main"]
