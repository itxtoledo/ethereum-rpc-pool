package handlers

import (
	"encoding/json"
	"net/http"
)

func SendError(w http.ResponseWriter, code int, message string, id interface{}) {
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
