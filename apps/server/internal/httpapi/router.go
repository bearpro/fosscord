package httpapi

import (
	"net/http"

	"fosscord/apps/server/internal/config"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func NewRouter(cfg config.Config) http.Handler {
	h := handlers{cfg: cfg}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{
			"http://localhost:1420",
			"http://127.0.0.1:1420",
			"http://localhost:5173",
			"http://127.0.0.1:5173",
			"tauri://localhost",
			"https://tauri.localhost",
		},
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type"},
		MaxAge:         300,
	}))

	r.Get("/health", h.getHealth)
	r.Route("/api", func(api chi.Router) {
		api.Get("/server-info", h.getServerInfo)
		api.Post("/livekit/token", h.postLiveKitToken)
	})

	return r
}
