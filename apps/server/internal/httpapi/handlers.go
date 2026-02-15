package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"fosscord/apps/server/internal/config"
	"fosscord/apps/server/internal/serverstate"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

type handlers struct {
	cfg   config.Config
	state *serverstate.State
}

type healthResponse struct {
	Status string `json:"status"`
}

type serverInfoResponse struct {
	ServerID                  string   `json:"serverId"`
	Name                      string   `json:"name"`
	PublicKeyFingerprintEmoji string   `json:"publicKeyFingerprintEmoji"`
	ServerFingerprint         string   `json:"serverFingerprint"`
	ServerPublicKey           string   `json:"serverPublicKey"`
	LiveKitURL                string   `json:"livekitUrl"`
	AdminPublicKeys           []string `json:"adminPublicKeys"`
}

type createInviteRequest struct {
	ClientPublicKey string `json:"clientPublicKey"`
	Label           string `json:"label"`
}

type createInviteByClientRequest struct {
	AdminPublicKey  string `json:"adminPublicKey"`
	ClientPublicKey string `json:"clientPublicKey"`
	Label           string `json:"label"`
	IssuedAt        string `json:"issuedAt"`
	Signature       string `json:"signature"`
}

type listInvitesByClientRequest struct {
	AdminPublicKey string `json:"adminPublicKey"`
	IssuedAt       string `json:"issuedAt"`
	Signature      string `json:"signature"`
}

type connectBeginRequest struct {
	InviteID string `json:"inviteId"`
}

type createMessageRequest struct {
	ContentMarkdown string `json:"contentMarkdown"`
}

type editMessageRequest struct {
	ContentMarkdown string `json:"contentMarkdown"`
}

type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(_ *http.Request) bool { return true },
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
		AdminPublicKeys:           info.AdminPublicKeys,
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

func (h handlers) postAdminInvitesClientSigned(w http.ResponseWriter, r *http.Request) {
	var req createInviteByClientRequest
	if err := decodeJSON(r, &req); err != nil {
		writeAPIError(w, &serverstate.APIError{Status: http.StatusBadRequest, Code: "invalid_json", Message: err.Error()})
		return
	}

	result, err := h.state.CreateInviteByAdminClient(serverstate.CreateInviteByAdminClientRequest{
		AdminPublicKey:  req.AdminPublicKey,
		ClientPublicKey: req.ClientPublicKey,
		Label:           req.Label,
		IssuedAt:        req.IssuedAt,
		Signature:       req.Signature,
	})
	if err != nil {
		writeAPIError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (h handlers) postAdminInvitesListClientSigned(w http.ResponseWriter, r *http.Request) {
	var req listInvitesByClientRequest
	if err := decodeJSON(r, &req); err != nil {
		writeAPIError(w, &serverstate.APIError{Status: http.StatusBadRequest, Code: "invalid_json", Message: err.Error()})
		return
	}

	result, err := h.state.ListInvitesByAdminClient(serverstate.ListInvitesByAdminClientRequest{
		AdminPublicKey: req.AdminPublicKey,
		IssuedAt:       req.IssuedAt,
		Signature:      req.Signature,
	})
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

func (h handlers) getChannelMessages(w http.ResponseWriter, r *http.Request) {
	channelID := chi.URLParam(r, "channelID")
	sessionToken, err := bearerTokenFromHeader(r)
	if err != nil {
		writeAPIError(w, err)
		return
	}

	limit := 100
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr != nil {
			writeAPIError(w, &serverstate.APIError{Status: http.StatusBadRequest, Code: "invalid_limit", Message: "limit must be an integer"})
			return
		}
		limit = parsed
	}

	result, err := h.state.ListMessages(sessionToken, channelID, limit)
	if err != nil {
		writeAPIError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (h handlers) postChannelMessage(w http.ResponseWriter, r *http.Request) {
	channelID := chi.URLParam(r, "channelID")
	sessionToken, err := bearerTokenFromHeader(r)
	if err != nil {
		writeAPIError(w, err)
		return
	}

	var req createMessageRequest
	if err := decodeJSON(r, &req); err != nil {
		writeAPIError(w, &serverstate.APIError{Status: http.StatusBadRequest, Code: "invalid_json", Message: err.Error()})
		return
	}

	message, err := h.state.CreateMessage(sessionToken, channelID, req.ContentMarkdown)
	if err != nil {
		writeAPIError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"message": message})
}

func (h handlers) patchChannelMessage(w http.ResponseWriter, r *http.Request) {
	channelID := chi.URLParam(r, "channelID")
	messageID := chi.URLParam(r, "messageID")
	sessionToken, err := bearerTokenFromHeader(r)
	if err != nil {
		writeAPIError(w, err)
		return
	}

	var req editMessageRequest
	if err := decodeJSON(r, &req); err != nil {
		writeAPIError(w, &serverstate.APIError{Status: http.StatusBadRequest, Code: "invalid_json", Message: err.Error()})
		return
	}

	message, err := h.state.EditMessage(sessionToken, channelID, messageID, req.ContentMarkdown)
	if err != nil {
		writeAPIError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"message": message})
}

func (h handlers) getChannelStream(w http.ResponseWriter, r *http.Request) {
	channelID := chi.URLParam(r, "channelID")
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		writeAPIError(w, &serverstate.APIError{
			Status:  http.StatusUnauthorized,
			Code:    "missing_session_token",
			Message: "session token is required",
		})
		return
	}

	stream, cancel, err := h.state.SubscribeChannelEvents(token, channelID)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	defer cancel()

	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		writeAPIError(w, fmt.Errorf("upgrade websocket: %w", err))
		return
	}
	defer conn.Close()

	if err := conn.WriteJSON(serverstate.ChannelEvent{Type: "ready"}); err != nil {
		return
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-done:
			return
		case event, ok := <-stream:
			if !ok {
				return
			}
			if err := conn.WriteJSON(event); err != nil {
				return
			}
		}
	}
}

func (h handlers) postLiveKitToken(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusNotImplemented, errorResponse{
		Error:   "not_implemented",
		Message: "livekit token generation stub: implement with server-sdk-go in next step",
	})
}

func (h handlers) serveWebApp(w http.ResponseWriter, r *http.Request) {
	webDist := strings.TrimSpace(h.cfg.WebDistDir)
	if webDist == "" {
		http.NotFound(w, r)
		return
	}

	cleaned := path.Clean("/" + r.URL.Path)
	if cleaned == "/api" || strings.HasPrefix(cleaned, "/api/") {
		http.NotFound(w, r)
		return
	}
	relPath := strings.TrimPrefix(cleaned, "/")
	if relPath == "" || relPath == "." {
		relPath = "index.html"
	}

	assetPath := filepath.Join(webDist, filepath.FromSlash(relPath))
	if info, err := os.Stat(assetPath); err == nil && !info.IsDir() {
		http.ServeFile(w, r, assetPath)
		return
	}

	indexPath := filepath.Join(webDist, "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		http.NotFound(w, r)
		return
	}

	http.ServeFile(w, r, indexPath)
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

func bearerTokenFromHeader(r *http.Request) (string, error) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	prefix := "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", &serverstate.APIError{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "missing bearer token"}
	}

	token := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	if token == "" {
		return "", &serverstate.APIError{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "empty bearer token"}
	}
	return token, nil
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
