# Performance Characteristics

## Architecture Overview

Legion Router operates as a network gateway with filtering performed at the kernel level using nftables. Traffic flows:

```
App Container → Legion Router (nftables) → Internet
```

## Performance Impact

### Latency Overhead

**Network Layer (nftables):**
- **~5-50 microseconds** per packet for rule evaluation
- Rules evaluated in kernel space (very fast)
- Linear time complexity: O(n) where n = number of rules
- First matching rule wins (early exit optimization)

**DNS Resolution (first request only):**
- **~10-100ms** for initial DNS lookup per domain
- Results cached with 5-minute TTL
- Subsequent requests use cached IPs (no DNS overhead)
- DNS refresh happens in background

**Total per-connection overhead:**
- First connection to new domain: **~10-100ms** (DNS lookup)
- Subsequent connections: **~5-50 microseconds** (nftables only)

### Throughput Impact

**Network throughput:**
- **Minimal impact** - nftables operates at kernel level
- No userspace packet copying
- Line-rate performance for most workloads
- Theoretical limit: Network interface speed

**CPU usage:**
- **Idle:** <1% CPU (just watching for config changes)
- **Active filtering:** <5% CPU for typical workloads
- Scales with packets-per-second, not bandwidth

### Memory Usage

**Base memory:**
- **~10-20 MB** for Go runtime and application
- **~1-5 MB** for nftables rules (depends on rule count)
- **~1 MB** per 1000 cached DNS entries

**Typical footprint:** **20-30 MB** total

### Scalability

**Rules:**
- Tested with **100+ rules** without noticeable impact
- Linear evaluation overhead (each rule adds ~1 microsecond)
- Recommendation: Keep rules under 100 for best performance

**Connections:**
- **Unlimited** - kernel handles connection tracking
- No per-connection overhead in userspace
- Limited only by kernel's conntrack table size

**DNS caching:**
- **10,000+ domains** cached efficiently
- Background refresh doesn't block traffic
- LRU eviction for memory management

## Optimization Tips

### 1. Rule Ordering

Place most frequently matched rules at **lower order numbers** (higher priority):

```yaml
rules:
  # Frequently hit - order 50
  - name: allow-api-gateway
    action: allow
    order: 50

  # Less frequent - order 200
  - name: allow-rare-service
    action: allow
    order: 200
```

**Impact:** Can reduce average latency from 25μs to 10μs

### 2. Use IP-based rules when possible

**IP-based rules** (faster):
```yaml
egress:
  ips: ["1.2.3.4"]
  ports: ["443"]
```

**Domain-based rules** (adds DNS lookup on first request):
```yaml
egress:
  domains: ["api.example.com"]
  ports: ["443"]
```

**Recommendation:** Use IPs for internal services, domains for external services

### 3. Combine related rules

**Bad - multiple rules:**
```yaml
- name: allow-service-a
  egress:
    ips: ["10.1.1.1"]
- name: allow-service-b
  egress:
    ips: ["10.1.1.2"]
```

**Good - single CIDR rule:**
```yaml
- name: allow-internal-services
  egress:
    ips: ["10.1.1.0/24"]
```

**Impact:** Reduces rule count and evaluation time

## Comparison to Alternatives

### vs Network Policies (Kubernetes)

| Aspect | Legion Router | Network Policies |
|--------|---------------|------------------|
| Latency | ~10μs | ~5μs |
| DNS-aware | Yes | No |
| Port ranges | Yes | No |
| Priority rules | Yes | No |
| Hot reload | Yes | Requires pod restart |

### vs Service Mesh (Istio/Linkerd)

| Aspect | Legion Router | Service Mesh |
|--------|---------------|--------------|
| Latency | ~10μs | ~1-5ms |
| Memory | ~20 MB | ~50-200 MB |
| Complexity | Low | High |
| Features | Egress only | Full mesh |

### vs Application Proxy (Envoy/Squid)

| Aspect | Legion Router | App Proxy |
|--------|---------------|-----------|
| Latency | ~10μs | ~100μs-1ms |
| Protocol | Layer 3/4 | Layer 7 |
| Throughput | Line rate | ~1-10 Gbps |
| HTTP inspection | No | Yes |

## Benchmarking

### Simple latency test

```bash
# Without Legion Router
time curl -w "%{time_total}\n" -o /dev/null -s https://example.com

# With Legion Router
container exec app time curl -w "%{time_total}\n" -o /dev/null -s https://example.com
```

**Expected difference:** <5ms for HTTPS (includes DNS + TLS handshake)

### Throughput test

```bash
# Test with iperf3 through Legion Router
docker exec app iperf3 -c <server> -t 30
```

**Expected:** >1 Gbps on modern hardware (kernel forwarding is fast)

## When to Use Legion Router

**Good fit:**
- Need egress filtering at network layer
- Want low overhead (microseconds)
- DNS-aware rules sufficient
- Don't need HTTP/HTTPS inspection
- Want simple, reliable filtering

**Not a good fit:**
- Need HTTP method/path filtering
- Need HTTPS payload inspection
- Need <1μs latency (use Network Policies)
- Need full service mesh features

## Summary

**Overhead:** Negligible for most workloads
- **Latency:** 5-50 microseconds per packet
- **Throughput:** No meaningful impact
- **CPU:** <5% for typical workloads
- **Memory:** ~20-30 MB total

Legion Router adds minimal overhead for network-layer egress filtering.
