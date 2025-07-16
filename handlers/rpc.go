package handlers

import (
	"bytes"
	"encoding/json"
	"ethereum-rpc-pool/utils"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var rpcs []string

// SetRPCs initializes the list of RPCs from a comma-separated string.
func SetRPCs(rpcList string) {
	rpcs = strings.Split(rpcList, ",")
	InitializeRPCStatus(rpcs)
}

// --- Background Fetching ---

// FetchBlockNumber starts a ticker to periodically fetch the latest block number from RPCs.
func FetchBlockNumber() {
	interval, err := strconv.Atoi(os.Getenv("BLOCK_NUMBER_FETCH_INTERVAL_SECONDS"))
	if err != nil {
		interval = 10 // Default to 10 seconds
	}

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		go fetchFromAllRPCs()
	}
}

// fetchFromAllRPCs iterates through the configured RPCs and fetches data from them.
func fetchFromAllRPCs() {
	for _, rpc := range rpcs {
		go fetchBlockNumberFromRPC(rpc)
	}
}

// fetchBlockNumberFromRPC fetches the latest block number from a single RPC.
func fetchBlockNumberFromRPC(rpcURL string) {
	start := time.Now()
	requestBody, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_blockNumber",
		"params":  []interface{}{},
		"id":      1,
	})
	if err != nil {
		log.Printf("Error creating eth_blockNumber request body for %s: %v", rpcURL, err)
		SetRPCStatus(rpcURL, "", 0, false)
		return
	}

	respBody, err := makeRPCRequest(rpcURL, requestBody)
	responseTime := time.Since(start).Milliseconds()
	if err != nil {
		log.Printf("Error fetching eth_blockNumber from %s: %v", rpcURL, err)
		SetRPCStatus(rpcURL, "", responseTime, false)
		return
	}

	var jsonResponse map[string]interface{}
	if err := json.Unmarshal(respBody, &jsonResponse); err != nil {
		log.Printf("Error unmarshalling eth_blockNumber response from %s: %v", rpcURL, err)
		SetRPCStatus(rpcURL, "", responseTime, false)
		return
	}

	if result, ok := jsonResponse["result"]; ok {
		if blockNumber, ok := result.(string); ok {
			SetRPCStatus(rpcURL, blockNumber, responseTime, true)
			log.Printf("Successfully fetched block number from %s: %s", rpcURL, blockNumber)
			go fetchBlock(rpcURL, blockNumber) // Fetch the full block as well
		} else {
			log.Printf("Invalid block number format from %s", rpcURL)
			SetRPCStatus(rpcURL, "", responseTime, false)
		}
	} else {
		SetRPCStatus(rpcURL, "", responseTime, false)
	}
}

// fetchBlock fetches a full block by its number from an RPC.
func fetchBlock(rpcURL, blockNumber string) {
	requestBody, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_getBlockByNumber",
		"params":  []interface{}{blockNumber, true},
		"id":      1,
	})
	if err != nil {
		log.Printf("Error creating eth_getBlockByNumber request body for %s: %v", rpcURL, err)
		return
	}

	respBody, err := makeRPCRequest(rpcURL, requestBody)
	if err != nil {
		log.Printf("Error fetching block %s from %s: %v", blockNumber, rpcURL, err)
		return
	}

	var jsonResponse map[string]interface{}
	if err := json.Unmarshal(respBody, &jsonResponse); err != nil {
		log.Printf("Error unmarshalling block response from %s: %v", rpcURL, err)
		return
	}

	if result, ok := jsonResponse["result"]; ok {
		if blockData, ok := result.(map[string]interface{}); ok {
			SetBlockCache(blockData)
			log.Printf("Successfully fetched block from %s: %s", rpcURL, blockNumber)
		}
	}
}

// --- HTTP Handlers ---

