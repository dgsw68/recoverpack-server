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

# The go-pdf/fpdf TTF parser only understands TrueType (glyf) outlines. apk's
# font-noto-cjk ships a TrueType Collection (.ttc) of CFF/OpenType (OTTO)
# fonts, which fpdf can't read at all (neither the container format nor the
# CFF outlines are supported) - AddUTF8FontFromBytes fails silently and the
# PDF only errors much later at render time with "undefined font". Pull out
# the Korean-tuned sub-font and convert its CFF outlines to genuine TrueType
# glyf outlines (cu2qu, via otf2ttf) so fpdf can embed it.
RUN apk add --no-cache --virtual .font-build py3-fonttools py3-pip \
	&& pip install --no-cache-dir --break-system-packages otf2ttf \
	&& python3 -c "from fontTools.ttLib import TTCollection; TTCollection('/usr/share/fonts/noto/NotoSansCJK-Regular.ttc').fonts[1].save('/tmp/NotoSansCJKkr.otf')" \
	&& otf2ttf /tmp/NotoSansCJKkr.otf -o /usr/share/fonts/noto/NotoSansCJK-Regular.ttf \
	&& rm -f /tmp/NotoSansCJKkr.otf \
	&& apk del .font-build

COPY --from=builder /app/server .
COPY --from=builder /app/data ./data

# Expose Go API port
EXPOSE 8080

ENV PORT=8080
ENV GIN_MODE=release
ENV RECOVERPACK_PDF_FONT=/usr/share/fonts/noto/NotoSansCJK-Regular.ttf

CMD ["./server"]
