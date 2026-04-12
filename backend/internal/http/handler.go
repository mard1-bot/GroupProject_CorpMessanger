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
	logger      *slog.Logger
	storage     storage.Storage
	ejabberd    ejabberd.Client
	jwt         *auth.JWTService
	corsOrigins map[string]bool
}

func NewHandler(logger *slog.Logger, storage storage.Storage, ejabberd ejabberd.Client, jwtSecret string, corsOrigins []string) stdhttp.Handler {
	// Build CORS origins map
	originsMap := make(map[string]bool)
	for _, origin := range corsOrigins {
		originsMap[origin] = true
	}

	h := &Handler{
		logger:      logger,
		storage:     storage,
		ejabberd:    ejabberd,
		jwt:         auth.NewJWTService(jwtSecret),
		corsOrigins: originsMap,
	}

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(h.corsMiddleware)
	r.Use(AccessLogMiddleware(logger))
	r.Use(RecoverMiddleware(logger))
	r.Use(middleware.StripSlashes)

	r.Get("/health", h.health)
	r.Get("/ready", h.ready)

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/register", h.register)
		r.Post("/auth/login", h.login)

		r.Group(func(r chi.Router) {
			r.Use(AuthMiddleware(h.jwt, h.storage))

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

func (h *Handler) corsMiddleware(next stdhttp.Handler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		origin := r.Header.Get("Origin")

		// Only set CORS headers for whitelisted origins
		if h.corsOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}
		// For requests without origin or unknown origins, no CORS headers are set
		// This is more secure than wildcard

		if r.Method == "OPTIONS" {
			w.WriteHeader(stdhttp.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func decodeJSON(r io.Reader, v interface{}) error {
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	return decoder.Decode(v)
}
