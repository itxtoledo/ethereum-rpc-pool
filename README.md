# Ethereum RPC Pool

Go-based proxy server that distributes Ethereum JSON-RPC requests across multiple upstream endpoints with round-robin load balancing, in-memory response caching, background health monitoring, and Prometheus metrics.

[![CI](https://github.com/itxtoledo/ethereum-rpc-pool/actions/workflows/ci.yaml/badge.svg)](https://github.com/itxtoledo/ethereum-rpc-pool/actions/workflows/ci.yaml)

## Features

- **Load Balancing** — Round-robin across configured RPC endpoints
- **Response Caching** — In-memory cache for `eth_blockNumber`, `eth_getBlockByNumber`, `eth_chainId`, `net_version`, `eth_gasPrice`
- **Health Monitoring** — Background goroutine checks every RPC's block number and response time
- **Graceful Shutdown** — Handles SIGTERM/SIGINT with draining connections
- **Prometheus Metrics** — `GET /metrics` exposes request counts, latency histograms, cache hit rates, per-RPC health
- **Structured Logging** — JSON logs to stderr via `log/slog`
- **Connection Pooling** — Shared `http.Client` with keep-alive and idle connection reuse (100 max, 20 per host)
- **Middleware Chain** — Request IDs, access logging, panic recovery, max body size (1MB)
- **Context Propagation** — Upstream requests respect caller context cancellation
- **Health & Status Endpoints** — `GET /healthz` for liveness probes, `GET /status` for per-RPC details
- **RPC List Trimming** — Whitespace around comma-separated URLs is stripped automatically
- **Retry Logic** — `eth_getBlockByNumber` retries up to 3 providers on null results
- **Multi-Arch Docker** — Pre-built images for `linux/amd64` and `linux/arm64` on [GHCR](https://github.com/itxtoledo/ethereum-rpc-pool/pkgs/container/ethereum-rpc-pool)
- **Minimal Image** — ~10MB distroless base, no shell, non-root user

## Available Images

Pre-built multi-arch images are published to [GitHub Container Registry](https://github.com/itxtoledo/ethereum-rpc-pool/pkgs/container/ethereum-rpc-pool) on every release. No build required.

| Tag | Description |
|---|---|
| `latest` | Most recent stable release |
| `v1.0.0` | Exact release version |
| `1.0` | Minor version (floating patches) |
| `1` | Major version (floating minors) |
| `sha-abc1234` | Specific commit |

**Architectures:** `linux/amd64`, `linux/arm64`

## Quick Start

### Run with Docker (recommended)

```sh
docker pull ghcr.io/itxtoledo/ethereum-rpc-pool:latest

docker run -d -p 8080:8080 \
  -e RPC_LIST="https://mainnet.infura.io/v3/YOUR_KEY,https://eth-mainnet.alchemyapi.io/v2/YOUR_KEY" \
  ghcr.io/itxtoledo/ethereum-rpc-pool:latest
```

### Docker Compose

```yaml
# docker-compose.yml
services:
  rpc-pool:
    image: ghcr.io/itxtoledo/ethereum-rpc-pool:latest
    ports:
      - "8080:8080"
    environment:
      RPC_LIST: "https://mainnet.infura.io/v3/YOUR_KEY,https://eth-mainnet.alchemyapi.io/v2/YOUR_KEY"
      BLOCK_NUMBER_FETCH_INTERVAL_SECONDS: "10"
```

### Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: rpc-pool
spec:
  replicas: 2
  selector:
    matchLabels:
      app: rpc-pool
  template:
    metadata:
      labels:
        app: rpc-pool
    spec:
      containers:
        - name: rpc-pool
          image: ghcr.io/itxtoledo/ethereum-rpc-pool:latest
          ports:
            - containerPort: 8080
          env:
            - name: RPC_LIST
              value: "https://mainnet.infura.io/v3/YOUR_KEY,https://eth-mainnet.alchemyapi.io/v2/YOUR_KEY"
          livenessProbe:
            httpGet:
              path: /healthz
              port: 8080
          readinessProbe:
            httpGet:
              path: /healthz
              port: 8080
          resources:
            limits:
              memory: "64Mi"
              cpu: "100m"
```

### Run locally (development)

```sh
git clone https://github.com/itxtoledo/ethereum-rpc-pool.git
cd ethereum-rpc-pool

cp .env.example .env
# Edit .env with your RPC_LIST

make dev
```

### Docker Compose (with local Anvil for development)

```sh
docker compose up
```

This starts the proxy + a local Anvil Ethereum node. Send requests to `http://localhost:8080/`.

## Usage Examples

```sh
# Latest block number
curl -X POST http://localhost:8080/ \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'

# Chain ID (cached)
curl -X POST http://localhost:8080/ \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}'

# Block by number
curl -X POST http://localhost:8080/ \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["0x1000",true],"id":1}'

# RPC health status
curl http://localhost:8080/status

# Kubernetes liveness probe
curl http://localhost:8080/healthz

# Prometheus metrics
curl http://localhost:8080/metrics
```

### Status Response

```json
{
  "https://mainnet.infura.io/v3/...": {
    "blockNumber": "0x1234567",
    "responseTime": 156,
    "timestamp": "2024-06-28T12:00:00Z",
    "online": true
  }
}
```

### Health Check Response

```json
{"status":"ok","healthy":true}
```

Returns `503` with `"status":"degraded"` when all RPCs are offline.

## Environment Variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `RPC_LIST` | **Yes** | — | Comma-separated RPC URLs (whitespace is trimmed) |
| `PORT` | No | `8080` | HTTP listen port |
| `BLOCK_NUMBER_FETCH_INTERVAL_SECONDS` | No | `10` | Health check interval |

Copy `.env.example` to `.env` and fill in your values.

## Development

**Requirements:** Go 1.23+, [Foundry](https://book.getfoundry.sh/) (for integration tests), [golangci-lint](https://golangci-lint.run/) (for linting)

```sh
# Run locally
make dev

# Build binary (optional — pre-built images available)
make build

# Run all tests
make test

# Unit tests only
make test-unit

# Integration tests (starts Anvil automatically)
make test-integration

# Lint
make lint
```

## Endpoints

| Path | Method | Description |
|---|---|---|
| `/` | POST | JSON-RPC proxy |
| `/` | GET | Health check (simple text) |
| `/status` | GET | Per-RPC health metrics JSON |
| `/healthz` | GET | Liveness probe (200/503) |
| `/metrics` | GET | Prometheus metrics |

## Prometheus Metrics

| Metric | Type | Labels | Description |
|---|---|---|---|
| `rpc_pool_requests_total` | Counter | method, status | Total RPC requests |
| `rpc_pool_request_duration_seconds` | Histogram | method | Request duration |
| `rpc_pool_upstream_duration_seconds` | Histogram | rpc_url | Upstream RPC latency |
| `rpc_pool_cache_hits_total` | Counter | method | Cache hits |
| `rpc_pool_cache_misses_total` | Counter | method | Cache misses |
| `rpc_pool_upstream_online` | Gauge | rpc_url | 1=online, 0=offline |
| `rpc_pool_upstream_block_number` | Gauge | rpc_url | Latest block number |
| `rpc_pool_upstream_response_time_ms` | Gauge | rpc_url | Last response time |

## Versioning

[Semantic Versioning](https://semver.org/). See [CHANGELOG.md](CHANGELOG.md) and [releases](https://github.com/itxtoledo/ethereum-rpc-pool/releases).

Docker images are tagged: `v1.0.0`, `1.0`, `1`, `latest`, and commit SHA.

## CI / CD

| Workflow | Trigger | What it does |
|---|---|---|
| **CI** | PRs and pushes to `main` | Unit tests, integration tests (Anvil), multi-arch Docker build |
| **Release** | Git tag `v*` | Builds and pushes multi-arch image to GHCR, creates GitHub Release |

## Project Structure

```
├── main.go                  # Entry point, graceful shutdown, middleware chain
├── doc.go                   # Package documentation
├── handlers/
│   ├── rpc.go               # Core proxy logic, caching, health checks
│   ├── cache.go             # Thread-safe in-memory caches
│   ├── error.go             # JSON-RPC error responses
│   ├── health.go            # /healthz handler
│   └── metrics.go           # Prometheus metric definitions
├── middleware/
│   └── middleware.go         # Recovery, request IDs, access log, max body
├── utils/
│   └── round_robin.go       # Atomic round-robin index
├── Dockerfile               # Multi-stage distroless build (pre-built images on GHCR)
├── docker-compose.yml       # Dev setup with Anvil
├── Makefile                 # Build, test, lint targets
├── .github/workflows/       # CI + Release workflows
├── .golangci.yml            # Linter configuration
├── VERSION                  # Current version
├── CHANGELOG.md             # Release history
└── .env.example             # Environment variable template
```

## License

MIT — see [LICENSE](LICENSE).
