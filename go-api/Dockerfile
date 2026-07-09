# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install git and ca-certificates
RUN apk add --no-cache git ca-certificates

COPY go.mod ./
RUN go mod download

COPY . .

# Build the Go application binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o server cmd/server/main.go

# Production runner stage
FROM alpine:latest

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/server .

# Expose Go API port
EXPOSE 8080

ENV PORT=8080
ENV GIN_MODE=release

CMD ["./server"]
