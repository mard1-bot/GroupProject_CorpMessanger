package http

import (
	"log/slog"
	stdhttp "net/http"

	"go-backend-scaffold/internal/ejabberd"
	"go-backend-scaffold/internal/storage"
)

type Handler struct {
	logger   *slog.Logger
	storage  storage.Storage
	ejabberd ejabberd.Client
}

func NewHandler(logger *slog.Logger, storage storage.Storage, ejabberd ejabberd.Client) stdhttp.Handler {
	h := &Handler{logger: logger, storage: storage, ejabberd: ejabberd}
	mux := stdhttp.NewServeMux()
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("GET /ready", h.ready)
	mux.HandleFunc("/", h.notFound)
	return RecoverMiddleware(logger)(AccessLogMiddleware(logger)(mux))
}
