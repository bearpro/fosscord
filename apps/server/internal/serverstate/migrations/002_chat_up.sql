CREATE TABLE IF NOT EXISTS members (
  public_key TEXT PRIMARY KEY,
  display_name TEXT NOT NULL,
  first_connected_at TEXT NOT NULL,
  last_connected_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
  token TEXT PRIMARY KEY,
  client_public_key TEXT NOT NULL,
  created_at TEXT NOT NULL,
  expires_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);

CREATE TABLE IF NOT EXISTS messages (
  id TEXT PRIMARY KEY,
  channel_id TEXT NOT NULL,
  author_public_key TEXT NOT NULL,
  author_name TEXT NOT NULL,
  content_markdown TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_messages_channel_created_at ON messages(channel_id, created_at);
