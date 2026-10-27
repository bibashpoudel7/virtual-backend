# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /src

# Download dependencies first (cached unless go.mod/go.sum change)
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Static binaries: the API and the SQL migration runner
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
 && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

# Runtime stage
FROM alpine:3.20

# CA certificates for SMTP/R2 TLS, tzdata for time zones; wget (busybox) is used by the healthcheck
RUN apk add --no-cache ca-certificates tzdata \
 && addgroup -S app && adduser -S -G app app

WORKDIR /app
COPY --from=builder /out/api /out/migrate ./
COPY migrations ./migrations

USER app

ENV PORT=5555 \
    GIN_MODE=release
EXPOSE 5555

# Configuration comes from environment variables at runtime (never baked into the image)
CMD ["./api"]
