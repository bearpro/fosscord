package serverstate

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"fosscord/apps/server/internal/config"
	_ "modernc.org/sqlite"
)

const (
	challengeTTL        = 2 * time.Minute
	adminRequestMaxSkew = 2 * time.Minute
	sessionTTL          = 30 * 24 * time.Hour
)

type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string {
	return e.Message
}

func newAPIError(status int, code, message string) *APIError {
	return &APIError{Status: status, Code: code, Message: message}
}

type Channel struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Name string `json:"name"`
}

type ServerInfo struct {
	ServerID          string   `json:"serverId"`
	Name              string   `json:"name"`
	ServerFingerprint string   `json:"serverFingerprint"`
	ServerPublicKey   string   `json:"serverPublicKey"`
	LiveKitURL        string   `json:"livekitUrl"`
	AdminPublicKeys   []string `json:"adminPublicKeys"`
}

type CreateInviteResult struct {
	InviteID          string `json:"inviteId"`
	ServerBaseURL     string `json:"serverBaseUrl"`
	ServerFingerprint string `json:"serverFingerprint"`
	InviteLink        string `json:"inviteLink"`
}

type CreateInviteByAdminClientRequest struct {
	AdminPublicKey  string
	ClientPublicKey string
	Label           string
	IssuedAt        string
	Signature       string
}

type ListInvitesByAdminClientRequest struct {
	AdminPublicKey string
	IssuedAt       string
	Signature      string
}

type InviteSummary struct {
	InviteID               string  `json:"inviteId"`
	AllowedClientPublicKey string  `json:"allowedClientPublicKey"`
	Label                  string  `json:"label"`
	CreatedAt              string  `json:"createdAt"`
	UsedAt                 *string `json:"usedAt,omitempty"`
	Status                 string  `json:"status"`
}

type ListInvitesResult struct {
	Invites []InviteSummary `json:"invites"`
}

type BeginResult struct {
	ServerPublicKey   string    `json:"serverPublicKey"`
	ServerFingerprint string    `json:"serverFingerprint"`
	Challenge         string    `json:"challenge"`
	ExpiresAt         time.Time `json:"expiresAt"`
}

type ClientInfo struct {
	DisplayName string `json:"displayName"`
}

type FinishRequest struct {
	InviteID        string     `json:"inviteId"`
	ClientPublicKey string     `json:"clientPublicKey"`
	Challenge       string     `json:"challenge"`
	Signature       string     `json:"signature"`
	ClientInfo      ClientInfo `json:"clientInfo"`
}

type FinishResult struct {
	ServerID          string    `json:"serverId"`
	ServerName        string    `json:"serverName"`
	ServerFingerprint string    `json:"serverFingerprint"`
	LiveKitURL        string    `json:"livekitUrl"`
	Channels          []Channel `json:"channels"`
	SessionToken      string    `json:"sessionToken,omitempty"`
}

type State struct {
	cfg config.Config

	mu         sync.Mutex
	db         *sql.DB
	serverCfg  serverConfigFile
	challenges map[string]pendingChallenge
	streams    map[string]map[int]chan ChannelEvent
	nextStream int

	serverID          string
	serverFingerprint string
	serverPublicKey   string
}

type identityRecord struct {
	PublicKey  string
	PrivateKey string
}

type serverConfigFile struct {
	ServerName      string    `json:"serverName"`
	Channels        []Channel `json:"channels"`
	AdminPublicKeys []string  `json:"adminPublicKeys"`
}

type inviteRecord struct {
	ID                     string
	AllowedClientPublicKey string
	Label                  string
	CreatedAt              string
	UsedAt                 *string
}

type pendingChallenge struct {
	Challenge string
	ExpiresAt time.Time
}

func New(cfg config.Config) (*State, error) {
	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	databasePath := resolveDatabasePath(cfg)
	if err := os.MkdirAll(filepath.Dir(databasePath), 0o700); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(`PRAGMA busy_timeout = 5000;`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set sqlite busy_timeout: %w", err)
	}

	if err := applyMigrations(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("apply migrations: %w", err)
	}

	serverCfg, err := loadOrCreateServerConfig(filepath.Join(cfg.DataDir, "server_config.json"), cfg.ServerName)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	identity, err := loadOrCreateIdentity(db)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	pub, err := decodePublicKey(identity.PublicKey)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	return &State{
		cfg:               cfg,
		db:                db,
		serverCfg:         serverCfg,
		challenges:        make(map[string]pendingChallenge),
		streams:           make(map[string]map[int]chan ChannelEvent),
		serverID:          stableServerID(pub),
		serverFingerprint: FingerprintFromPublicKey(pub),
		serverPublicKey:   base64.StdEncoding.EncodeToString(pub),
	}, nil
}

