//go:build integration

package integration_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

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

type createInviteResponse struct {
	InviteID          string `json:"inviteId"`
	ServerBaseURL     string `json:"serverBaseUrl"`
	ServerFingerprint string `json:"serverFingerprint"`
	InviteLink        string `json:"inviteLink"`
}

type connectBeginRequest struct {
	InviteID string `json:"inviteId"`
}

type connectBeginResponse struct {
	ServerPublicKey   string `json:"serverPublicKey"`
	ServerFingerprint string `json:"serverFingerprint"`
	Challenge         string `json:"challenge"`
	ExpiresAt         string `json:"expiresAt"`
}

type connectFinishRequest struct {
	InviteID        string `json:"inviteId"`
	ClientPublicKey string `json:"clientPublicKey"`
	Challenge       string `json:"challenge"`
	Signature       string `json:"signature"`
	ClientInfo      struct {
		DisplayName string `json:"displayName"`
	} `json:"clientInfo"`
}

type channel struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Name string `json:"name"`
}

type connectFinishResponse struct {
	ServerID          string    `json:"serverId"`
	ServerName        string    `json:"serverName"`
	ServerFingerprint string    `json:"serverFingerprint"`
	LiveKitURL        string    `json:"livekitUrl"`
	Channels          []channel `json:"channels"`
	SessionToken      string    `json:"sessionToken"`
}

type connectedSession struct {
	Finish          connectFinishResponse
	ClientPublicKey string
}

type messageAuthor struct {
	DisplayName string `json:"displayName"`
	PublicKey   string `json:"publicKey"`
}

type channelMessage struct {
	ID              string        `json:"id"`
	ChannelID       string        `json:"channelId"`
	Author          messageAuthor `json:"author"`
	ContentMarkdown string        `json:"contentMarkdown"`
	CreatedAt       string        `json:"createdAt"`
	UpdatedAt       string        `json:"updatedAt"`
}

type listMessagesResponse struct {
	Messages []channelMessage `json:"messages"`
}

type mutateMessageRequest struct {
	ContentMarkdown string `json:"contentMarkdown"`
}

type mutateMessageResponse struct {
	Message channelMessage `json:"message"`
}

type apiErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

type liveKitTokenRequest struct {
	ChannelID string `json:"channelId"`
}

type liveKitTokenResponse struct {
	Token         string `json:"token"`
	RoomName      string `json:"roomName"`
	ChannelID     string `json:"channelId"`
	ParticipantID string `json:"participantId"`
}

type voiceTouchRequest struct {
	ChannelID          string `json:"channelId"`
	AudioStreams       int    `json:"audioStreams"`
	VideoStreams       int    `json:"videoStreams"`
	CameraEnabled      bool   `json:"cameraEnabled"`
	ScreenEnabled      bool   `json:"screenEnabled"`
	ScreenAudioEnabled bool   `json:"screenAudioEnabled"`
}

type voiceParticipant struct {
	PublicKey          string `json:"publicKey"`
	DisplayName        string `json:"displayName"`
	ChannelID          string `json:"channelId"`
	JoinedAt           string `json:"joinedAt"`
	LastSeenAt         string `json:"lastSeenAt"`
	AudioStreams       int    `json:"audioStreams"`
	VideoStreams       int    `json:"videoStreams"`
	CameraEnabled      bool   `json:"cameraEnabled"`
	ScreenEnabled      bool   `json:"screenEnabled"`
	ScreenAudioEnabled bool   `json:"screenAudioEnabled"`
}

type voiceStateResponse struct {
	ChannelID    string             `json:"channelId"`
	Participants []voiceParticipant `json:"participants"`
}

func TestHealth(t *testing.T) {
	t.Parallel()

	baseURL := apiBaseURL()
	body := requestJSON(t, http.MethodGet, baseURL+"/health", nil, nil, http.StatusOK)

	var parsed healthResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("failed to parse /health response as JSON: %v\nbody=%s", err, string(body))
	}

	if parsed.Status != "ok" {
		t.Fatalf("unexpected health status: got=%q want=%q", parsed.Status, "ok")
	}
}