// RPCHandler is the main entry point for all RPC requests.
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

	reqBody, err := io.ReadAll(r.Body)
	if err != nil {
		SendError(w, -32603, "Error reading request body", nil)
		return
	}
	defer r.Body.Close()

	var jsonRequest map[string]interface{}
	if err := json.Unmarshal(reqBody, &jsonRequest); err != nil {
		SendError(w, -32603, "Invalid JSON request body", nil)
		return
	}

	// Check if we can handle this request from cache
	if method, ok := jsonRequest["method"].(string); ok {
		switch method {
		case "eth_blockNumber":
			if handleBlockNumber(w, jsonRequest) {
				return
			}
		case "eth_getBlockByNumber":
			if handleGetBlockByNumber(w, jsonRequest) {
				return
			}
		}
	}

	// If not handled by cache, proxy the request
	targetRPC := utils.GetNextRPC(rpcs)
	fmt.Printf("Proxying request to RPC: %s\n", targetRPC)
	proxyRequest(w, targetRPC, reqBody, jsonRequest["id"])
}

// StatusHandler provides the health status of the configured RPCs.
func StatusHandler(w http.ResponseWriter, r *http.Request) {
	sendJSONResponse(w, GetRPCStatus(), http.StatusOK)
}

// --- Handler Helpers ---

// handleBlockNumber serves eth_blockNumber requests from the cache if possible.
func handleBlockNumber(w http.ResponseWriter, jsonRequest map[string]interface{}) bool {
	statuses := GetRPCStatus()
	var latestBlockNumber string

	for _, status := range statuses {
		if status.Online && (latestBlockNumber == "" || status.BlockNumber > latestBlockNumber) {
			latestBlockNumber = status.BlockNumber
		}
	}

	if latestBlockNumber != "" {
		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      jsonRequest["id"],
			"result":  latestBlockNumber,
		}
		sendJSONResponse(w, response, http.StatusOK)
		return true
	}
	return false
}

// handleGetBlockByNumber serves eth_getBlockByNumber requests from the cache if possible.
func handleGetBlockByNumber(w http.ResponseWriter, jsonRequest map[string]interface{}) bool {
	params, ok := jsonRequest["params"].([]interface{})
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

	// Time-based expiration
	interval, err := strconv.Atoi(os.Getenv("BLOCK_NUMBER_FETCH_INTERVAL_SECONDS"))
	if err != nil {
		interval = 10 // Default to 10 seconds
	}
	if time.Since(cache.Timestamp) > time.Duration(interval)*time.Second {
		return false // Cache is stale
	}

	cachedBlockNumber, ok := cache.BlockData["number"].(string)
	if !ok {
		return false
	}

	if requestedBlock == "latest" {
		statuses := GetRPCStatus()
		var latestBlockNumber string
		for _, status := range statuses {
			if status.Online && (latestBlockNumber == "" || status.BlockNumber > latestBlockNumber) {
				latestBlockNumber = status.BlockNumber
			}
		}
		if latestBlockNumber != cachedBlockNumber {
			return false // Cached block is not the latest one
		}
	} else if requestedBlock != cachedBlockNumber {
		return false // incorrect block in cache
	}

	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      jsonRequest["id"],
		"result":  cache.BlockData,
	}
	sendJSONResponse(w, response, http.StatusOK)
	return true
}

// proxyRequest forwards the original request to a target RPC.
func proxyRequest(w http.ResponseWriter, targetRPC string, reqBody []byte, id interface{}) {
	respBody, err := makeRPCRequest(targetRPC, reqBody)
	if err != nil {
		SendError(w, -32603, "Error making request to RPC", id)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("X-Powered-By", "https://github.dev/itxtoledo/ethereum-rpc-pool")
	w.WriteHeader(http.StatusOK)
	w.Write(respBody)
}

// --- Utility Functions ---

// makeRPCRequest sends a JSON-RPC request to the given URL and returns the response body.
func makeRPCRequest(rpcURL string, requestBody []byte) ([]byte, error) {
	proxyReq, err := http.NewRequest("POST", rpcURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("error creating proxy request: %w", err)
	}
	proxyReq.Header.Add("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(proxyReq)
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

// sendJSONResponse is a helper to send a JSON response with common headers.
func sendJSONResponse(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("X-Powered-By", "https://github.dev/itxtoledo/ethereum-rpc-pool")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
	}
}