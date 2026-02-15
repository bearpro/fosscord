//go:build integration

package integration_test

import (
	"encoding/json"
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
	Name                      string `json:"name"`
	PublicKeyFingerprintEmoji string `json:"publicKeyFingerprintEmoji"`
	LiveKitURL                string `json:"livekitUrl"`
}

func TestHealth(t *testing.T) {
	t.Parallel()

	baseURL := apiBaseURL()
	body := requestJSON(t, http.MethodGet, baseURL+"/health", http.StatusOK)

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
	body := requestJSON(t, http.MethodGet, baseURL+"/api/server-info", http.StatusOK)

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
	if strings.TrimSpace(parsed.LiveKitURL) == "" {
		t.Fatal("expected non-empty 'livekitUrl'")
	}
}

func requestJSON(t *testing.T, method, url string, expectedStatus int) []byte {
	t.Helper()

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != expectedStatus {
		t.Fatalf("unexpected status for %s %s: got=%d want=%d body=%s", method, url, resp.StatusCode, expectedStatus, string(body))
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Fatalf("unexpected content-type for %s %s: %q body=%s", method, url, contentType, string(body))
	}

	return body
}

func apiBaseURL() string {
	if value := strings.TrimSpace(os.Getenv("API_BASE_URL")); value != "" {
		return strings.TrimRight(value, "/")
	}
	return "http://localhost:8080"
}