func TestServerInfo(t *testing.T) {
	t.Parallel()

	baseURL := apiBaseURL()
	body := requestJSON(t, http.MethodGet, baseURL+"/api/server-info", nil, nil, http.StatusOK)

	var parsed serverInfoResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("failed to parse /api/server-info response as JSON: %v\nbody=%s", err, string(body))
	}

	if strings.TrimSpace(parsed.Name) == "" {
		t.Fatal("expected non-empty 'name'")
	}
	if strings.TrimSpace(parsed.PublicKeyFingerprintEmoji) == "" {
		t.Fatal("expected non-empty 'publicKeyFingerprintEmoji'")
	}
	if strings.TrimSpace(parsed.ServerFingerprint) == "" {
		t.Fatal("expected non-empty 'serverFingerprint'")
	}
	if strings.TrimSpace(parsed.ServerPublicKey) == "" {
		t.Fatal("expected non-empty 'serverPublicKey'")
	}
	if strings.TrimSpace(parsed.LiveKitURL) == "" {
		t.Fatal("expected non-empty 'livekitUrl'")
	}
	if parsed.AdminPublicKeys == nil {
		t.Fatal("expected 'adminPublicKeys' to be present (possibly empty array)")
	}
}

func TestConnectHandshakeSuccess(t *testing.T) {
	t.Parallel()

	baseURL := apiBaseURL()
	adminToken := adminToken()

	clientPublicB64, clientPrivate := generateClientKeypair(t)

	inviteBody := requestJSON(t, http.MethodPost, baseURL+"/api/admin/invites", map[string]string{
		"Authorization": "Bearer " + adminToken,
	}, createInviteRequest{ClientPublicKey: clientPublicB64, Label: "integration-success"}, http.StatusOK)

	var invite createInviteResponse
	mustParseJSON(t, inviteBody, &invite)

	beginBody := requestJSON(t, http.MethodPost, baseURL+"/api/connect/begin", nil, connectBeginRequest{InviteID: invite.InviteID}, http.StatusOK)

	var begin connectBeginResponse
	mustParseJSON(t, beginBody, &begin)

	challengeRaw, err := base64.StdEncoding.DecodeString(begin.Challenge)
	if err != nil {
		t.Fatalf("invalid challenge encoding: %v", err)
	}

	hash := signaturePayloadHash(challengeRaw, invite.InviteID, begin.ServerFingerprint)
	signature := ed25519.Sign(clientPrivate, hash[:])

	finishReq := connectFinishRequest{
		InviteID:        invite.InviteID,
		ClientPublicKey: clientPublicB64,
		Challenge:       begin.Challenge,
		Signature:       base64.StdEncoding.EncodeToString(signature),
	}
	finishReq.ClientInfo.DisplayName = "integration-client"

	finishBody := requestJSON(t, http.MethodPost, baseURL+"/api/connect/finish", nil, finishReq, http.StatusOK)

	var finish connectFinishResponse
	mustParseJSON(t, finishBody, &finish)

	if strings.TrimSpace(finish.ServerID) == "" {
		t.Fatal("expected non-empty serverId")
	}
	if strings.TrimSpace(finish.ServerName) == "" {
		t.Fatal("expected non-empty serverName")
	}
	if strings.TrimSpace(finish.ServerFingerprint) == "" {
		t.Fatal("expected non-empty serverFingerprint")
	}
	if len(finish.Channels) == 0 {
		t.Fatal("expected channels in handshake finish response")
	}
	if strings.TrimSpace(finish.SessionToken) == "" {
		t.Fatal("expected non-empty sessionToken")
	}
}

func TestConnectFingerprintMismatch(t *testing.T) {
	t.Parallel()

	baseURL := apiBaseURL()
	adminToken := adminToken()
	clientPublicB64, _ := generateClientKeypair(t)

	inviteBody := requestJSON(t, http.MethodPost, baseURL+"/api/admin/invites", map[string]string{
		"Authorization": "Bearer " + adminToken,
	}, createInviteRequest{ClientPublicKey: clientPublicB64, Label: "integration-fingerprint"}, http.StatusOK)

	var invite createInviteResponse
	mustParseJSON(t, inviteBody, &invite)

	beginBody := requestJSON(t, http.MethodPost, baseURL+"/api/connect/begin", nil, connectBeginRequest{InviteID: invite.InviteID}, http.StatusOK)

	var begin connectBeginResponse
	mustParseJSON(t, beginBody, &begin)

	err := verifyExpectedFingerprint("😈😈😈😈", begin.ServerFingerprint)
	if err == nil {
		t.Fatal("expected fingerprint mismatch error")
	}
}

