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

# Stage 2: Minimal runtime (no Python, no Node.js)
FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates curl && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Install Claude CLI (standalone binary, no Node.js needed)
RUN curl -fsSL https://claude.ai/install.sh | bash && \
    cp /root/.local/share/claude/versions/* /usr/local/bin/claude

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
