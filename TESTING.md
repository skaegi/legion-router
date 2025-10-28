# Testing Legion Router

## Running Tests

All tests run in a Linux Docker container to ensure they execute in the target environment:

```bash
make test
```

This builds a Docker image and runs all unit and integration tests inside it.

## Manual Testing

### 1. Start Legion Router

```bash
# Create network
docker network create legion-test-net

# Start router
docker run -d \
    --name legion-router \
    --network legion-test-net \
    -v $(pwd)/examples:/etc/legion-router \
    legion-router
```

### 2. Start Test App

```bash
# Start app container
docker run -d \
    --name test-app \
    --network legion-test-net \
    alpine:latest \
    sleep 3600

# Install tools
docker exec test-app apk add curl bind-tools

# Get router IP and configure routing
ROUTER_IP=$(docker inspect legion-router | jq -r '.[0].networks[0].address' | cut -d'/' -f1)
docker exec test-app ip route del default
docker exec test-app ip route add default via $ROUTER_IP
```

### 3. Test Connectivity

```bash
# Test DNS (should work if allowed)
docker exec test-app dig google.com @8.8.8.8

# Test HTTPS (should work if allowed)
docker exec test-app curl -v https://google.com

# Test blocked traffic (should timeout)
docker exec test-app curl -v --max-time 5 https://github.com
```

### 4. Inspect Rules

```bash
# View nftables rules
docker exec legion-router nft list ruleset

# View active connections
docker exec legion-router conntrack -L

# View logs
docker logs legion-router
```

### 5. Clean Up

```bash
docker stop test-app legion-router
docker rm test-app legion-router
docker network rm legion-test-net
```

## Debugging

### Check if traffic is being filtered

```bash
# Verify routing in app container
docker exec test-app ip route
# Should show: default via <ROUTER_IP>

# Check nftables rules
docker exec legion-router nft list table legion_filter
```

### DNS not working

- Ensure DNS servers (8.8.8.8, 1.1.1.1) are allowed in config
- Check both UDP and TCP port 53 are allowed
- Test DNS from router: `docker exec legion-router dig google.com`

### Domain rules not working

- Check Legion Router logs for DNS resolution errors
- Verify domain names are correct
- Domains must resolve to IPs; wildcard patterns (*.example.com) are not enforced at runtime

## Test Configuration

Use `examples/test-config.yaml` for a minimal test setup that allows:
- DNS to 8.8.8.8 and 1.1.1.1
- HTTPS to google.com
- Everything else blocked