func (s *State) ServerInfo() ServerInfo {
	s.mu.Lock()
	defer s.mu.Unlock()

	admins := make([]string, len(s.serverCfg.AdminPublicKeys))
	copy(admins, s.serverCfg.AdminPublicKeys)

	return ServerInfo{
		ServerID:          s.serverID,
		Name:              s.serverCfg.ServerName,
		ServerFingerprint: s.serverFingerprint,
		ServerPublicKey:   s.serverPublicKey,
		LiveKitURL:        s.cfg.LiveKitURL,
		AdminPublicKeys:   admins,
	}
}

func (s *State) Channels() []Channel {
	s.mu.Lock()
	defer s.mu.Unlock()

	channels := make([]Channel, len(s.serverCfg.Channels))
	copy(channels, s.serverCfg.Channels)
	return channels
}

func (s *State) CreateInvite(clientPublicKeyB64, label string) (CreateInviteResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := decodePublicKey(clientPublicKeyB64); err != nil {
		return CreateInviteResult{}, newAPIError(400, "invalid_client_public_key", "clientPublicKey must be base64(ed25519 public key)")
	}

	return s.createInviteLocked(clientPublicKeyB64, label)
}

func (s *State) CreateInviteByAdminClient(req CreateInviteByAdminClientRequest) (CreateInviteResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	req.AdminPublicKey = strings.TrimSpace(req.AdminPublicKey)
	req.ClientPublicKey = strings.TrimSpace(req.ClientPublicKey)
	req.IssuedAt = strings.TrimSpace(req.IssuedAt)
	req.Signature = strings.TrimSpace(req.Signature)

	if req.AdminPublicKey == "" || req.ClientPublicKey == "" || req.IssuedAt == "" || req.Signature == "" {
		return CreateInviteResult{}, newAPIError(400, "invalid_request", "adminPublicKey, clientPublicKey, issuedAt and signature are required")
	}

	adminKey, err := decodePublicKey(req.AdminPublicKey)
	if err != nil {
		return CreateInviteResult{}, newAPIError(400, "invalid_admin_public_key", "adminPublicKey must be base64(ed25519 public key)")
	}
	if _, err := decodePublicKey(req.ClientPublicKey); err != nil {
		return CreateInviteResult{}, newAPIError(400, "invalid_client_public_key", "clientPublicKey must be base64(ed25519 public key)")
	}

	if !s.isAdminPublicKeyLocked(req.AdminPublicKey) {
		return CreateInviteResult{}, newAPIError(403, "admin_forbidden", "client is not an administrator")
	}

	issuedAt, err := time.Parse(time.RFC3339, req.IssuedAt)
	if err != nil {
		return CreateInviteResult{}, newAPIError(400, "invalid_issued_at", "issuedAt must be RFC3339")
	}
	if time.Since(issuedAt.UTC()) > adminRequestMaxSkew || time.Until(issuedAt.UTC()) > adminRequestMaxSkew {
		return CreateInviteResult{}, newAPIError(401, "stale_request", "issuedAt is outside allowed skew")
	}

	signature, err := decodeSignature(req.Signature)
	if err != nil {
		return CreateInviteResult{}, newAPIError(400, "invalid_signature", "signature must be base64(ed25519 signature)")
	}

	hash := AdminInvitePayloadHash(req.AdminPublicKey, req.ClientPublicKey, req.IssuedAt)
	if !ed25519.Verify(adminKey, hash[:], signature) {
		return CreateInviteResult{}, newAPIError(401, "invalid_signature", "signature verification failed")
	}

	return s.createInviteLocked(req.ClientPublicKey, req.Label)
}

