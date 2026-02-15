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
	ServerID                  string `json:"serverId"`
	Name                      string `json:"name"`
	PublicKeyFingerprintEmoji string `json:"publicKeyFingerprintEmoji"`
	ServerFingerprint         string `json:"serverFingerprint"`
	ServerPublicKey           string `json:"serverPublicKey"`
	LiveKitURL                string `json:"livekitUrl"`
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

type apiErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
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
