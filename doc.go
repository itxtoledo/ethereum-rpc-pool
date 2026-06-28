// Package main provides ethereum-rpc-pool, a Go-based proxy server that
// load-balances Ethereum JSON-RPC requests across multiple upstream endpoints.
//
// Features:
//   - Round-robin load balancing across configured RPC endpoints
//   - In-memory response caching for common methods (eth_blockNumber,
//     eth_getBlockByNumber, eth_chainId, net_version, eth_gasPrice)
//   - Background health monitoring of all RPCs with configurable interval
//   - Status endpoint exposing per-RPC health metrics
//   - Prometheus metrics for request counts, latency, cache hit rates
//   - Connection pooling with keep-alive
//   - Graceful shutdown on SIGTERM/SIGINT
//   - Structured JSON logging
//
// Quick start:
//
//	export RPC_LIST="https://mainnet.infura.io/v3/YOUR_KEY,https://eth-mainnet.alchemyapi.io/v2/YOUR_KEY"
//	go run .
//
// Then send JSON-RPC requests to http://localhost:8080/.
package main