func (s *State) ListInvitesByAdminClient(req ListInvitesByAdminClientRequest) (ListInvitesResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	req.AdminPublicKey = strings.TrimSpace(req.AdminPublicKey)
	req.IssuedAt = strings.TrimSpace(req.IssuedAt)
	req.Signature = strings.TrimSpace(req.Signature)

	if req.AdminPublicKey == "" || req.IssuedAt == "" || req.Signature == "" {
		return ListInvitesResult{}, newAPIError(400, "invalid_request", "adminPublicKey, issuedAt and signature are required")
	}

	adminKey, err := decodePublicKey(req.AdminPublicKey)
	if err != nil {
		return ListInvitesResult{}, newAPIError(400, "invalid_admin_public_key", "adminPublicKey must be base64(ed25519 public key)")
	}
	if !s.isAdminPublicKeyLocked(req.AdminPublicKey) {
		return ListInvitesResult{}, newAPIError(403, "admin_forbidden", "client is not an administrator")
	}

	issuedAt, err := time.Parse(time.RFC3339, req.IssuedAt)
	if err != nil {
		return ListInvitesResult{}, newAPIError(400, "invalid_issued_at", "issuedAt must be RFC3339")
	}
	if time.Since(issuedAt.UTC()) > adminRequestMaxSkew || time.Until(issuedAt.UTC()) > adminRequestMaxSkew {
		return ListInvitesResult{}, newAPIError(401, "stale_request", "issuedAt is outside allowed skew")
	}

	signature, err := decodeSignature(req.Signature)
	if err != nil {
		return ListInvitesResult{}, newAPIError(400, "invalid_signature", "signature must be base64(ed25519 signature)")
	}

	hash := AdminListInvitesPayloadHash(req.AdminPublicKey, req.IssuedAt)
	if !ed25519.Verify(adminKey, hash[:], signature) {
		return ListInvitesResult{}, newAPIError(401, "invalid_signature", "signature verification failed")
	}

	rows, err := s.db.Query(`SELECT id, allowed_client_public_key, label, created_at, used_at FROM invites ORDER BY created_at DESC`)
	if err != nil {
		return ListInvitesResult{}, fmt.Errorf("query invites list: %w", err)
	}
	defer rows.Close()

	result := ListInvitesResult{
		Invites: []InviteSummary{},
	}

	for rows.Next() {
		var (
			inviteID         string
			allowedClientKey string
			label            string
			createdAt        string
			usedAt           sql.NullString
			usedAtPointer    *string
			status           = "active"
		)

		if err := rows.Scan(&inviteID, &allowedClientKey, &label, &createdAt, &usedAt); err != nil {
			return ListInvitesResult{}, fmt.Errorf("scan invites list row: %w", err)
		}

		if usedAt.Valid {
			usedAtCopy := usedAt.String
			usedAtPointer = &usedAtCopy
			status = "used"
		}

		result.Invites = append(result.Invites, InviteSummary{
			InviteID:               inviteID,
			AllowedClientPublicKey: allowedClientKey,
			Label:                  label,
			CreatedAt:              createdAt,
			UsedAt:                 usedAtPointer,
			Status:                 status,
		})
	}

	if err := rows.Err(); err != nil {
		return ListInvitesResult{}, fmt.Errorf("iterate invites list rows: %w", err)
	}

	return result, nil
}

func (s *State) createInviteLocked(clientPublicKeyB64, label string) (CreateInviteResult, error) {
	inviteID, err := randomHex(16)
	if err != nil {
		return CreateInviteResult{}, fmt.Errorf("generate invite id: %w", err)
	}

	createdAt := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.db.Exec(
		`INSERT INTO invites(id, allowed_client_public_key, label, created_at) VALUES (?, ?, ?, ?)`,
		inviteID,
		clientPublicKeyB64,
		strings.TrimSpace(label),
		createdAt,
	); err != nil {
		return CreateInviteResult{}, fmt.Errorf("persist invite: %w", err)
	}

	serverBaseURL := strings.TrimRight(s.cfg.ServerPublicBaseURL, "/")
	params := url.Values{}
	params.Set("baseUrl", serverBaseURL)
	params.Set("inviteId", inviteID)
	params.Set("serverFp", s.serverFingerprint)

	return CreateInviteResult{
		InviteID:          inviteID,
		ServerBaseURL:     serverBaseURL,
		ServerFingerprint: s.serverFingerprint,
		InviteLink:        "fw://connect?" + params.Encode(),
	}, nil
}

