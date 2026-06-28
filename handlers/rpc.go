package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"ethereum-rpc-pool/middleware"
	"ethereum-rpc-pool/utils"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	rpcs   []string
	logger *slog.Logger
)

var httpClient = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	},
}

func SetLogger(l *slog.Logger) {
	logger = l
}

func Logger() *slog.Logger {
	if logger == nil {
		logger = slog.Default()
	}
	return logger
}

func SetRPCs(rpcList string) error {
	parts := strings.Split(rpcList, ",")
	rpcs = make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			rpcs = append(rpcs, trimmed)
		}
	}
	if len(rpcs) == 0 {
		return fmt.Errorf("RPC_LIST is empty after trimming")
	}
	Logger().Info("RPC endpoints configured", "count", len(rpcs))
	InitializeRPCStatus(rpcs)
	return nil
}

func FetchBlockNumber() {
	interval, err := strconv.Atoi(os.Getenv("BLOCK_NUMBER_FETCH_INTERVAL_SECONDS"))
	if err != nil {
		interval = 10
	}

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					Logger().Error("health check goroutine panicked", "error", r)
				}
			}()
			fetchFromAllRPCs()
		}()
	}
}

func fetchFromAllRPCs() {
	for _, rpc := range rpcs {
		go func(url string) {
			defer func() {
				if r := recover(); r != nil {
					Logger().Error("fetch goroutine panicked", "rpc", url, "error", r)
				}
			}()
			fetchBlockNumberFromRPC(url)
		}(rpc)
	}
}

func fetchBlockNumberFromRPC(rpcURL string) {
	start := time.Now()
	requestBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  "eth_blockNumber",
		"params":  []any{},
		"id":      1,
	})
	if err != nil {
		Logger().Error("failed to marshal eth_blockNumber request", "rpc", rpcURL, "error", err)
		SetRPCStatus(rpcURL, "", 0, false)
		return
	}

	respBody, err := makeRPCRequest(rpcURL, requestBody)
	responseTime := time.Since(start).Milliseconds()
	if err != nil {
		Logger().Warn("eth_blockNumber fetch failed", "rpc", rpcURL, "error", err)
		SetRPCStatus(rpcURL, "", responseTime, false)
		return
	}

	var jsonResponse map[string]any
	if err := json.Unmarshal(respBody, &jsonResponse); err != nil {
		Logger().Warn("failed to unmarshal eth_blockNumber response", "rpc", rpcURL, "error", err)
		SetRPCStatus(rpcURL, "", responseTime, false)
		return
	}

	if result, ok := jsonResponse["result"]; ok {
		if blockNumber, ok := result.(string); ok {
			SetRPCStatus(rpcURL, blockNumber, responseTime, true)
			go func(url, bn string) {
				defer func() {
					if r := recover(); r != nil {
						Logger().Error("block fetch goroutine panicked", "rpc", url, "error", r)
					}
				}()
				fetchBlock(url, bn)
			}(rpcURL, blockNumber)
		} else {
			Logger().Warn("invalid block number format", "rpc", rpcURL)
			SetRPCStatus(rpcURL, "", responseTime, false)
		}
	} else {
		SetRPCStatus(rpcURL, "", responseTime, false)
	}
}

func fetchBlock(rpcURL, blockNumber string) {
	requestBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  "eth_getBlockByNumber",
		"params":  []any{blockNumber, true},
		"id":      1,
	})
	if err != nil {
		Logger().Error("failed to marshal eth_getBlockByNumber request", "rpc", rpcURL, "error", err)
		return
	}

	respBody, err := makeRPCRequest(rpcURL, requestBody)
	if err != nil {
		Logger().Warn("block fetch failed", "rpc", rpcURL, "block", blockNumber, "error", err)
		return
	}

	var jsonResponse map[string]any
	if err := json.Unmarshal(respBody, &jsonResponse); err != nil {
		Logger().Warn("failed to unmarshal block response", "rpc", rpcURL, "error", err)
		return
	}

	if result, ok := jsonResponse["result"]; ok {
		if blockData, ok := result.(map[string]any); ok {
			SetBlockCache(blockData)
		}
	}
}

func RPCHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("running ethereum-rpc-pool by https://github.dev/itxtoledo/ethereum-rpc-pool"))
		return
	}
	if r.Method != http.MethodPost {
		SendError(w, -32601, "Method not allowed", nil)
		return
	}

	start := time.Now()

	reqBody, err := io.ReadAll(r.Body)
	if err != nil {
		SendError(w, -32603, "Error reading request body", nil)
		return
	}
	defer r.Body.Close()

	var jsonRequest map[string]any
	if err := json.Unmarshal(reqBody, &jsonRequest); err != nil {
		SendError(w, -32603, "Invalid JSON request body", nil)
		return
	}

	var method string
	method, _ = jsonRequest["method"].(string)
	defer func() {
		RequestDuration.WithLabelValues(method).Observe(time.Since(start).Seconds())
	}()

	switch method {
	case "eth_blockNumber":
		if handleBlockNumber(w, jsonRequest) {
			CacheHitsTotal.WithLabelValues(method).Inc()
			RequestsTotal.WithLabelValues(method, "cache_hit").Inc()
			return
		}
		CacheMissesTotal.WithLabelValues(method).Inc()
	case "eth_getBlockByNumber":
		if handleGetBlockByNumber(w, jsonRequest) {
			CacheHitsTotal.WithLabelValues(method).Inc()
			RequestsTotal.WithLabelValues(method, "cache_hit").Inc()
			return
		}
		CacheMissesTotal.WithLabelValues(method).Inc()
		proxyRequestWithRetry(w, r, reqBody, jsonRequest["id"])
		return
	case "eth_chainId":
		handleMethodCache(w, r, jsonRequest, reqBody, "eth_chainId", 0)
		return
	case "net_version":
		handleMethodCache(w, r, jsonRequest, reqBody, "net_version", 0)
		return
	case "eth_gasPrice":
		handleMethodCache(w, r, jsonRequest, reqBody, "eth_gasPrice", 3*time.Second)
		return
	}

	targetRPC := utils.GetNextRPC(rpcs)
	Logger().Debug("proxying request", "rpc", targetRPC, "method", method)
	proxyRequest(w, r, targetRPC, reqBody, jsonRequest["id"])
	RequestsTotal.WithLabelValues(method, "proxied").Inc()
}

func StatusHandler(w http.ResponseWriter, r *http.Request) {
	sendJSONResponse(w, GetRPCStatus(), http.StatusOK)
}

func handleBlockNumber(w http.ResponseWriter, jsonRequest map[string]any) bool {
	statuses := GetRPCStatus()
	var latestBlockNumber string
	var latestBlockNum int64

	for _, status := range statuses {
		if !status.Online || status.BlockNumber == "" {
			continue
		}
		if bn, err := parseHexBlock(status.BlockNumber); err == nil && bn > latestBlockNum {
			latestBlockNum = bn
			latestBlockNumber = status.BlockNumber
		} else if latestBlockNumber == "" {
			latestBlockNumber = status.BlockNumber
		}
	}

	if latestBlockNumber != "" {
		response := map[string]any{
			"jsonrpc": "2.0",
			"id":      jsonRequest["id"],
			"result":  latestBlockNumber,
		}
		sendJSONResponse(w, response, http.StatusOK)
		return true
	}
	return false
}

func handleGetBlockByNumber(w http.ResponseWriter, jsonRequest map[string]any) bool {
	params, ok := jsonRequest["params"].([]any)
	if !ok || len(params) == 0 {
		return false
	}

	requestedBlock, ok := params[0].(string)
	if !ok {
		return false
	}

	cache := GetBlockCache()
	if cache.BlockData == nil {
		return false
	}

	interval, err := strconv.Atoi(os.Getenv("BLOCK_NUMBER_FETCH_INTERVAL_SECONDS"))
	if err != nil {
		interval = 10
	}
	if time.Since(cache.Timestamp) > time.Duration(interval)*time.Second {
		return false
	}

	cachedBlockNumber, ok := cache.BlockData["number"].(string)
	if !ok {
		return false
	}

	if requestedBlock == "latest" {
		statuses := GetRPCStatus()
		var latestBlockNumber string
		var latestBlockNum int64
		for _, status := range statuses {
			if !status.Online || status.BlockNumber == "" {
				continue
			}
			if bn, err := parseHexBlock(status.BlockNumber); err == nil && bn > latestBlockNum {
				latestBlockNum = bn
				latestBlockNumber = status.BlockNumber
			} else if latestBlockNumber == "" {
				latestBlockNumber = status.BlockNumber
			}
		}
		if latestBlockNumber != "" && latestBlockNumber != cachedBlockNumber {
			return false
		}
	} else if requestedBlock != cachedBlockNumber {
		return false
	}

	response := map[string]any{
		"jsonrpc": "2.0",
		"id":      jsonRequest["id"],
		"result":  cache.BlockData,
	}
	sendJSONResponse(w, response, http.StatusOK)
	return true
}

