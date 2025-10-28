# Legion Router

A high-performance egress filtering Docker container that acts as a network gateway for application containers. Traffic is routed through Legion Router by modifying the application container's default route, allowing fine-grained control over outbound connections.

**Target Platform: Linux Docker containers only.** This project uses Linux-specific kernel features (nftables) and is designed to run exclusively in containerized Linux environments.

## Features

- **Network-layer filtering**: Protocol, IP, and port-based rules using nftables
- **Domain-based filtering**: DNS-aware filtering with automatic IP resolution and caching
- **Priority-based rules**: Control rule evaluation order with explicit priorities
- **High performance**: Written in Go with efficient nftables integration
- **Dynamic updates**: Automatic DNS refresh to track changing IP addresses
- **CIDR support**: Filter entire network ranges
- **Wildcard domains**: Support for `*.example.com` patterns

## Architecture

```
┌─────────────────────┐
│  App Container      │
│  (route modified)   │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│  Legion Router      │
│  ┌───────────────┐  │
│  │ DNS Resolver  │  │
│  └───────────────┘  │
│  ┌───────────────┐  │
│  │   nftables    │  │
│  └───────────────┘  │
└──────────┬──────────┘
           │
           ▼
      Internet
```

## Quick Start

### Building the Docker Image

```bash
docker build -t legion-router .
```

### Running Legion Router

```bash
# With YAML config
docker run -d \
  --name legion-router \
  --cap-add=NET_ADMIN \
  --cap-add=NET_RAW \
  -v /path/to/config.yaml:/etc/legion-router/config.yaml \
  legion-router

# With JSON config
docker run -d \
  --name legion-router \
  --cap-add=NET_ADMIN \
  --cap-add=NET_RAW \
  -v /path/to/config.json:/etc/legion-router/config.json \
  legion-router -config /etc/legion-router/config.json
```

### Configuring App Container

Modify your application container's default route to send traffic through Legion Router:

```bash
# Get Legion Router's IP address
LEGION_IP=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' legion-router)

# In your app container, replace default route
docker exec app-container ip route del default
docker exec app-container ip route add default via $LEGION_IP
```

## Configuration

Configuration can be defined in either YAML or JSON format.

**Example configurations:**
- [examples/config.yaml](examples/config.yaml) - Default configuration (development environment)
- [examples/config.json](examples/config.json) - JSON format example
- [examples/test-config.yaml](examples/test-config.yaml) - Minimal test configuration

The file format is automatically detected based on the file extension (`.yaml`, `.yml`, `.json`).

### Schema (YAML)

```yaml
version: string

rules:
  - name: string              # Unique rule name
    action: allow|deny        # Action to take
    order: integer            # Priority (lower = higher priority)

    egress:                   # Optional - if omitted, matches all traffic
      protocols:              # Optional - tcp, udp, icmp
        - tcp

      domains:                # Optional - supports wildcards (*.example.com)
        - api.github.com

      ips:                    # Optional - supports CIDR notation
        - 10.0.0.0/8
        - 169.254.169.254

      ports:                  # Optional - single ports or ranges
        - "443"
        - "8000-9000"
```

### Schema (JSON)

```json
{
  "version": "string",
  "rules": [
    {
      "name": "string",
      "action": "allow|deny",
      "order": 100,
      "egress": {
        "protocols": ["tcp", "udp", "icmp"],
        "domains": ["api.github.com"],
        "ips": ["10.0.0.0/8", "169.254.169.254"],
        "ports": ["443", "8000-9000"]
      }
    }
  ]
}
```

### Rule Matching Logic

- Rules are evaluated in order of priority (`order` field, lower numbers first)
- Within a rule's `egress` section, criteria are ANDed together
- If a field is omitted, it matches all values for that field
- First matching rule determines the action (allow or deny)
- **Default policy**: If no rules match, traffic is **DROPPED**

### Example Rules

#### Block Cloud Metadata Service

```yaml
- name: block-metadata-service
  action: deny
  order: 50
  egress:
    ips: ["169.254.169.254"]
```