func (s *State) BeginConnect(inviteID string) (BeginResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	inviteID = strings.TrimSpace(inviteID)
	if inviteID == "" {
		return BeginResult{}, newAPIError(400, "invalid_invite", "inviteId is required")
	}

	invite, err := s.lookupInvite(inviteID)
	if err != nil {
		return BeginResult{}, err
	}
	if invite.UsedAt != nil {
		return BeginResult{}, newAPIError(403, "invite_used", "invite has already been used")
	}

	challengeRaw := make([]byte, 32)
	if _, err := rand.Read(challengeRaw); err != nil {
		return BeginResult{}, fmt.Errorf("generate challenge: %w", err)
	}

	challenge := base64.StdEncoding.EncodeToString(challengeRaw)
	expiresAt := time.Now().UTC().Add(challengeTTL)
	s.challenges[inviteID] = pendingChallenge{
		Challenge: challenge,
		ExpiresAt: expiresAt,
	}

	return BeginResult{
		ServerPublicKey:   s.serverPublicKey,
		ServerFingerprint: s.serverFingerprint,
		Challenge:         challenge,
		ExpiresAt:         expiresAt,
	}, nil
}

func (s *State) FinishConnect(req FinishRequest) (FinishResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	req.InviteID = strings.TrimSpace(req.InviteID)
	if req.InviteID == "" {
		return FinishResult{}, newAPIError(400, "invalid_request", "inviteId is required")
	}
	if strings.TrimSpace(req.ClientPublicKey) == "" || strings.TrimSpace(req.Challenge) == "" || strings.TrimSpace(req.Signature) == "" {
		return FinishResult{}, newAPIError(400, "invalid_request", "clientPublicKey, challenge and signature are required")
	}

	invite, err := s.lookupInvite(req.InviteID)
	if err != nil {
		return FinishResult{}, err
	}
	if invite.UsedAt != nil {
		return FinishResult{}, newAPIError(403, "invite_used", "invite has already been used")
	}
	if req.ClientPublicKey != invite.AllowedClientPublicKey {
		return FinishResult{}, newAPIError(403, "client_not_allowed", "client public key is not allowed for this invite")
	}

	challenge, ok := s.challenges[req.InviteID]
	if !ok {
		return FinishResult{}, newAPIError(401, "challenge_missing", "challenge not initialized")
	}
	if time.Now().UTC().After(challenge.ExpiresAt) {
		delete(s.challenges, req.InviteID)
		return FinishResult{}, newAPIError(401, "challenge_expired", "challenge has expired")
	}
	if req.Challenge != challenge.Challenge {
		return FinishResult{}, newAPIError(401, "challenge_mismatch", "challenge mismatch")
	}

	clientPublicKey, err := decodePublicKey(req.ClientPublicKey)
	if err != nil {
		return FinishResult{}, newAPIError(400, "invalid_client_public_key", "clientPublicKey must be base64(ed25519 public key)")
	}

	signature, err := decodeSignature(req.Signature)
	if err != nil {
		return FinishResult{}, newAPIError(400, "invalid_signature", "signature must be base64(ed25519 signature)")
	}

	challengeBytes, err := base64.StdEncoding.DecodeString(req.Challenge)
	if err != nil {
		return FinishResult{}, newAPIError(400, "invalid_challenge", "challenge must be base64")
	}

	hash := SignaturePayloadHash(challengeBytes, req.InviteID, s.serverFingerprint)
	if !ed25519.Verify(clientPublicKey, hash[:], signature) {
		return FinishResult{}, newAPIError(401, "invalid_signature", "signature verification failed")
	}

	usedAt := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.Exec(`UPDATE invites SET used_at = ? WHERE id = ? AND used_at IS NULL`, usedAt, req.InviteID)
	if err != nil {
		return FinishResult{}, fmt.Errorf("mark invite as used: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return FinishResult{}, fmt.Errorf("check invite update result: %w", err)
	}
	if rowsAffected == 0 {
		return FinishResult{}, newAPIError(403, "invite_used", "invite has already been used")
	}

	delete(s.challenges, req.InviteID)

	channels := make([]Channel, len(s.serverCfg.Channels))
	copy(channels, s.serverCfg.Channels)

	displayName := normalizeDisplayName(req.ClientInfo.DisplayName, req.ClientPublicKey)
	if err := s.upsertMemberLocked(req.ClientPublicKey, displayName); err != nil {
		return FinishResult{}, err
	}

	sessionToken, err := s.issueSessionTokenLocked(req.ClientPublicKey)
	if err != nil {
		return FinishResult{}, err
	}

	return FinishResult{
		ServerID:          s.serverID,
		ServerName:        s.serverCfg.ServerName,
		ServerFingerprint: s.serverFingerprint,
		LiveKitURL:        s.cfg.LiveKitURL,
		Channels:          channels,
		SessionToken:      sessionToken,
	}, nil
}

func (s *State) lookupInvite(inviteID string) (inviteRecord, error) {
	var invite inviteRecord
	var usedAt sql.NullString

	err := s.db.QueryRow(
		`SELECT id, allowed_client_public_key, label, created_at, used_at FROM invites WHERE id = ?`,
		inviteID,
	).Scan(
		&invite.ID,
		&invite.AllowedClientPublicKey,
		&invite.Label,
		&invite.CreatedAt,
		&usedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return inviteRecord{}, newAPIError(404, "invite_not_found", "invite does not exist")
	}
	if err != nil {
		return inviteRecord{}, fmt.Errorf("query invite: %w", err)
	}

	if usedAt.Valid {
		invite.UsedAt = &usedAt.String
	}
	return invite, nil
}

func (s *State) isAdminPublicKeyLocked(publicKey string) bool {
	for _, admin := range s.serverCfg.AdminPublicKeys {
		if admin == publicKey {
			return true
		}
	}
	return false
}

func SignaturePayloadHash(challenge []byte, inviteID, serverFingerprint string) [32]byte {
	payload := make([]byte, 0, len(challenge)+len(inviteID)+len(serverFingerprint))
	payload = append(payload, challenge...)
	payload = append(payload, []byte(inviteID)...)
	payload = append(payload, []byte(serverFingerprint)...)
	return sha256.Sum256(payload)
}

func AdminInvitePayloadHash(adminPublicKey, clientPublicKey, issuedAt string) [32]byte {
	payload := make([]byte, 0, len(adminPublicKey)+len(clientPublicKey)+len(issuedAt))
	payload = append(payload, []byte(adminPublicKey)...)
	payload = append(payload, []byte(clientPublicKey)...)
	payload = append(payload, []byte(issuedAt)...)
	return sha256.Sum256(payload)
}

func AdminListInvitesPayloadHash(adminPublicKey, issuedAt string) [32]byte {
	payload := make([]byte, 0, len(adminPublicKey)+len(issuedAt))
	payload = append(payload, []byte(adminPublicKey)...)
	payload = append(payload, []byte(issuedAt)...)
	return sha256.Sum256(payload)
}

func FingerprintFromPublicKey(publicKey []byte) string {
	hash := sha256.Sum256(publicKey)
	parts := make([]string, 4)
	for i := 0; i < 4; i++ {
		parts[i] = fingerprintEmojis[int(hash[i])%len(fingerprintEmojis)]
	}
	return strings.Join(parts, "")
}

func stableServerID(publicKey []byte) string {
	hash := sha256.Sum256(publicKey)
	return "srv-" + hex.EncodeToString(hash[:8])
}

func resolveDatabasePath(cfg config.Config) string {
	raw := strings.TrimSpace(cfg.DatabasePath)
	if raw == "" {
		return filepath.Join(cfg.DataDir, "server.db")
	}
	if filepath.IsAbs(raw) {
		return raw
	}
	return filepath.Join(cfg.DataDir, raw)
}

func loadOrCreateIdentity(db *sql.DB) (identityRecord, error) {
	var identity identityRecord

	err := db.QueryRow(`SELECT public_key, private_key FROM server_identity WHERE id = 1`).Scan(&identity.PublicKey, &identity.PrivateKey)
	if errors.Is(err, sql.ErrNoRows) {
		publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return identityRecord{}, fmt.Errorf("generate server identity: %w", err)
		}

		identity = identityRecord{
			PublicKey:  base64.StdEncoding.EncodeToString(publicKey),
			PrivateKey: base64.StdEncoding.EncodeToString(privateKey),
		}

		if _, err := db.Exec(
			`INSERT INTO server_identity(id, public_key, private_key, created_at) VALUES (1, ?, ?, ?)`,
			identity.PublicKey,
			identity.PrivateKey,
			time.Now().UTC().Format(time.RFC3339),
		); err != nil {
			return identityRecord{}, fmt.Errorf("persist server identity: %w", err)
		}

		return identity, nil
	}
	if err != nil {
		return identityRecord{}, fmt.Errorf("load server identity: %w", err)
	}

	if _, err := decodePublicKey(identity.PublicKey); err != nil {
		return identityRecord{}, fmt.Errorf("invalid persisted server public key: %w", err)
	}
	privateKey, err := decodePrivateKey(identity.PrivateKey)
	if err != nil {
		return identityRecord{}, fmt.Errorf("invalid persisted server private key: %w", err)
	}
	if pub := privateKey.Public().(ed25519.PublicKey); base64.StdEncoding.EncodeToString(pub) != identity.PublicKey {
		return identityRecord{}, errors.New("server identity keypair mismatch")
	}

	return identity, nil
}

