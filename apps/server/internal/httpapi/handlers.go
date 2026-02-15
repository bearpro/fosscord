package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"fosscord/apps/server/internal/config"
	"fosscord/apps/server/internal/serverstate"
)

type handlers struct {
	cfg   config.Config
	state *serverstate.State
}

type healthResponse struct {
	Status string `json:"status"`
}

type serverInfoResponse struct {
	ServerID                  string `json:"serverId"`
	Name                      string `json:"name"`
	PublicKeyFingerprintEmoji string `json:"publicKeyFingerprintEmoji"`
	ServerFingerprint         string `json:"serverFingerprint"`
	ServerPublicKey           string `json:"serverPublicKey"`
	LiveKitURL                string `json:"livekitUrl"`
}

type livekitTokenResponse struct {
	Token string `json:"token"`
}

type createInviteRequest struct {
	ClientPublicKey string `json:"clientPublicKey"`
	Label           string `json:"label"`
}

type connectBeginRequest struct {
	InviteID string `json:"inviteId"`
}

type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func (h handlers) getHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
}

func (h handlers) getServerInfo(w http.ResponseWriter, _ *http.Request) {
	info := h.state.ServerInfo()
	writeJSON(w, http.StatusOK, serverInfoResponse{
		ServerID:                  info.ServerID,
		Name:                      info.Name,
		PublicKeyFingerprintEmoji: info.ServerFingerprint,
		ServerFingerprint:         info.ServerFingerprint,
		ServerPublicKey:           info.ServerPublicKey,
		LiveKitURL:                info.LiveKitURL,
	})
}

func (h handlers) getChannels(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"channels": h.state.Channels(),
	})
}

func (h handlers) postAdminInvites(w http.ResponseWriter, r *http.Request) {
	if err := h.authorizeAdmin(r); err != nil {
		writeAPIError(w, err)
		return
	}

	var req createInviteRequest
	if err := decodeJSON(r, &req); err != nil {
		writeAPIError(w, &serverstate.APIError{Status: http.StatusBadRequest, Code: "invalid_json", Message: err.Error()})
		return
	}

	result, err := h.state.CreateInvite(strings.TrimSpace(req.ClientPublicKey), req.Label)
	if err != nil {
		writeAPIError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (h handlers) postConnectBegin(w http.ResponseWriter, r *http.Request) {
	var req connectBeginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeAPIError(w, &serverstate.APIError{Status: http.StatusBadRequest, Code: "invalid_json", Message: err.Error()})
		return
	}

	result, err := h.state.BeginConnect(req.InviteID)
	if err != nil {
		writeAPIError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (h handlers) postConnectFinish(w http.ResponseWriter, r *http.Request) {
	var req serverstate.FinishRequest
	if err := decodeJSON(r, &req); err != nil {
		writeAPIError(w, &serverstate.APIError{Status: http.StatusBadRequest, Code: "invalid_json", Message: err.Error()})
		return
	}

	result, err := h.state.FinishConnect(req)
	if err != nil {
		writeAPIError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (h handlers) postLiveKitToken(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusNotImplemented, errorResponse{
		Error:   "not_implemented",
		Message: "livekit token generation stub: implement with server-sdk-go in next step",
	})
}

func (h handlers) authorizeAdmin(r *http.Request) error {
	token := strings.TrimSpace(h.cfg.AdminToken)
	if token == "" {
		return &serverstate.APIError{Status: http.StatusServiceUnavailable, Code: "admin_disabled", Message: "ADMIN_TOKEN is not configured"}
	}

	header := strings.TrimSpace(r.Header.Get("Authorization"))
	prefix := "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return &serverstate.APIError{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "missing bearer token"}
	}

	if strings.TrimSpace(strings.TrimPrefix(header, prefix)) != token {
		return &serverstate.APIError{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "invalid admin token"}
	}

	return nil
}

func decodeJSON(r *http.Request, out any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	return nil
}

func writeAPIError(w http.ResponseWriter, err error) {
	var apiErr *serverstate.APIError
	if errors.As(err, &apiErr) {
		writeJSON(w, apiErr.Status, errorResponse{Error: apiErr.Code, Message: apiErr.Message})
		return
	}

	writeJSON(w, http.StatusInternalServerError, errorResponse{
		Error:   "internal_error",
		Message: fmt.Sprintf("internal error: %v", err),
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
