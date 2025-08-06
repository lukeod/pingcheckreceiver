# Ping Receiver

## Getting Started

By default, the Ping receiver will ping each configured target and report round-trip time and packet loss metrics:

```yaml
receivers:
  ping:
    targets:
      - endpoint: google.com
      - endpoint: 8.8.8.8
```

## Configuration

The following settings are available:

- `collection_interval` (default: `60s`): How often to ping targets
- `initial_delay` (default: `1s`): Time to wait before first collection
- `privileged` (default: `false`): Whether to use raw ICMP sockets (requires privileges)
- `targets`: List of endpoints to ping
  - `endpoint`: Hostname or IP address to ping (required)
  - `count` (default: `4`): Number of packets to send
  - `timeout` (default: `5s`): Timeout for the ping operation
  - `interval` (default: `1s`): Interval between packets

### Example Configuration

```yaml
receivers:
  ping:
    collection_interval: 60s
    initial_delay: 1s
    privileged: false
    targets:
      - endpoint: google.com
        count: 4
        timeout: 5s
        interval: 1s
      - endpoint: 8.8.8.8
        count: 3
        timeout: 3s
      - endpoint: internal.service.local
        count: 5
        timeout: 10s
        interval: 2s
```

## Privilege Requirements

ICMP operations may require elevated privileges depending on the platform:

### Linux/Unix

For raw ICMP sockets (privileged mode):
```bash
# Grant CAP_NET_RAW capability
sudo setcap cap_net_raw+ep /path/to/collector
```

### Windows

Windows always requires privileged mode for ICMP operations. The receiver automatically enables it on Windows platforms.

### Kubernetes

```yaml
securityContext:
  capabilities:
    add:
    - NET_RAW
```

### Unprivileged Mode

When `privileged: false` (Linux/Unix only), the receiver uses UDP sockets which work without special privileges.

## Metrics

The following metrics are emitted by this receiver:

| Metric | Description | Unit | Type | Attributes |
|--------|-------------|------|------|------------|
| `ping.duration` | Round-trip time for individual ping packets | ms | Gauge | net.peer.name, net.peer.ip |
| `ping.duration.min` | Minimum round-trip time | ms | Gauge | net.peer.name, net.peer.ip |
| `ping.duration.max` | Maximum round-trip time | ms | Gauge | net.peer.name, net.peer.ip |
| `ping.duration.avg` | Average round-trip time | ms | Gauge | net.peer.name, net.peer.ip |
| `ping.duration.stddev` | Standard deviation of round-trip times | ms | Gauge | net.peer.name, net.peer.ip |
| `ping.packet_loss` | Ratio of packets lost (0.0 to 1.0) | 1 | Gauge | net.peer.name, net.peer.ip |
| `ping.packets.sent` | Total number of packets sent | {packet} | Sum | net.peer.name, net.peer.ip |
| `ping.packets.received` | Total number of packets received | {packet} | Sum | net.peer.name, net.peer.ip |
| `ping.errors` | Number of errors encountered (disabled by default) | {error} | Sum | net.peer.name, net.peer.ip, error.type |

### Attributes

- `net.peer.name`: The hostname or endpoint as configured
- `net.peer.ip`: The resolved IP address of the target
- `error.type`: Type of error (when applicable): `timeout`, `dns_failure`, `network_unreachable`, `permission_denied`, `unknown`

## Example Pipeline

```yaml
receivers:
  ping:
    collection_interval: 60s
    targets:
      - endpoint: google.com
      - endpoint: cloudflare.com
      - endpoint: 8.8.8.8

processors:
  batch:
    timeout: 10s

exporters:
  prometheus:
    endpoint: "0.0.0.0:8889"
  debug:
    verbosity: detailed

service:
  pipelines:
    metrics:
      receivers: [ping]
      processors: [batch]
      exporters: [prometheus, debug]
```
