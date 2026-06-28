package handlers

import (
	"encoding/json"
	"net/http"
)

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	statuses := GetRPCStatus()
	healthy := false
	for _, s := range statuses {
		if s.Online {
			healthy = true
			break
		}
	}

	code := http.StatusOK
	if !healthy && len(statuses) > 0 {
		code = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]any{
		"status":  map[bool]string{true: "ok", false: "degraded"}[healthy],
		"healthy": healthy,
	})
}
