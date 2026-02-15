package httpapi

import (
	"encoding/json"
	"net/http"

	"fosscord/apps/server/internal/config"
)

type handlers struct {
	cfg config.Config
}

type healthResponse struct {
	Status string `json:"status"`
}

type serverInfoResponse struct {
	Name                      string `json:"name"`
	PublicKeyFingerprintEmoji string `json:"publicKeyFingerprintEmoji"`
	LiveKitURL                string `json:"livekitUrl"`
}

type livekitTokenResponse struct {
	Token string `json:"token"`
}

type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func (h handlers) getHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
}

func (h handlers) getServerInfo(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, serverInfoResponse{
		Name:                      h.cfg.ServerName,
		PublicKeyFingerprintEmoji: h.cfg.PublicKeyFingerprintEmoji,
		LiveKitURL:                h.cfg.LiveKitURL,
	})
}

func (h handlers) postLiveKitToken(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusNotImplemented, errorResponse{
		Error:   "not_implemented",
		Message: "livekit token generation stub: implement with server-sdk-go in next step",
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