func TestConnectInvalidSignature(t *testing.T) {
	t.Parallel()

	baseURL := apiBaseURL()
	adminToken := adminToken()

	allowedClientPublicB64, _ := generateClientKeypair(t)
	_, attackerPrivate := generateClientKeypair(t)

	inviteBody := requestJSON(t, http.MethodPost, baseURL+"/api/admin/invites", map[string]string{
		"Authorization": "Bearer " + adminToken,
	}, createInviteRequest{ClientPublicKey: allowedClientPublicB64, Label: "integration-invalid-signature"}, http.StatusOK)

	var invite createInviteResponse
	mustParseJSON(t, inviteBody, &invite)

	beginBody := requestJSON(t, http.MethodPost, baseURL+"/api/connect/begin", nil, connectBeginRequest{InviteID: invite.InviteID}, http.StatusOK)

	var begin connectBeginResponse
	mustParseJSON(t, beginBody, &begin)

	challengeRaw, err := base64.StdEncoding.DecodeString(begin.Challenge)
	if err != nil {
		t.Fatalf("invalid challenge encoding: %v", err)
	}

	hash := signaturePayloadHash(challengeRaw, invite.InviteID, begin.ServerFingerprint)
	forgedSignature := ed25519.Sign(attackerPrivate, hash[:])

	finishReq := connectFinishRequest{
		InviteID:        invite.InviteID,
		ClientPublicKey: allowedClientPublicB64,
		Challenge:       begin.Challenge,
		Signature:       base64.StdEncoding.EncodeToString(forgedSignature),
	}
	finishReq.ClientInfo.DisplayName = "integration-attacker"

	body := requestJSON(t, http.MethodPost, baseURL+"/api/connect/finish", nil, finishReq, http.StatusUnauthorized)

	var apiErr apiErrorResponse
	mustParseJSON(t, body, &apiErr)
	if apiErr.Error != "invalid_signature" {
		t.Fatalf("unexpected error code: got=%q want=%q body=%s", apiErr.Error, "invalid_signature", string(body))
	}
}

func TestTextMessagesCreateListEdit(t *testing.T) {
	t.Parallel()

	baseURL := apiBaseURL()
	session := createConnectedClientSession(t, baseURL)
	finish := session.Finish
	if strings.TrimSpace(finish.SessionToken) == "" {
		t.Fatal("expected sessionToken from connect/finish")
	}

	textChannelID := ""
	for _, ch := range finish.Channels {
		if ch.Type == "text" {
			textChannelID = ch.ID
			break
		}
	}
	if textChannelID == "" {
		t.Fatal("expected at least one text channel")
	}

	createBody := requestJSON(t, http.MethodPost, baseURL+"/api/channels/"+textChannelID+"/messages", map[string]string{
		"Authorization": "Bearer " + finish.SessionToken,
	}, mutateMessageRequest{ContentMarkdown: "Hello **integration** test"}, http.StatusOK)

	var created mutateMessageResponse
	mustParseJSON(t, createBody, &created)
	if created.Message.ID == "" {
		t.Fatal("expected message id")
	}
	if created.Message.ChannelID != textChannelID {
		t.Fatalf("unexpected channel id: got=%q want=%q", created.Message.ChannelID, textChannelID)
	}
	if strings.TrimSpace(created.Message.Author.PublicKey) == "" || strings.TrimSpace(created.Message.Author.DisplayName) == "" {
		t.Fatal("expected author info in created message")
	}

	listBody := requestJSON(t, http.MethodGet, baseURL+"/api/channels/"+textChannelID+"/messages?limit=100", map[string]string{
		"Authorization": "Bearer " + finish.SessionToken,
	}, nil, http.StatusOK)

	var listed listMessagesResponse
	mustParseJSON(t, listBody, &listed)
	if len(listed.Messages) == 0 {
		t.Fatal("expected non-empty message history")
	}

	containsCreated := false
	for _, message := range listed.Messages {
		if message.ID == created.Message.ID {
			containsCreated = true
			break
		}
	}
	if !containsCreated {
		t.Fatal("expected created message in history response")
	}

	editBody := requestJSON(t, http.MethodPatch, baseURL+"/api/channels/"+textChannelID+"/messages/"+created.Message.ID, map[string]string{
		"Authorization": "Bearer " + finish.SessionToken,
	}, mutateMessageRequest{ContentMarkdown: "Edited message"}, http.StatusOK)

	var edited mutateMessageResponse
	mustParseJSON(t, editBody, &edited)
	if edited.Message.ContentMarkdown != "Edited message" {
		t.Fatalf("unexpected edited content: got=%q", edited.Message.ContentMarkdown)
	}
	if strings.TrimSpace(edited.Message.UpdatedAt) == "" {
		t.Fatal("expected updatedAt to be set after edit")
	}
	if edited.Message.UpdatedAt < edited.Message.CreatedAt {
		t.Fatalf("updatedAt must not be earlier than createdAt: createdAt=%q updatedAt=%q", edited.Message.CreatedAt, edited.Message.UpdatedAt)
	}
}

