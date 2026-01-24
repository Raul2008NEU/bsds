
# Build stage
FROM golang:1.23-alpine AS build

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY *.go ./

# Build the application
RUN go build -o /docker-go-app

# Run stage
FROM alpine:latest

WORKDIR /

# Copy the binary from build stage
COPY --from=build /docker-go-app /docker-go-app

# Expose port 8080
EXPOSE 8080

# Run the application
CMD ["/docker-go-app"]
