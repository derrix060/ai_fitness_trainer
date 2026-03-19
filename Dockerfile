# Stage 1: Build all Go binaries
FROM golang:1.26-bookworm AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o bot ./cmd/bot/
RUN CGO_ENABLED=0 GOOS=linux go build -o gcal-mcp ./cmd/gcal-mcp/

# Also build intervals-mcp
RUN go install github.com/derrix060/intervals-mcp@latest

# Stage 2: Runtime (Node.js needed only for Claude CLI npm package)
FROM node:24-bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates curl && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Install Claude CLI via npm (install.sh is blocked from Docker)
RUN npm install -g @anthropic-ai/claude-code

# Copy Go binaries
COPY --from=builder /build/bot /app/bot
COPY --from=builder /build/gcal-mcp /app/gcal-mcp
COPY --from=builder /go/bin/intervals-mcp /app/intervals-mcp

COPY CLAUDE.md .mcp.json /app/

WORKDIR /app

RUN useradd --create-home --shell /bin/bash appuser && \
    mkdir -p /app/data /app/config && \
    chown -R appuser:appuser /app

USER appuser

CMD ["/app/bot"]
