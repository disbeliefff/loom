FROM golang:1.25-bookworm AS builder

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends git && \
    rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o loom ./cmd/loom

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    git \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /workspace

COPY --from=builder /app/loom /usr/local/bin/loom

CMD ["/bin/bash"]
