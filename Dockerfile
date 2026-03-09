FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install git for any CGO dependencies if needed (though go-git is pure go)
# RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build statically linked binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o loom ./cmd/loom

FROM alpine:3.21

# Install git just in case the CI environment expects it or if any submodules are cloned
RUN apk add --no-cache ca-certificates git bash

WORKDIR /workspace

COPY --from=builder /app/loom /usr/local/bin/loom

# By default, provide a shell so GitLab CI can run its script blocks
CMD ["/bin/bash"]
