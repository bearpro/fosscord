package httpapi

import (
	"net/http"
	"strings"

	"fosscord/apps/server/internal/config"
	"fosscord/apps/server/internal/serverstate"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func NewRouter(cfg config.Config, state *serverstate.State) http.Handler {
	h := handlers{cfg: cfg, state: state}

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
			"http://localhost:8088",
			"http://127.0.0.1:8088",
			"http://localhost:3000",
			"http://127.0.0.1:3000",
			"tauri://localhost",
			"https://tauri.localhost",
		},
		AllowedMethods: []string{"GET", "POST", "PATCH", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type"},
		MaxAge:         300,
	}))

	r.Get("/health", h.getHealth)
	r.Route("/api", func(api chi.Router) {
		api.Get("/server-info", h.getServerInfo)
		api.Get("/channels", h.getChannels)
		api.Route("/channels/{channelID}", func(channel chi.Router) {
			channel.Get("/messages", h.getChannelMessages)
			channel.Post("/messages", h.postChannelMessage)
			channel.Patch("/messages/{messageID}", h.patchChannelMessage)
			channel.Get("/stream", h.getChannelStream)
		})
		api.Post("/connect/begin", h.postConnectBegin)
		api.Post("/connect/finish", h.postConnectFinish)
		api.Route("/admin", func(admin chi.Router) {
			admin.Post("/invites", h.postAdminInvites)
			admin.Post("/invites/client-signed", h.postAdminInvitesClientSigned)
			admin.Post("/invites/list/client-signed", h.postAdminInvitesListClientSigned)
		})
		api.Post("/livekit/token", h.postLiveKitToken)
		api.Post("/livekit/voice/touch", h.postLiveKitVoiceTouch)
		api.Post("/livekit/voice/leave", h.postLiveKitVoiceLeave)
		api.Get("/livekit/voice/channels/{channelID}/state", h.getLiveKitVoiceChannelState)
	})

	if strings.TrimSpace(cfg.WebDistDir) != "" {
		r.Get("/", h.serveWebApp)
		r.Get("/*", h.serveWebApp)
	}

	return r
}
