package http

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
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
	if payload == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		return
	}

	// Marshal to buffer first to catch encoding errors before writing headers
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(payload); err != nil {
		slog.Error("JSON encoding error", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(stdhttp.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(errorResponse{Error: apiError{Code: "internal", Message: "Failed to encode response"}})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}
func (h *Handler) health(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
	WriteJSON(w, stdhttp.StatusOK, map[string]any{"status": "ok"})
}
func (h *Handler) ready(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := h.storage.Ready(ctx); err != nil {
		WriteErrorCode(w, stdhttp.StatusServiceUnavailable, "dependency_unavailable", "storage is not ready")
		return
	}
	if err := h.ejabberd.Ready(ctx); err != nil {
		WriteErrorCode(w, stdhttp.StatusServiceUnavailable, "dependency_unavailable", "ejabberd is not ready")
		return
	}
	WriteJSON(w, stdhttp.StatusOK, map[string]any{"status": "ready"})
}
func (h *Handler) notFound(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if r.URL.Path == "/" {
		WriteJSON(w, stdhttp.StatusOK, map[string]any{"service": "api", "status": "ok"})
		return
	}
	WriteErrorCode(w, stdhttp.StatusNotFound, "not_found", "resource not found")
}
