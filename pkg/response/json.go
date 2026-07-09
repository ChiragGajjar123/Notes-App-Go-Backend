package response

import (
	"encoding/json"
	"net/http"
)

// JSON sends a JSON response with status code
func JSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(data)
}

// Error sends a JSON error response with status code
func Error(w http.ResponseWriter, statusCode int, errMsg string) {
	JSON(w, statusCode, map[string]string{"error": errMsg})
}
