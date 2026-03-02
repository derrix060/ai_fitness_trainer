# Stage 1: Build intervals-mcp Go binary
FROM golang:1.26-bookworm AS go-builder

RUN go install github.com/derrix060/intervals-mcp@latest


# Stage 2: Python runtime with Node.js and Claude CLI
FROM python:3.14-slim-bookworm

# Install Node.js 24 LTS (needed for google-calendar MCP via npx)
RUN apt-get update && \
    apt-get install -y --no-install-recommends curl ca-certificates && \
    curl -fsSL https://deb.nodesource.com/setup_24.x | bash - && \
    apt-get install -y --no-install-recommends nodejs && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Install Claude CLI (native installer, npm method is deprecated)
RUN curl -fsSL https://claude.ai/install.sh | bash && \
    cp /root/.local/share/claude/versions/* /usr/local/bin/claude

# Install uv
COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv

# Copy intervals-mcp binary from Go build stage
COPY --from=go-builder /go/bin/intervals-mcp /app/intervals-mcp

# Set up app directory
WORKDIR /app

# Copy application source and install Python dependencies
COPY pyproject.toml ./
COPY src/ ./src/
COPY CLAUDE.md .mcp.json ./
RUN uv pip install --system .

# Create non-root user
RUN useradd --create-home --shell /bin/bash appuser && \
    mkdir -p /app/data /app/config && \
    chown -R appuser:appuser /app

USER appuser

CMD ["python", "-m", "src.main"]
