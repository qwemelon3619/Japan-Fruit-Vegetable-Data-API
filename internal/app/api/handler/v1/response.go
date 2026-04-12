package v1

import (
	"encoding/json"
	"net/http"
)

func writeMethodNotAllowed(w http.ResponseWriter) {
	writeErr(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
}

func writeOK(w http.ResponseWriter, data any, meta apiMeta) {
	if meta == nil {
		meta = apiMeta{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "meta": meta})
}

func writeErr(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("X-Error-Code", code)
	writeJSON(w, status, map[string]any{"error": apiError{Code: code, Message: message}})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
