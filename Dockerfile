# ─────────────────────────────────────────────────────────
# Stage 1 — Builder
# ─────────────────────────────────────────────────────────
FROM golang:1.21-alpine AS builder

# Install git for go mod download (private modules / VCS stamps)
RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Cache dependency downloads before copying source
COPY go.mod ./
RUN go mod download

# Copy source
COPY . .

# Build a statically linked binary with debug info stripped
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
      -ldflags="-w -s" \
      -o /url-checker \
      ./cmd/server

# ─────────────────────────────────────────────────────────
# Stage 2 — Final minimal image
# ─────────────────────────────────────────────────────────
FROM alpine:3.19

# Security: run as non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Trust CA certs so HTTPS calls work
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the compiled binary
COPY --from=builder /url-checker /url-checker

# Ensure binary is owned by appuser
RUN chown appuser:appgroup /url-checker

USER appuser

EXPOSE 8080

ENTRYPOINT ["/url-checker"]
