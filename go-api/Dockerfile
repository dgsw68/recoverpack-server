# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install git and ca-certificates
RUN apk add --no-cache git ca-certificates

COPY go.mod ./
COPY . .
RUN go mod tidy

# Build the Go application binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o server cmd/server/main.go

# Production runner stage
FROM alpine:latest

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata font-noto-cjk

COPY --from=builder /app/server .
COPY --from=builder /app/data ./data

# Expose Go API port
EXPOSE 8080

ENV PORT=8080
ENV GIN_MODE=release
ENV RECOVERPACK_PDF_FONT=/usr/share/fonts/noto-cjk/NotoSansCJK-Regular.ttc

CMD ["./server"]
