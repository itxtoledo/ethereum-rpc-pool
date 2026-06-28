package handlers

import (
	"strconv"
	"sync"
	"time"
)

type RPCStatus struct {
	BlockNumber  string    `json:"blockNumber"`
	ResponseTime int64     `json:"responseTime"`
	Timestamp    time.Time `json:"timestamp"`
	Online       bool      `json:"online"`
}

var rpcStatusCache = make(map[string]*RPCStatus)
var rpcStatusMutex = &sync.RWMutex{}

func SetRPCStatus(rpcURL string, blockNumber string, responseTime int64, online bool) {
	rpcStatusMutex.Lock()
	defer rpcStatusMutex.Unlock()

	status, ok := rpcStatusCache[rpcURL]
	if !ok {
		status = &RPCStatus{}
		rpcStatusCache[rpcURL] = status
	}

	if blockNumber != "" {
		status.BlockNumber = blockNumber
	}
	status.ResponseTime = responseTime
	status.Timestamp = time.Now()
	status.Online = online

	onlineVal := 0.0
	if online {
		onlineVal = 1.0
	}
	RPCOffline.WithLabelValues(rpcURL).Set(onlineVal)
	RPCResponseTime.WithLabelValues(rpcURL).Set(float64(responseTime))

	if blockNumber != "" {
		if bn, err := strconv.ParseInt(blockNumber[2:], 16, 64); err == nil {
			RPCBlockNumber.WithLabelValues(rpcURL).Set(float64(bn))
		}
	}
}

func GetRPCStatus() map[string]*RPCStatus {
	rpcStatusMutex.RLock()
	defer rpcStatusMutex.RUnlock()

	clone := make(map[string]*RPCStatus)
	for k, v := range rpcStatusCache {
		clone[k] = v
	}
	return clone
}

func InitializeRPCStatus(rpcs []string) {
	rpcStatusMutex.Lock()
	defer rpcStatusMutex.Unlock()
	for _, rpc := range rpcs {
		rpcStatusCache[rpc] = &RPCStatus{Online: false}
	}
}

type BlockCache struct {
	mu        sync.RWMutex
	BlockData map[string]any
	Timestamp time.Time
}

var blockCache BlockCache

func GetBlockCache() *BlockCache {
	blockCache.mu.RLock()
	defer blockCache.mu.RUnlock()
	return &blockCache
}

func SetBlockCache(blockData map[string]any) {
	blockCache.mu.Lock()
	defer blockCache.mu.Unlock()
	blockCache.BlockData = blockData
	blockCache.Timestamp = time.Now()
}

type methodCacheEntry struct {
	value     []byte
	timestamp time.Time
	ttl       time.Duration
}

var methodCacheStore = make(map[string]*methodCacheEntry)
var methodCacheMutex = &sync.RWMutex{}

func SetMethodCache(key string, value []byte, ttl time.Duration) {
	methodCacheMutex.Lock()
	defer methodCacheMutex.Unlock()
	methodCacheStore[key] = &methodCacheEntry{
		value:     value,
		timestamp: time.Now(),
		ttl:       ttl,
	}
}

func GetMethodCache(key string) ([]byte, bool) {
	methodCacheMutex.RLock()
	defer methodCacheMutex.RUnlock()
	entry, ok := methodCacheStore[key]
	if !ok {
		return nil, false
	}
	if entry.ttl > 0 && time.Since(entry.timestamp) > entry.ttl {
		return nil, false
	}
	return entry.value, true
}