#### Allow DNS Queries

```yaml
- name: allow-dns
  action: allow
  order: 100
  egress:
    protocols: [udp, tcp]
    ips: ["8.8.8.8", "1.1.1.1"]
    ports: ["53"]
```

#### Allow HTTPS to Specific Domains

```yaml
- name: allow-api-access
  action: allow
  order: 100
  egress:
    protocols: [tcp]
    domains: ["api.github.com", "api.stripe.com"]
    ports: ["443"]
```

#### Allow Internal Network

```yaml
- name: allow-internal
  action: allow
  order: 200
  egress:
    protocols: [tcp, udp]
    ips: ["10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"]
```

## Docker Compose Example

```yaml
version: '3.8'

services:
  legion-router:
    build: .
    container_name: legion-router
    cap_add:
      - NET_ADMIN
      - NET_RAW
    volumes:
      - ./config.yaml:/etc/legion-router/config.yaml
    networks:
      - app-network

  app:
    image: your-app:latest
    depends_on:
      - legion-router
    networks:
      - app-network
    # Route modification would happen in entrypoint script

networks:
  app-network:
    driver: bridge
```

## Kubernetes Example

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: app-with-egress-filter
spec:
  containers:
  - name: app
    image: your-app:latest

  - name: legion-router
    image: legion-router:latest
    securityContext:
      capabilities:
        add:
        - NET_ADMIN
        - NET_RAW
    volumeMounts:
    - name: config
      mountPath: /etc/legion-router

  volumes:
  - name: config
    configMap:
      name: legion-router-config
```

## Hot Reload

Legion Router automatically watches the configuration file for changes and reloads rules without requiring a container restart. When the config file is modified:

1. The new configuration is loaded and validated
2. Existing nftables rules are cleared
3. New rules are applied based on the updated configuration
4. If validation fails, the error is logged and the old rules remain active

To trigger a reload, edit and save the configuration file:

```bash
# Edit config
vi /path/to/config.yaml

# Changes are automatically detected and applied
# Check docker logs to see reload status:
docker logs legion-router
```

## Monitoring and Logging

### Viewing Active Connections

To see what connections are being allowed through the router:

```bash
# View all active connections
docker exec legion-router conntrack -L

# Monitor new connections in real-time
docker exec legion-router conntrack -E
```

### Viewing nftables Rules

To see the actual nftables rules that are applied:

```bash
# View all rules in the legion_filter table
docker exec legion-router nft list table legion_filter

# View just the forward chain rules
docker exec legion-router nft list chain legion_filter egress_filter
```

### Debugging Blocked Connections

If a connection is being blocked and you're not sure why:

1. Check the nftables rules to see what's configured:
   ```bash
   docker exec legion-router nft list ruleset
   ```

2. Test connectivity from the app container:
   ```bash
   # Try to connect
   docker exec app-container curl -v https://example.com

   # Check if DNS is working
   docker exec app-container dig example.com
   ```

3. Review the application logs:
   ```bash
   docker logs legion-router
   ```

4. Verify the routing is correct:
   ```bash
   # In app container
   docker exec app-container ip route
   # Should show default route via legion router IP
   ```

## Performance

Legion Router operates at the kernel level using nftables, providing:
- **Latency:** ~5-50 microseconds per packet
- **Throughput:** Line-rate (no meaningful impact)
- **Memory:** ~20-30 MB total footprint
- **CPU:** <5% for typical workloads

See [PERFORMANCE.md](PERFORMANCE.md) for detailed analysis and benchmarking.

## Requirements

- Linux container runtime (Docker, Podman, etc.)
- CAP_NET_ADMIN and CAP_NET_RAW capabilities
- Linux kernel with nftables support (kernel 3.13+)

## Security Considerations

- Requires privileged network capabilities (NET_ADMIN, NET_RAW)
- Default policy is DENY - explicitly allow required traffic only
- Use trusted DNS servers (8.8.8.8, 1.1.1.1 by default)
- Review and test filtering rules before production use

## License

Apache 2.0
