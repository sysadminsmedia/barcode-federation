package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

func JSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

func Error(w http.ResponseWriter, status int, code, message string) {
	JSON(w, status, ErrorResponse{Error: message, Code: code})
}

func NotFound(w http.ResponseWriter, message string) {
	Error(w, http.StatusNotFound, "not_found", message)
}

func BadRequest(w http.ResponseWriter, message string) {
	Error(w, http.StatusBadRequest, "bad_request", message)
}

func Forbidden(w http.ResponseWriter, capability string) {
	Error(w, http.StatusForbidden, "forbidden", "missing capability: "+capability)
}

func Unauthorized(w http.ResponseWriter) {
	Error(w, http.StatusUnauthorized, "unauthorized", "valid Bearer token required")
}

func InternalError(w http.ResponseWriter, err error) {
	slog.Error("internal error", "error", err)
	Error(w, http.StatusInternalServerError, "internal_error", "internal server error")
}
