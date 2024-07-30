package handlers

import (
	"bytes"
	"encoding/json"
	"ethereum-rpc-pool/utils"
	"fmt"
	"io"
	"net/http"
	"strings"
)

var rpcs []string

func SetRPCs(rpcList string) {
	rpcs = strings.Split(rpcList, ",")
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

	// Get the next RPC using round robin
	targetRPC := utils.GetNextRPC(rpcs)
	fmt.Printf("Proxying request to RPC: %s\n", targetRPC)

	// Create a new request with the same body as the original request
	reqBody, err := io.ReadAll(r.Body)
	if err != nil {
		SendError(w, -32603, "Error reading request body", nil)
		return
	}

	var jsonRequest map[string]interface{}
	if err := json.Unmarshal(reqBody, &jsonRequest); err != nil {
		SendError(w, -32603, "Invalid JSON request body", nil)
		return
	}

	id := jsonRequest["id"]

	proxyReq, err := http.NewRequest("POST", targetRPC, bytes.NewBuffer(reqBody))
	if err != nil {
		SendError(w, -32603, "Error creating proxy request", id)
		return
	}

	proxyReq.Header.Add("Content-Type", "application/json")

	// Send the request to the RPC
	client := &http.Client{}
	resp, err := client.Do(proxyReq)
	if err != nil {
		SendError(w, -32603, "Error making request to RPC", id)
		return
	}
	defer resp.Body.Close()

	// Read the RPC response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		SendError(w, -32603, "Error reading RPC response", id)
		return
	}

	// Forward the response body to the original client
	w.WriteHeader(resp.StatusCode)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("X-Powered-By", "https://github.dev/itxtoledo/ethereum-rpc-pool")
	w.Write(respBody)
}
