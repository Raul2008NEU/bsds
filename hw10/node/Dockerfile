# ── Build stage ──────────────────────────────────────────────────────────────
FROM --platform=linux/amd64 golang:1.22-alpine AS builder

WORKDIR /app

# Copy module files first so the layer is cached when only source changes.
COPY go.mod ./

# Download deps (none beyond stdlib, but keeps the pattern consistent).
RUN go mod download

COPY main.go ./

# Cross-compile for linux/amd64 explicitly (required on Apple Silicon hosts).
ENV GOARCH=amd64 GOOS=linux CGO_ENABLED=0

RUN go build -o node .

# ── Runtime stage ─────────────────────────────────────────────────────────────
FROM --platform=linux/amd64 alpine:latest

WORKDIR /app

COPY --from=builder /app/node .

EXPOSE 8080

CMD ["./node"]
