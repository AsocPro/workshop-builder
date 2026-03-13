package main

import (
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/asocpro/workshop-builder/backend/handlers"
	"github.com/asocpro/workshop-builder/backend/store"
)

func NewServer(meta *store.Metadata, st *store.State, managementURL string) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// CORS for dev (frontend dev server proxies, but be permissive)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	h := handlers.New(meta, st, managementURL)

	// Student API
	r.Get("/api/state", h.GetState)
	r.Get("/api/steps", h.ListSteps)
	r.Get("/api/steps/{id}/content", h.GetStepContent)
	r.Post("/api/steps/{id}/navigate", h.Navigate)
	r.Post("/api/steps/{id}/validate", h.Validate)
	r.Get("/api/commands", h.ListCommands)
	r.Get("/api/recordings", h.ListRecordings)
	r.Get("/api/recordings/{filename}", h.GetRecording)
	r.Post("/api/steps/{id}/llm/help", h.LLMHelp)
	r.Get("/api/steps/{id}/llm/history", h.LLMHistory)

	// Terminal WebSocket
	r.Get("/ws/terminal", h.TerminalWS)

	// ttyd reverse proxy — preserve /ttyd prefix so ttyd sees it (matches --base-path /ttyd)
	ttydURL, _ := url.Parse("http://127.0.0.1:7681")
	ttydProxy := httputil.NewSingleHostReverseProxy(ttydURL)
	r.Handle("/ttyd", ttydProxy)
	r.Handle("/ttyd/*", ttydProxy)

	// Embedded frontend (must be last — catches all unmatched routes)
	r.Mount("/", frontendHandler())

	return r
}