func TestVoiceTokenAndPresence(t *testing.T) {
	t.Parallel()

	baseURL := apiBaseURL()
	session := createConnectedClientSession(t, baseURL)
	finish := session.Finish

	voiceChannelID := ""
	for _, ch := range finish.Channels {
		if ch.Type == "voice" {
			voiceChannelID = ch.ID
			break
		}
	}
	if voiceChannelID == "" {
		t.Fatal("expected at least one voice channel")
	}

	tokenBody := requestJSON(t, http.MethodPost, baseURL+"/api/livekit/token", map[string]string{
		"Authorization": "Bearer " + finish.SessionToken,
	}, liveKitTokenRequest{ChannelID: voiceChannelID}, http.StatusOK)

	var tokenResp liveKitTokenResponse
	mustParseJSON(t, tokenBody, &tokenResp)
	if strings.TrimSpace(tokenResp.Token) == "" {
		t.Fatal("expected non-empty livekit token")
	}
	if tokenResp.ChannelID != voiceChannelID {
		t.Fatalf("unexpected channel id in token response: got=%q want=%q", tokenResp.ChannelID, voiceChannelID)
	}
	if tokenResp.ParticipantID != session.ClientPublicKey {
		t.Fatalf("unexpected participant identity in token response: got=%q want=%q", tokenResp.ParticipantID, session.ClientPublicKey)
	}

	_ = requestJSON(t, http.MethodPost, baseURL+"/api/livekit/voice/touch", map[string]string{
		"Authorization": "Bearer " + finish.SessionToken,
	}, voiceTouchRequest{
		ChannelID:          voiceChannelID,
		AudioStreams:       2,
		VideoStreams:       1,
		CameraEnabled:      true,
		ScreenEnabled:      false,
		ScreenAudioEnabled: false,
	}, http.StatusOK)

	stateBody := requestJSON(t, http.MethodGet, baseURL+"/api/livekit/voice/channels/"+voiceChannelID+"/state", map[string]string{
		"Authorization": "Bearer " + finish.SessionToken,
	}, nil, http.StatusOK)

	var state voiceStateResponse
	mustParseJSON(t, stateBody, &state)
	if state.ChannelID != voiceChannelID {
		t.Fatalf("unexpected voice state channel id: got=%q want=%q", state.ChannelID, voiceChannelID)
	}
	if len(state.Participants) == 0 {
		t.Fatal("expected at least one participant in voice state")
	}

	found := false
	for _, participant := range state.Participants {
		if participant.PublicKey != session.ClientPublicKey {
			continue
		}
		found = true
		if participant.AudioStreams != 2 {
			t.Fatalf("unexpected audio streams: got=%d want=%d", participant.AudioStreams, 2)
		}
		if participant.VideoStreams != 1 {
			t.Fatalf("unexpected video streams: got=%d want=%d", participant.VideoStreams, 1)
		}
		if !participant.CameraEnabled {
			t.Fatal("expected cameraEnabled=true")
		}
	}
	if !found {
		t.Fatal("expected to find test participant in voice state")
	}
}

