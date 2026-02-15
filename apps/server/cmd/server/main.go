package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"fosscord/apps/server/internal/config"
	"fosscord/apps/server/internal/httpapi"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := config.Load()

	logger.Info("starting server",
		"addr", cfg.Addr,
		"livekit_url_set", cfg.LiveKitURL != "",
		"livekit_api_key_set", cfg.LiveKitAPIKey != "",
		"livekit_api_secret_set", cfg.LiveKitAPISecret != "",
	)

	router := httpapi.NewRouter(cfg)
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server exited", "error", err)
		os.Exit(1)
	}
}
