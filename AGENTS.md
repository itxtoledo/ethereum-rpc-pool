# AGENTS.md

## Project Overview

Go-based proxy server that load-balances Ethereum JSON-RPC requests across multiple upstream endpoints. Implements round-robin routing, in-memory response caching for `eth_blockNumber` and `eth_getBlockByNumber`, and background health monitoring of all configured RPCs.

Go 1.19, single external dependency (`godotenv`). No framework — raw `net/http`.

## Commands

```bash
# Run locally (loads .env if present)
go run .

# Build binary
go build -o app .

# Docker build (CI does this too)
docker build -t go-rpc-proxy .
```

There are **no tests** and **no linter** configured in this project. CI (`.github/workflows/on_pr.yaml`) only validates the Docker build.

## Architecture & Data Flow

```
Client POST /  ──►  handlers.RPCHandler
                       │
                       ├─ method is eth_blockNumber or eth_getBlockByNumber?
                       │     YES → try cache → hit? return cached
                       │     NO  → fall through
                       │
                       └─ utils.GetNextRPC(rpcs)  ← round-robin (atomic counter)
                              │
                              ▼
                         proxyRequest() → upstream RPC → return response to client

GET /status  ──►  handlers.StatusHandler → GetRPCStatus()
```

**Background loop** (`FetchBlockNumber` goroutine, started in `main.go`):
- Ticks every `BLOCK_NUMBER_FETCH_INTERVAL_SECONDS` (default 10s)
- Fires `eth_blockNumber` to **all** configured RPCs concurrently
- On success, also fetches the full block (`eth_getBlockByNumber`) and caches it
- Updates per-RPC status (online/offline, block number, response time) in `rpcStatusCache`

## Environment Variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `RPC_LIST` | **Yes** | — | Comma-separated RPC URLs |
| `PORT` | No | `8080` | HTTP listen port |
| `BLOCK_NUMBER_FETCH_INTERVAL_SECONDS` | No | `10` | Health check interval |

**Critical**: `RPC_LIST` values are split by comma with **no whitespace trimming**. If a value has leading/trailing spaces, those become part of the URL and will cause connection failures.

## Key Files

| File | Purpose |
|---|---|
| `main.go` | Entry point: env loading, route setup, starts background fetcher |
| `handlers/rpc.go` | Core handler logic, caching decisions, proxying |
| `handlers/cache.go` | Thread-safe in-memory caches (`RPCStatus`, `BlockCache`) |
| `handlers/error.go` | JSON-RPC error response helper |
| `utils/round_robin.go` | Atomic round-robin index selection |

## Code Patterns & Conventions

- **No external router/mux** — uses `http.HandleFunc` with `nil` mux
- **CORS**: All responses set `Access-Control-Allow-Origin: *`
- **X-Powered-By header** on every response
- **Mutex pattern**: `sync.RWMutex` for status/block caches; `sync/atomic.Uint32` for round-robin counter
- **Error format**: Follows JSON-RPC 2.0 error spec (`{"jsonrpc":"2.0","error":{"code":...,"message":...},"id":...}`)
- **HTTP client**: Inline `&http.Client{Timeout: 10 * time.Second}` per request (not pooled)

## Gotchas

- **RPC_LIST whitespace**: Comma-split is `strings.Split(rpcList, ",")` — no trimming. Extra whitespace in URLs will break.
- **Round-robin counter wraps**: `uint32` wraps silently at 4,294,967,295. With many requests over time, the modulo will still work correctly but the index resets to 0 on overflow.
- **Cache returned unsafely**: `GetBlockCache()` returns a pointer while only holding `RLock`. Callers should not mutate the `BlockData` map. The returned `Timestamp` is safe to read.
- **`.env` file optional**: Server starts with system env vars if `.env` is missing. A warning is logged.
- **Cache freshness logic**: `eth_getBlockByNumber` cache is considered stale if older than `BLOCK_NUMBER_FETCH_INTERVAL_SECONDS` **or** if the cached block number doesn't match the latest block seen by health checks.
- **No request pooling**: Every upstream RPC call creates a new `http.Client`. Under high load, consider connection reuse.
- **Background goroutines are fire-and-forget**: No error recovery or restart if health check goroutines panic.

## Testing

No test files exist. Standard Go test conventions apply if adding tests: `go test ./...`, test files alongside source in `handlers/` and `utils/`.

## Docker

Multi-stage build: `golang:1.19` (builder) → `alpine:latest` (runtime with `ca-certificates`). Binary is copied from builder, not a scratch image.
