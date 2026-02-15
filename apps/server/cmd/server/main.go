package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"fosscord/apps/server/internal/config"
	"fosscord/apps/server/internal/httpapi"
	"fosscord/apps/server/internal/serverstate"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := config.Load()
	state, err := serverstate.New(cfg)
	if err != nil {
		logger.Error("failed to initialize server state", "error", err)
		os.Exit(1)
	}

	logger.Info("starting server",
		"addr", cfg.Addr,
		"data_dir", cfg.DataDir,
		"db_path_set", cfg.DatabasePath != "",
		"admin_token_set", cfg.AdminToken != "",
		"livekit_url_set", cfg.LiveKitURL != "",
		"livekit_api_key_set", cfg.LiveKitAPIKey != "",
		"livekit_api_secret_set", cfg.LiveKitAPISecret != "",
	)

	router := httpapi.NewRouter(cfg, state)
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
