# ============================================================
# Multi-stage build for Go app on Fargate (linux/amd64)
# ============================================================

# --- Build stage ---
    FROM --platform=linux/amd64 golang:1.24-alpine AS builder

    WORKDIR /app
    
    # Cache dependencies first
    COPY go.mod go.sum ./
    RUN go mod download
    
    # Build the binary
    COPY . .
    RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server .
    
    # --- Runtime stage ---
    FROM --platform=linux/amd64 alpine:3.19
    
    RUN apk --no-cache add ca-certificates
    
    WORKDIR /app
    COPY --from=builder /app/server .
    
    EXPOSE 8080
    
    CMD ["./server"]