func createConnectedClientSession(t *testing.T, baseURL string) connectedSession {
	t.Helper()

	adminToken := adminToken()
	clientPublicB64, clientPrivate := generateClientKeypair(t)

	inviteBody := requestJSON(t, http.MethodPost, baseURL+"/api/admin/invites", map[string]string{
		"Authorization": "Bearer " + adminToken,
	}, createInviteRequest{ClientPublicKey: clientPublicB64, Label: "integration-chat"}, http.StatusOK)

	var invite createInviteResponse
	mustParseJSON(t, inviteBody, &invite)

	beginBody := requestJSON(t, http.MethodPost, baseURL+"/api/connect/begin", nil, connectBeginRequest{InviteID: invite.InviteID}, http.StatusOK)

	var begin connectBeginResponse
	mustParseJSON(t, beginBody, &begin)

	challengeRaw, err := base64.StdEncoding.DecodeString(begin.Challenge)
	if err != nil {
		t.Fatalf("invalid challenge encoding: %v", err)
	}

	hash := signaturePayloadHash(challengeRaw, invite.InviteID, begin.ServerFingerprint)
	signature := ed25519.Sign(clientPrivate, hash[:])

	finishReq := connectFinishRequest{
		InviteID:        invite.InviteID,
		ClientPublicKey: clientPublicB64,
		Challenge:       begin.Challenge,
		Signature:       base64.StdEncoding.EncodeToString(signature),
	}
	finishReq.ClientInfo.DisplayName = "integration-client"

	finishBody := requestJSON(t, http.MethodPost, baseURL+"/api/connect/finish", nil, finishReq, http.StatusOK)

	var finish connectFinishResponse
	mustParseJSON(t, finishBody, &finish)
	return connectedSession{
		Finish:          finish,
		ClientPublicKey: clientPublicB64,
	}
}

func verifyExpectedFingerprint(expected, actual string) error {
	if expected == actual {
		return nil
	}
	return errors.New("server fingerprint mismatch")
}

func signaturePayloadHash(challenge []byte, inviteID, serverFingerprint string) [32]byte {
	payload := make([]byte, 0, len(challenge)+len(inviteID)+len(serverFingerprint))
	payload = append(payload, challenge...)
	payload = append(payload, []byte(inviteID)...)
	payload = append(payload, []byte(serverFingerprint)...)
	return sha256.Sum256(payload)
}

func generateClientKeypair(t *testing.T) (string, ed25519.PrivateKey) {
	t.Helper()

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate client keypair: %v", err)
	}
	return base64.StdEncoding.EncodeToString(pub), priv
}

func mustParseJSON(t *testing.T, raw []byte, out any) {
	t.Helper()
	if err := json.Unmarshal(raw, out); err != nil {
		t.Fatalf("failed to parse JSON: %v\nbody=%s", err, string(raw))
	}
}

func requestJSON(t *testing.T, method, url string, headers map[string]string, body any, expectedStatus int) []byte {
	t.Helper()

	client := &http.Client{Timeout: 5 * time.Second}

	var requestBody io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to encode request body: %v", err)
		}
		requestBody = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, url, requestBody)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	responseBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != expectedStatus {
		t.Fatalf("unexpected status for %s %s: got=%d want=%d body=%s", method, url, resp.StatusCode, expectedStatus, string(responseBody))
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Fatalf("unexpected content-type for %s %s: %q body=%s", method, url, contentType, string(responseBody))
	}

	return responseBody
}

func apiBaseURL() string {
	if value := strings.TrimSpace(os.Getenv("API_BASE_URL")); value != "" {
		return strings.TrimRight(value, "/")
	}
	return "http://localhost:8080"
}

func adminToken() string {
	if value := strings.TrimSpace(os.Getenv("ADMIN_TOKEN")); value != "" {
		return value
	}
	return "devadmin"
}
