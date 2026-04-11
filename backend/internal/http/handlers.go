package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"time"
)

type errorResponse struct {
	Error apiError `json:"error"`
}
type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func WriteJSON(w stdhttp.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload != nil {
		_ = json.NewEncoder(w).Encode(payload)
	}
}
func WriteError(w stdhttp.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, errorResponse{Error: apiError{Code: code, Message: message}})
}
func (h *Handler) health(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
	WriteJSON(w, stdhttp.StatusOK, map[string]any{"status": "ok"})
}
func (h *Handler) ready(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := h.storage.Ready(ctx); err != nil {
		WriteError(w, stdhttp.StatusServiceUnavailable, "dependency_unavailable", "storage is not ready")
		return
	}
	if err := h.ejabberd.Ready(ctx); err != nil {
		WriteError(w, stdhttp.StatusServiceUnavailable, "dependency_unavailable", "ejabberd is not ready")
		return
	}
	WriteJSON(w, stdhttp.StatusOK, map[string]any{"status": "ready"})
}
func (h *Handler) notFound(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if r.URL.Path == "/" {
		WriteJSON(w, stdhttp.StatusOK, map[string]any{"service": "api", "status": "ok"})
		return
	}
	WriteError(w, stdhttp.StatusNotFound, "not_found", "resource not found")
}
