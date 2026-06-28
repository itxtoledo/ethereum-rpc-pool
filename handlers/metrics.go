package handlers

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	RequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rpc_pool_requests_total",
			Help: "Total number of RPC requests by method and status.",
		},
		[]string{"method", "status"},
	)
	RequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rpc_pool_request_duration_seconds",
			Help:    "Request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method"},
	)
	UpstreamDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rpc_pool_upstream_duration_seconds",
			Help:    "Upstream RPC request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"rpc_url"},
	)
	CacheHitsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rpc_pool_cache_hits_total",
			Help: "Total number of cache hits by method.",
		},
		[]string{"method"},
	)
	CacheMissesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rpc_pool_cache_misses_total",
			Help: "Total number of cache misses by method.",
		},
		[]string{"method"},
	)
	RPCOffline = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "rpc_pool_upstream_online",
			Help: "Whether an upstream RPC is online (1) or offline (0).",
		},
		[]string{"rpc_url"},
	)
	RPCBlockNumber = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "rpc_pool_upstream_block_number",
			Help: "Latest block number reported by upstream RPC.",
		},
		[]string{"rpc_url"},
	)
	RPCResponseTime = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "rpc_pool_upstream_response_time_ms",
			Help: "Last response time of upstream RPC in milliseconds.",
		},
		[]string{"rpc_url"},
	)
)
