package handlers

import (
	"encoding/json"
	"net/http"
)

// SendError answers with HTTP 400, for requests the pool itself rejects.
func SendError(w http.ResponseWriter, code int, message string, id any) {
	SendErrorWithData(w, http.StatusBadRequest, code, message, id, nil)
}

// SendErrorWithData answers with a JSON-RPC error and an explicit HTTP status.
// Upstream failures use 502/504 so clients do not read them as a bad request.
func SendErrorWithData(w http.ResponseWriter, status, code int, message string, id any, data any) {
	errObj := map[string]any{
		"code":    code,
		"message": message,
	}
	if data != nil {
		errObj["data"] = data
	}
	errorResponse := map[string]any{
		"jsonrpc": "2.0",
		"error":   errObj,
		"id":      id,
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Powered-By", "https://github.dev/itxtoledo/ethereum-rpc-pool")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(errorResponse)
}
