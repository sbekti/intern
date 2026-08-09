package apierror

import (
	"encoding/json"
	"net/http"

	"github.com/sbekti/intern/internal/api"
)

// Write emits the ordinary API error envelope used outside device-flow OAuth errors.
func Write(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(api.ErrorResponse{Code: code, Message: message})
}
