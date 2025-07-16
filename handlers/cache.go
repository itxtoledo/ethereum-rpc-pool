package handlers

import (
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
}

func GetRPCStatus() map[string]*RPCStatus {
	rpcStatusMutex.RLock()
	defer rpcStatusMutex.RUnlock()

	// Return a copy to avoid race conditions
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
	BlockData map[string]interface{}
	Timestamp time.Time
}

var blockCache BlockCache

func GetBlockCache() *BlockCache {
	blockCache.mu.RLock()
	defer blockCache.mu.RUnlock()
	return &blockCache
}

func SetBlockCache(blockData map[string]interface{}) {
	blockCache.mu.Lock()
	defer blockCache.mu.Unlock()
	blockCache.BlockData = blockData
	blockCache.Timestamp = time.Now()
}