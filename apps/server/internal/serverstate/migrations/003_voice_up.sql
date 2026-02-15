CREATE TABLE IF NOT EXISTS voice_presence (
  client_public_key TEXT PRIMARY KEY,
  channel_id TEXT NOT NULL,
  display_name TEXT NOT NULL,
  joined_at TEXT NOT NULL,
  last_seen_at TEXT NOT NULL,
  audio_streams INTEGER NOT NULL DEFAULT 0,
  video_streams INTEGER NOT NULL DEFAULT 0,
  camera_enabled INTEGER NOT NULL DEFAULT 0,
  screen_enabled INTEGER NOT NULL DEFAULT 0,
  screen_audio_enabled INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_voice_presence_channel_last_seen
  ON voice_presence(channel_id, last_seen_at);
