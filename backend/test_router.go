package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"github.com/go-chi/chi/v5"
)

func main() {
	r := chi.NewRouter()
	r.Get("/api/v1/users/{id}", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "GET ID") })
	r.Put("/api/v1/users/me", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "PUT ME") })

	req := httptest.NewRequest("PUT", "/api/v1/users/me", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	fmt.Printf("Status: %d\nBody: %s\n", rec.Code, rec.Body.String())
}