func loadOrCreateServerConfig(path, defaultServerName string) (serverConfigFile, error) {
	if fileExists(path) {
		var cfg serverConfigFile
		if err := readJSON(path, &cfg); err != nil {
			return serverConfigFile{}, fmt.Errorf("load server config: %w", err)
		}
		if strings.TrimSpace(cfg.ServerName) == "" {
			return serverConfigFile{}, errors.New("server config has empty serverName")
		}
		if len(cfg.Channels) == 0 {
			return serverConfigFile{}, errors.New("server config has no channels")
		}
		admins, err := normalizePublicKeys(cfg.AdminPublicKeys)
		if err != nil {
			return serverConfigFile{}, fmt.Errorf("invalid adminPublicKeys in server config: %w", err)
		}
		cfg.AdminPublicKeys = admins
		return cfg, nil
	}

	cfg := serverConfigFile{
		ServerName: strings.TrimSpace(defaultServerName),
		Channels: []Channel{
			{ID: "general", Type: "text", Name: "general"},
			{ID: "voice-main", Type: "voice", Name: "Voice"},
			{ID: "voice-afk", Type: "voice", Name: "AFK"},
		},
		AdminPublicKeys: []string{},
	}

	if cfg.ServerName == "" {
		cfg.ServerName = "Local Server"
	}

	if err := writeJSON(path, cfg, 0o600); err != nil {
		return serverConfigFile{}, fmt.Errorf("persist server config: %w", err)
	}

	return cfg, nil
}

