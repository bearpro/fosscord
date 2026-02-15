package serverstate

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	voicePresenceTTL    = 30 * time.Second
	voicePresenceMaxLag = 5 * time.Second
)

type VoiceParticipant struct {
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

type VoiceChannelState struct {
	ChannelID    string             `json:"channelId"`
	Participants []VoiceParticipant `json:"participants"`
}

type VoicePresenceUpdate struct {
	AudioStreams       int  `json:"audioStreams"`
	VideoStreams       int  `json:"videoStreams"`
	CameraEnabled      bool `json:"cameraEnabled"`
	ScreenEnabled      bool `json:"screenEnabled"`
	ScreenAudioEnabled bool `json:"screenAudioEnabled"`
}

type VoiceJoinContext struct {
	Identity  SessionIdentity
	ChannelID string
	RoomName  string
}

func (s *State) BeginVoiceJoin(sessionToken, channelID string) (VoiceJoinContext, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	identity, err := s.authenticateSessionLocked(sessionToken)
	if err != nil {
		return VoiceJoinContext{}, err
	}
	if err := s.ensureVoiceChannelLocked(channelID); err != nil {
		return VoiceJoinContext{}, err
	}

	if err := s.cleanupVoicePresenceLocked(); err != nil {
		return VoiceJoinContext{}, err
	}

	update := VoicePresenceUpdate{
		AudioStreams:       1,
		VideoStreams:       0,
		CameraEnabled:      false,
		ScreenEnabled:      false,
		ScreenAudioEnabled: false,
	}
	if err := s.upsertVoicePresenceLocked(identity, channelID, update); err != nil {
		return VoiceJoinContext{}, err
	}

	return VoiceJoinContext{
		Identity:  identity,
		ChannelID: channelID,
		RoomName:  VoiceRoomName(s.serverID, channelID),
	}, nil
}

func (s *State) TouchVoicePresence(sessionToken, channelID string, update VoicePresenceUpdate) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	identity, err := s.authenticateSessionLocked(sessionToken)
	if err != nil {
		return err
	}
	if err := s.ensureVoiceChannelLocked(channelID); err != nil {
		return err
	}

	if err := s.cleanupVoicePresenceLocked(); err != nil {
		return err
	}

	if err := s.upsertVoicePresenceLocked(identity, channelID, clampVoicePresenceUpdate(update)); err != nil {
		return err
	}
	return nil
}

func (s *State) LeaveVoiceChannel(sessionToken string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	identity, err := s.authenticateSessionLocked(sessionToken)
	if err != nil {
		return err
	}

	if err := s.cleanupVoicePresenceLocked(); err != nil {
		return err
	}

	if _, err := s.db.Exec(`DELETE FROM voice_presence WHERE client_public_key = ?`, identity.PublicKey); err != nil {
		return fmt.Errorf("delete voice presence: %w", err)
	}
	return nil
}

func (s *State) GetVoiceChannelState(sessionToken, channelID string) (VoiceChannelState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.authenticateSessionLocked(sessionToken); err != nil {
		return VoiceChannelState{}, err
	}
	if err := s.ensureVoiceChannelLocked(channelID); err != nil {
		return VoiceChannelState{}, err
	}

	if err := s.cleanupVoicePresenceLocked(); err != nil {
		return VoiceChannelState{}, err
	}

	rows, err := s.db.Query(`
		SELECT
			client_public_key,
			channel_id,
			display_name,
			joined_at,
			last_seen_at,
			audio_streams,
			video_streams,
			camera_enabled,
			screen_enabled,
			screen_audio_enabled
		FROM voice_presence
		WHERE channel_id = ?
		ORDER BY joined_at ASC
	`, channelID)
	if err != nil {
		return VoiceChannelState{}, fmt.Errorf("query voice presence: %w", err)
	}
	defer rows.Close()

	participants := make([]VoiceParticipant, 0, 8)
	for rows.Next() {
		participant, err := scanVoiceParticipant(rows)
		if err != nil {
			return VoiceChannelState{}, err
		}
		participants = append(participants, participant)
	}
	if err := rows.Err(); err != nil {
		return VoiceChannelState{}, fmt.Errorf("iterate voice presence rows: %w", err)
	}

	return VoiceChannelState{
		ChannelID:    channelID,
		Participants: participants,
	}, nil
}

