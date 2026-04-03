# ── Build stage ──────────────────────────────────────────────────────────────
FROM --platform=linux/amd64 golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY main.go ./

ENV GOARCH=amd64 GOOS=linux CGO_ENABLED=0

RUN go build -o loadtester .

# ── Runtime stage ─────────────────────────────────────────────────────────────
FROM --platform=linux/amd64 alpine:latest

WORKDIR /app

COPY --from=builder /app/loadtester .

ENTRYPOINT ["./loadtester"]
