package serverstate

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	defaultMessageHistoryLimit = 100
	maxMessageHistoryLimit     = 100
	maxMessageLength           = 4000
)

type SessionIdentity struct {
	PublicKey   string
	DisplayName string
}

type MessageAuthor struct {
	DisplayName string `json:"displayName"`
	PublicKey   string `json:"publicKey"`
}

type ChannelMessage struct {
	ID              string        `json:"id"`
	ChannelID       string        `json:"channelId"`
	Author          MessageAuthor `json:"author"`
	ContentMarkdown string        `json:"contentMarkdown"`
	CreatedAt       string        `json:"createdAt"`
	UpdatedAt       string        `json:"updatedAt"`
}

type ListMessagesResult struct {
	Messages []ChannelMessage `json:"messages"`
}

type ChannelEvent struct {
	Type    string          `json:"type"`
	Message *ChannelMessage `json:"message,omitempty"`
}

func (s *State) AuthenticateSession(token string) (SessionIdentity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.authenticateSessionLocked(token)
}

func (s *State) authenticateSessionLocked(token string) (SessionIdentity, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return SessionIdentity{}, newAPIError(401, "missing_session_token", "session token is required")
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.db.Exec(`DELETE FROM sessions WHERE expires_at <= ?`, now); err != nil {
		return SessionIdentity{}, fmt.Errorf("clean expired sessions: %w", err)
	}

	var identity SessionIdentity
	var expiresAt string
	err := s.db.QueryRow(`
		SELECT s.client_public_key, m.display_name, s.expires_at
		FROM sessions s
		JOIN members m ON m.public_key = s.client_public_key
		WHERE s.token = ?
	`, token).Scan(&identity.PublicKey, &identity.DisplayName, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return SessionIdentity{}, newAPIError(401, "invalid_session_token", "session token is invalid or expired")
	}
	if err != nil {
		return SessionIdentity{}, fmt.Errorf("query session: %w", err)
	}

	if expiresAt <= now {
		if _, err := s.db.Exec(`DELETE FROM sessions WHERE token = ?`, token); err != nil {
			return SessionIdentity{}, fmt.Errorf("delete expired session: %w", err)
		}
		return SessionIdentity{}, newAPIError(401, "invalid_session_token", "session token is invalid or expired")
	}

	return identity, nil
}