func proxyRequest(w http.ResponseWriter, r *http.Request, targetRPC string, reqBody []byte, id any) {
	start := time.Now()
	respBody, err := makeRPCRequestContext(r.Context(), targetRPC, reqBody)
	UpstreamDuration.WithLabelValues(targetRPC).Observe(time.Since(start).Seconds())

	if err != nil {
		Logger().Warn("upstream request failed",
			"rpc", targetRPC,
			"error", err,
			"request_id", middleware.GetRequestID(r),
		)
		SendError(w, -32603, "Error making request to RPC", id)
		return
	}

	writeJSONResponse(w, http.StatusOK, respBody)
}

func handleMethodCache(w http.ResponseWriter, r *http.Request, jsonRequest map[string]any, reqBody []byte, method string, ttl time.Duration) {
	if cached, ok := GetMethodCache(method); ok {
		CacheHitsTotal.WithLabelValues(method).Inc()
		RequestsTotal.WithLabelValues(method, "cache_hit").Inc()
		writeJSONResponse(w, http.StatusOK, cached)
		return
	}

	CacheMissesTotal.WithLabelValues(method).Inc()
	targetRPC := utils.GetNextRPC(rpcs)
	Logger().Debug("proxying method", "rpc", targetRPC, "method", method)

	start := time.Now()
	respBody, err := makeRPCRequestContext(r.Context(), targetRPC, reqBody)
	UpstreamDuration.WithLabelValues(targetRPC).Observe(time.Since(start).Seconds())

	if err != nil {
		Logger().Warn("upstream method request failed", "rpc", targetRPC, "method", method, "error", err)
		SendError(w, -32603, "Error making request to RPC", jsonRequest["id"])
		return
	}

	RequestsTotal.WithLabelValues(method, "proxied").Inc()
	SetMethodCache(method, respBody, ttl)
	writeJSONResponse(w, http.StatusOK, respBody)
}

func proxyRequestWithRetry(w http.ResponseWriter, r *http.Request, reqBody []byte, id any) {
	maxAttempts := len(rpcs)
	if maxAttempts > 3 {
		maxAttempts = 3
	}

	var lastRespBody []byte

	for i := 0; i < maxAttempts; i++ {
		targetRPC := utils.GetNextRPC(rpcs)
		Logger().Debug("retry attempt", "rpc", targetRPC, "attempt", i+1, "max", maxAttempts)

		start := time.Now()
		respBody, err := makeRPCRequestContext(r.Context(), targetRPC, reqBody)
		UpstreamDuration.WithLabelValues(targetRPC).Observe(time.Since(start).Seconds())

		if err != nil {
			Logger().Warn("retry attempt failed", "rpc", targetRPC, "attempt", i+1, "error", err)
			continue
		}

		if isNullResult(respBody) {
			lastRespBody = respBody
			continue
		}

		RequestsTotal.WithLabelValues("eth_getBlockByNumber", "proxied").Inc()
		writeJSONResponse(w, http.StatusOK, respBody)
		return
	}

	if lastRespBody != nil {
		RequestsTotal.WithLabelValues("eth_getBlockByNumber", "proxied").Inc()
		writeJSONResponse(w, http.StatusOK, lastRespBody)
		return
	}

	SendError(w, -32603, "All RPC providers returned errors", id)
}

func isNullResult(respBody []byte) bool {
	var resp map[string]any
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return false
	}
	return resp["result"] == nil
}

func makeRPCRequest(rpcURL string, requestBody []byte) ([]byte, error) {
	return makeRPCRequestContext(context.Background(), rpcURL, requestBody)
}

func makeRPCRequestContext(ctx context.Context, rpcURL string, requestBody []byte) ([]byte, error) {
	proxyReq, err := http.NewRequestWithContext(ctx, "POST", rpcURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("error creating proxy request: %w", err)
	}
	proxyReq.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(proxyReq)
	if err != nil {
		return nil, fmt.Errorf("error making request to RPC: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading RPC response: %w", err)
	}
	return respBody, nil
}

func sendJSONResponse(w http.ResponseWriter, data any, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("X-Powered-By", "https://github.dev/itxtoledo/ethereum-rpc-pool")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		Logger().Error("failed to encode JSON response", "error", err)
	}
}

func parseHexBlock(hex string) (int64, error) {
	if len(hex) < 3 || hex[:2] != "0x" {
		return 0, fmt.Errorf("invalid hex block number: %s", hex)
	}
	return strconv.ParseInt(hex[2:], 16, 64)
}

func writeJSONResponse(w http.ResponseWriter, statusCode int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("X-Powered-By", "https://github.dev/itxtoledo/ethereum-rpc-pool")
	w.WriteHeader(statusCode)
	w.Write(body)
}
