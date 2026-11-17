# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.2] - 2026-11-17

### Added
- Retry on every method: an upstream that times out, drops the connection, answers a non-2xx status, returns invalid JSON or a rate-limit/internal JSON-RPC error is retried on another upstream. Tunable with `RPC_MAX_ATTEMPTS` (default 3), `RPC_RETRY_BACKOFF_MS`, `UPSTREAM_TIMEOUT_SECONDS` and `REQUEST_TIMEOUT_SECONDS`.
- `LOG_LEVEL` (`debug`, `info`, `warn`, `error`) and `LOG_BODIES` / `LOG_BODY_MAX_BYTES` for detailed debug logs: upstream order, every attempt with status, duration, size, outcome and reason, and optionally the full request and response bodies. All log lines of a request share its `request_id`.
- The access log now includes `origin`, `user_agent` and `content_length`.

### Changed
- When every attempt fails the pool answers `502` (`504` when the request budget ran out) instead of `400`, with the `request_id` and attempt count in the JSON-RPC error `data`.
- Upstream URLs are logged redacted (`https://host/…#fingerprint`) because they often embed API keys.
- Offline upstreams (per the health check) are tried last.
- Failed or invalid upstream answers are no longer cached for `eth_chainId`, `net_version` and `eth_gasPrice`.
- Health checks treat a non-2xx upstream status as offline.

## [1.0.1] - 2026-11-16

### Added
- `ALLOWED_ORIGINS` environment variable to restrict CORS to specific origins. Unset or `*` keeps allowing every origin; a comma-separated list allows only those origins and rejects browser requests from any other origin with `403`. Requests without an `Origin` header are unaffected.

### Fixed
- CORS preflight (`OPTIONS`) requests are now answered with `204` and the proper `Access-Control-Allow-*` headers instead of falling through to the JSON-RPC handler, which broke browser `POST` requests with `Content-Type: application/json`.

## [1.0.0] - 2024-06-28

### Added
- Round-robin load balancing across multiple Ethereum JSON-RPC endpoints
- In-memory response caching for `eth_blockNumber`, `eth_getBlockByNumber`, `eth_chainId`, `net_version`, and `eth_gasPrice`
- Background health monitoring of all configured RPCs with configurable interval
- `/status` endpoint exposing per-RPC health metrics (online status, block number, response time)
- Retry logic for `eth_getBlockByNumber` with null results (up to 3 providers)
- Docker multi-stage build producing a minimal Alpine-based image
- GitHub Actions CI workflow for Docker build validation on PRs
- Configurable via environment variables: `RPC_LIST`, `PORT`, `BLOCK_NUMBER_FETCH_INTERVAL_SECONDS`
- `.env` file support via godotenv
- CORS headers on all responses

[1.0.2]: https://github.com/itxtoledo/ethereum-rpc-pool/releases/tag/v1.0.2
[1.0.1]: https://github.com/itxtoledo/ethereum-rpc-pool/releases/tag/v1.0.1
[1.0.0]: https://github.com/itxtoledo/ethereum-rpc-pool/releases/tag/v1.0.0
