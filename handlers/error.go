package handlers

import (
	"encoding/json"
	"net/http"
)

func SendError(w http.ResponseWriter, code int, message string, id any) {
	errorResponse := map[string]any{
		"jsonrpc": "2.0",
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
		"id": id,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(errorResponse)
}
