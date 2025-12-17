package common

import (
	"encoding/json"
	"net/http"
)

// Response is a generic API response wrapper.
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// WriteJSON sends a JSON response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// WriteError sends an error response with the given status code.
func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, Response{
		Success: false,
		Error:   message,
	})
}

// WriteSuccess sends a success response with the given data.
func WriteSuccess(w http.ResponseWriter, data interface{}) {
	WriteJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    data,
	})
}