func normalizePublicKeys(values []string) ([]string, error) {
	unique := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, err := decodePublicKey(value); err != nil {
			return nil, err
		}
		if _, exists := unique[value]; exists {
			continue
		}
		unique[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result, nil
}

func readJSON(path string, out any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return err
	}
	return nil
}

func writeJSON(path string, value any, mode os.FileMode) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, mode); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func decodePublicKey(value string) (ed25519.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, err
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("expected %d bytes, got %d", ed25519.PublicKeySize, len(raw))
	}
	return ed25519.PublicKey(raw), nil
}

func decodePrivateKey(value string) (ed25519.PrivateKey, error) {
	raw, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, err
	}
	if len(raw) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("expected %d bytes, got %d", ed25519.PrivateKeySize, len(raw))
	}
	return ed25519.PrivateKey(raw), nil
}

func decodeSignature(value string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, err
	}
	if len(raw) != ed25519.SignatureSize {
		return nil, fmt.Errorf("expected %d bytes, got %d", ed25519.SignatureSize, len(raw))
	}
	return raw, nil
}

func randomHex(size int) (string, error) {
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

var fingerprintEmojis = []string{
	"😀", "😎", "🚀", "🌈", "🔥", "🧩", "🎯", "🎧",
	"🛰️", "🛡️", "🌊", "🍀", "🧠", "🌙", "⚡", "🧭",
	"🧱", "🪐", "🍉", "🎲", "🎹", "📡", "🧪", "🐙",
	"🦊", "🦉", "🐳", "🪁", "🏔️", "🌵", "🍄", "🍓",
}