func (s *State) ListMessages(sessionToken, channelID string, limit int) (ListMessagesResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.authenticateSessionLocked(sessionToken); err != nil {
		return ListMessagesResult{}, err
	}
	if err := s.ensureTextChannelLocked(channelID); err != nil {
		return ListMessagesResult{}, err
	}

	if limit <= 0 || limit > maxMessageHistoryLimit {
		limit = defaultMessageHistoryLimit
	}

	rows, err := s.db.Query(`
		SELECT id, channel_id, author_public_key, author_name, content_markdown, created_at, updated_at
		FROM messages
		WHERE channel_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`, channelID, limit)
	if err != nil {
		return ListMessagesResult{}, fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()

	desc := make([]ChannelMessage, 0, limit)
	for rows.Next() {
		message, err := scanMessageRow(rows)
		if err != nil {
			return ListMessagesResult{}, err
		}
		desc = append(desc, message)
	}
	if err := rows.Err(); err != nil {
		return ListMessagesResult{}, fmt.Errorf("iterate message rows: %w", err)
	}

	messages := make([]ChannelMessage, 0, len(desc))
	for i := len(desc) - 1; i >= 0; i-- {
		messages = append(messages, desc[i])
	}

	return ListMessagesResult{Messages: messages}, nil
}

func (s *State) CreateMessage(sessionToken, channelID, contentMarkdown string) (ChannelMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	identity, err := s.authenticateSessionLocked(sessionToken)
	if err != nil {
		return ChannelMessage{}, err
	}
	if err := s.ensureTextChannelLocked(channelID); err != nil {
		return ChannelMessage{}, err
	}

	content, err := normalizeMessageContent(contentMarkdown)
	if err != nil {
		return ChannelMessage{}, err
	}

	messageID, err := randomHex(16)
	if err != nil {
		return ChannelMessage{}, fmt.Errorf("generate message id: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.db.Exec(`
		INSERT INTO messages(id, channel_id, author_public_key, author_name, content_markdown, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, messageID, channelID, identity.PublicKey, identity.DisplayName, content, now, now); err != nil {
		return ChannelMessage{}, fmt.Errorf("insert message: %w", err)
	}

	message := ChannelMessage{
		ID:        messageID,
		ChannelID: channelID,
		Author: MessageAuthor{
			DisplayName: identity.DisplayName,
			PublicKey:   identity.PublicKey,
		},
		ContentMarkdown: content,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	s.broadcastChannelEventLocked(channelID, ChannelEvent{
		Type:    "message.created",
		Message: &message,
	})

	return message, nil
}

func (s *State) EditMessage(sessionToken, channelID, messageID, contentMarkdown string) (ChannelMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.authenticateSessionLocked(sessionToken); err != nil {
		return ChannelMessage{}, err
	}
	if err := s.ensureTextChannelLocked(channelID); err != nil {
		return ChannelMessage{}, err
	}

	content, err := normalizeMessageContent(contentMarkdown)
	if err != nil {
		return ChannelMessage{}, err
	}

	existing, err := s.findMessageLocked(channelID, messageID)
	if err != nil {
		return ChannelMessage{}, err
	}

	updatedAt := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.db.Exec(`
		UPDATE messages
		SET content_markdown = ?, updated_at = ?
		WHERE id = ? AND channel_id = ?
	`, content, updatedAt, messageID, channelID); err != nil {
		return ChannelMessage{}, fmt.Errorf("update message: %w", err)
	}

	updated := existing
	updated.ContentMarkdown = content
	updated.UpdatedAt = updatedAt

	s.broadcastChannelEventLocked(channelID, ChannelEvent{
		Type:    "message.updated",
		Message: &updated,
	})

	return updated, nil
}

func (s *State) SubscribeChannelEvents(sessionToken, channelID string) (<-chan ChannelEvent, func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.authenticateSessionLocked(sessionToken); err != nil {
		return nil, nil, err
	}
	if err := s.ensureTextChannelLocked(channelID); err != nil {
		return nil, nil, err
	}

	if _, exists := s.streams[channelID]; !exists {
		s.streams[channelID] = make(map[int]chan ChannelEvent)
	}

	s.nextStream++
	streamID := s.nextStream
	stream := make(chan ChannelEvent, 32)
	s.streams[channelID][streamID] = stream

	cancel := func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		channelStreams, exists := s.streams[channelID]
		if !exists {
			return
		}

		ch, ok := channelStreams[streamID]
		if !ok {
			return
		}
		delete(channelStreams, streamID)
		close(ch)
		if len(channelStreams) == 0 {
			delete(s.streams, channelID)
		}
	}

	return stream, cancel, nil
}

func (s *State) broadcastChannelEventLocked(channelID string, event ChannelEvent) {
	channelStreams, exists := s.streams[channelID]
	if !exists {
		return
	}

	for _, stream := range channelStreams {
		select {
		case stream <- event:
		default:
		}
	}
}

func (s *State) ensureTextChannelLocked(channelID string) error {
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return newAPIError(400, "invalid_channel", "channel id is required")
	}

	for _, channel := range s.serverCfg.Channels {
		if channel.ID != channelID {
			continue
		}
		if channel.Type != "text" {
			return newAPIError(400, "invalid_channel_type", "channel is not a text channel")
		}
		return nil
	}

	return newAPIError(404, "channel_not_found", "channel does not exist")
}

func normalizeMessageContent(contentMarkdown string) (string, error) {
	content := strings.TrimSpace(contentMarkdown)
	if content == "" {
		return "", newAPIError(400, "invalid_message", "message content cannot be empty")
	}
	if len(content) > maxMessageLength {
		return "", newAPIError(400, "invalid_message", "message content exceeds maximum length")
	}
	return content, nil
}

func (s *State) findMessageLocked(channelID, messageID string) (ChannelMessage, error) {
	row := s.db.QueryRow(`
		SELECT id, channel_id, author_public_key, author_name, content_markdown, created_at, updated_at
		FROM messages
		WHERE id = ? AND channel_id = ?
	`, messageID, channelID)

	message, err := scanMessageRow(row)
	if err != nil {
		return ChannelMessage{}, err
	}
	return message, nil
}

type messageScanner interface {
	Scan(dest ...any) error
}

func scanMessageRow(scanner messageScanner) (ChannelMessage, error) {
	var (
		messageID    string
		channelID    string
		authorPublic string
		authorName   string
		content      string
		createdAt    string
		updatedAt    string
	)

	if err := scanner.Scan(&messageID, &channelID, &authorPublic, &authorName, &content, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ChannelMessage{}, newAPIError(404, "message_not_found", "message does not exist")
		}
		return ChannelMessage{}, fmt.Errorf("scan message row: %w", err)
	}

	return ChannelMessage{
		ID:        messageID,
		ChannelID: channelID,
		Author: MessageAuthor{
			DisplayName: authorName,
			PublicKey:   authorPublic,
		},
		ContentMarkdown: content,
		CreatedAt:       createdAt,
		UpdatedAt:       updatedAt,
	}, nil
}

func (s *State) upsertMemberLocked(publicKey, displayName string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.db.Exec(`
		INSERT INTO members(public_key, display_name, first_connected_at, last_connected_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(public_key) DO UPDATE SET
			display_name = excluded.display_name,
			last_connected_at = excluded.last_connected_at
	`, publicKey, displayName, now, now); err != nil {
		return fmt.Errorf("upsert member: %w", err)
	}
	return nil
}

func (s *State) issueSessionTokenLocked(publicKey string) (string, error) {
	now := time.Now().UTC()
	token, err := randomHex(32)
	if err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}

	if _, err := s.db.Exec(`
		INSERT INTO sessions(token, client_public_key, created_at, expires_at)
		VALUES (?, ?, ?, ?)
	`, token, publicKey, now.Format(time.RFC3339), now.Add(sessionTTL).Format(time.RFC3339)); err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}

	return token, nil
}

func normalizeDisplayName(displayName, publicKey string) string {
	name := strings.TrimSpace(displayName)
	if name != "" {
		return name
	}

	shortKey := strings.TrimSpace(publicKey)
	if len(shortKey) > 8 {
		shortKey = shortKey[:8]
	}
	if shortKey == "" {
		shortKey = "unknown"
	}
	return "User " + shortKey
}
