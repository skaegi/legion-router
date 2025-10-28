# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /build

# Copy go mod files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary - static build, no CGO
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -a -installsuffix cgo -ldflags '-w -s -extldflags "-static"' -o legion-router .

# Runtime stage - use Alpine for smaller size and simpler package management
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache \
    iptables \
    ip6tables \
    nftables \
    iproute2 \
    ca-certificates \
    conntrack-tools

# Create config directory
RUN mkdir -p /etc/legion-router

# Copy binary from builder
COPY --from=builder /build/legion-router /usr/local/bin/legion-router

# Copy default config
COPY examples/config.yaml /etc/legion-router/config.yaml

# Enable IP forwarding by default
RUN echo "net.ipv4.ip_forward=1" >> /etc/sysctl.conf

# Set capabilities needed for networking
# Note: These need to be set when running the container
# docker run --cap-add=NET_ADMIN --cap-add=NET_RAW ...

ENTRYPOINT ["/usr/local/bin/legion-router"]
CMD ["-config", "/etc/legion-router/config.yaml"]
