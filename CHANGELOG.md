# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

[1.0.1]: https://github.com/itxtoledo/ethereum-rpc-pool/releases/tag/v1.0.1
[1.0.0]: https://github.com/itxtoledo/ethereum-rpc-pool/releases/tag/v1.0.0
