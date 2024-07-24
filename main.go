package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
)

// List of RPCs to be loaded from the environment variable
var rpcs []string

// Index used for the round robin algorithm
var currentIndex uint32

func main() {
	// Load RPCs from the environment variable
	rpcList := os.Getenv("RPC_LIST")
	if rpcList == "" {
		log.Fatal("The environment variable RPC_LIST is not defined")
	}
	rpcs = strings.Split(rpcList, ",")

	// Load the port from the environment variable, default to 8080 if not set
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Set up the handler for the "/" route
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			sendError(w, -32601, "Method not allowed", nil)
			return
		}

		// Get the next RPC using round robin
		targetRPC := getNextRPC()
		fmt.Printf("Proxying request to RPC: %s\n", targetRPC)

		// Create a new request with the same body as the original request
		reqBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			sendError(w, -32603, "Error reading request body", nil)
			return
		}

		var jsonRequest map[string]interface{}
		if err := json.Unmarshal(reqBody, &jsonRequest); err != nil {
			sendError(w, -32603, "Invalid JSON request body", nil)
			return
		}

		id := jsonRequest["id"]

		proxyReq, err := http.NewRequest("POST", targetRPC, bytes.NewBuffer(reqBody))
		if err != nil {
			sendError(w, -32603, "Error creating proxy request", id)
			return
		}

		// Copy the headers from the original request to the proxy request
		for key, values := range r.Header {
			for _, value := range values {
				proxyReq.Header.Add(key, value)
			}
		}

		// Send the request to the RPC
		client := &http.Client{}
		resp, err := client.Do(proxyReq)
		if err != nil {
			sendError(w, -32603, "Error making request to RPC", id)
			return
		}
		defer resp.Body.Close()

		// Read the RPC response
		respBody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			sendError(w, -32603, "Error reading RPC response", id)
			return
		}

		// Copy the headers from the RPC response to the client response
		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}

		// Write the status code of the RPC response to the client response
		w.WriteHeader(resp.StatusCode)
		// Write the body of the RPC response to the client response
		w.Write(respBody)
	})

	// Start the server on the specified port
	log.Printf("Server started on port %s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// Function to get the next RPC using round robin
func getNextRPC() string {
	index := atomic.AddUint32(&currentIndex, 1)
	return rpcs[(index-1)%uint32(len(rpcs))]
}

// Function to send error responses in Ethereum RPC format
func sendError(w http.ResponseWriter, code int, message string, id interface{}) {
	errorResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
		"id": id,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(errorResponse)
}
