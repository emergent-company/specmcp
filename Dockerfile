# Build stage
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags "-s -w -X main.Version=${VERSION}" \
    -o /specmcp ./cmd/specmcp/

# Runtime stage
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 1000 specmcp

COPY --from=builder /specmcp /usr/local/bin/specmcp

USER specmcp

# Default to HTTP transport mode for containerized deployment.
ENV SPECMCP_TRANSPORT=http
ENV SPECMCP_PORT=21452
ENV SPECMCP_HOST=0.0.0.0
ENV SPECMCP_LOG_LEVEL=info

EXPOSE 21452

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:21452/health || exit 1

ENTRYPOINT ["specmcp"]