func (s *State) ensureVoiceChannelLocked(channelID string) error {
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return newAPIError(400, "invalid_channel", "channel id is required")
	}

	for _, channel := range s.serverCfg.Channels {
		if channel.ID != channelID {
			continue
		}
		if channel.Type != "voice" {
			return newAPIError(400, "invalid_channel_type", "channel is not a voice channel")
		}
		return nil
	}

	return newAPIError(404, "channel_not_found", "channel does not exist")
}

func (s *State) cleanupVoicePresenceLocked() error {
	cutoff := time.Now().UTC().Add(-(voicePresenceTTL + voicePresenceMaxLag)).Format(time.RFC3339)
	if _, err := s.db.Exec(`DELETE FROM voice_presence WHERE last_seen_at < ?`, cutoff); err != nil {
		return fmt.Errorf("cleanup stale voice presence: %w", err)
	}
	return nil
}

func (s *State) upsertVoicePresenceLocked(identity SessionIdentity, channelID string, update VoicePresenceUpdate) error {
	now := time.Now().UTC().Format(time.RFC3339)

	if _, err := s.db.Exec(`
		INSERT INTO voice_presence(
			client_public_key,
			channel_id,
			display_name,
			joined_at,
			last_seen_at,
			audio_streams,
			video_streams,
			camera_enabled,
			screen_enabled,
			screen_audio_enabled
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(client_public_key) DO UPDATE SET
			channel_id = excluded.channel_id,
			display_name = excluded.display_name,
			last_seen_at = excluded.last_seen_at,
			audio_streams = excluded.audio_streams,
			video_streams = excluded.video_streams,
			camera_enabled = excluded.camera_enabled,
			screen_enabled = excluded.screen_enabled,
			screen_audio_enabled = excluded.screen_audio_enabled,
			joined_at = CASE
				WHEN voice_presence.channel_id = excluded.channel_id THEN voice_presence.joined_at
				ELSE excluded.joined_at
			END
	`,
		identity.PublicKey,
		channelID,
		identity.DisplayName,
		now,
		now,
		update.AudioStreams,
		update.VideoStreams,
		boolToInt(update.CameraEnabled),
		boolToInt(update.ScreenEnabled),
		boolToInt(update.ScreenAudioEnabled),
	); err != nil {
		return fmt.Errorf("upsert voice presence: %w", err)
	}

	return nil
}

func clampVoicePresenceUpdate(update VoicePresenceUpdate) VoicePresenceUpdate {
	if update.AudioStreams < 0 {
		update.AudioStreams = 0
	}
	if update.VideoStreams < 0 {
		update.VideoStreams = 0
	}
	if update.AudioStreams > 16 {
		update.AudioStreams = 16
	}
	if update.VideoStreams > 16 {
		update.VideoStreams = 16
	}
	return update
}

func VoiceRoomName(serverID, channelID string) string {
	return fmt.Sprintf("%s:%s", strings.TrimSpace(serverID), strings.TrimSpace(channelID))
}

func scanVoiceParticipant(scanner messageScanner) (VoiceParticipant, error) {
	var (
		participant        VoiceParticipant
		cameraEnabled      int
		screenEnabled      int
		screenAudioEnabled int
	)
	if err := scanner.Scan(
		&participant.PublicKey,
		&participant.ChannelID,
		&participant.DisplayName,
		&participant.JoinedAt,
		&participant.LastSeenAt,
		&participant.AudioStreams,
		&participant.VideoStreams,
		&cameraEnabled,
		&screenEnabled,
		&screenAudioEnabled,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return VoiceParticipant{}, newAPIError(404, "voice_state_not_found", "voice channel state is not available")
		}
		return VoiceParticipant{}, fmt.Errorf("scan voice participant row: %w", err)
	}

	participant.CameraEnabled = cameraEnabled != 0
	participant.ScreenEnabled = screenEnabled != 0
	participant.ScreenAudioEnabled = screenAudioEnabled != 0
	return participant, nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
