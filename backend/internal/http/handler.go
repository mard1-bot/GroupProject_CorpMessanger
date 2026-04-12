package http

import (
	"encoding/json"
	"io"
	"log/slog"
	stdhttp "net/http"

	"corp-messenger/backend/internal/auth"
	"corp-messenger/backend/internal/ejabberd"
	"corp-messenger/backend/internal/storage"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Handler struct {
	logger   *slog.Logger
	storage  storage.Storage
	ejabberd ejabberd.Client
	jwt      *auth.JWTService
}

func NewHandler(logger *slog.Logger, storage storage.Storage, ejabberd ejabberd.Client, jwtSecret string) stdhttp.Handler {
	h := &Handler{
		logger:   logger,
		storage:  storage,
		ejabberd: ejabberd,
		jwt:      auth.NewJWTService(jwtSecret),
	}

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(AccessLogMiddleware(logger))
	r.Use(RecoverMiddleware(logger))
	r.Use(middleware.StripSlashes)

	r.Get("/health", h.health)
	r.Get("/ready", h.ready)

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/register", h.register)
		r.Post("/auth/login", h.login)

		r.Group(func(r chi.Router) {
			r.Use(AuthMiddleware(h.jwt))

			r.Post("/auth/logout", h.logout)
			r.Get("/auth/me", h.getCurrentUser)

			r.Get("/users", h.getUsers)
			r.Get("/users/{id}", h.getUserByID)
			r.Put("/users/me", h.updateCurrentUser)

			r.Post("/chats", h.createChat)
			r.Get("/chats", h.getUserChats)
			r.Get("/chats/{id}", h.getChatByID)
			r.Post("/chats/{id}/members", h.addChatMember)

			r.Post("/chats/{id}/messages", h.sendMessage)
			r.Get("/chats/{id}/messages", h.getChatMessages)
		})
	})

	r.NotFound(h.notFound)

	return r
}

func decodeJSON(r io.Reader, v interface{}) error {
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	return decoder.Decode(v)
}
