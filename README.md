# Fosscord

Scaffold with two client modes:

- Desktop mode: Tauri v2 + SvelteKit (`apps/client`)
- Single-server web mode: static client behind nginx edge proxy
- Go backend (`apps/server`) + LiveKit (`deploy/livekit.yaml`)
- Integration harness (`deploy/docker-compose.test.yml`, Go + Node tests)

## Prerequisites

- Go 1.24+
- Node.js 22 + `pnpm`
- Rust toolchain (`rustup`, `cargo`) for Tauri desktop
- Docker + Docker Compose

## Run Desktop (Tauri)

```bash
cp .env.example .env
pnpm --dir apps/client install
docker compose up -d livekit
make dev
```

## Run Full Web Stack (one command)

This starts LiveKit + backend + edge nginx with prebuilt frontend:

```bash
cp .env.example .env
docker compose up --build
```

Open:

- Web app: `http://localhost:${EDGE_HTTP_PORT}` (default `8088`)
- LiveKit: `http://localhost:7880`

In web mode, LiveKit signaling is proxied through edge (`/rtc/*`), so clients can use the same origin as the app.

## Backend Data Model

Backend uses `DATA_DIR` (default local dev: `apps/server/data`):

- `server.db` (SQLite): server identity, invites, migration history
- `server_config.json` (outside DB): server name, channels, admin list

Example `server_config.json` fragment:

```json
{
  "serverName": "Local Server",
  "channels": [
    { "id": "general", "type": "text", "name": "general" }
  ],
  "adminPublicKeys": [
    "<base64-ed25519-public-key>"
  ]
}
```

## SQLite Migrations

- Migrations: `apps/server/internal/serverstate/migrations/*.sql`
- Applied migrations tracked in `schema_migrations`
- Backend auto-applies pending `*_up.sql` on startup

## API

- `GET /health`
- `GET /api/server-info` (includes `adminPublicKeys`)
- `GET /api/channels`
- `POST /api/connect/begin`
- `POST /api/connect/finish`
- `POST /api/admin/invites` (Bearer `ADMIN_TOKEN`)
- `POST /api/admin/invites/client-signed` (admin client signature)
- `POST /api/livekit/token` (Bearer session token, voice channel token)
- `POST /api/livekit/voice/touch` (heartbeat + stream counters)
- `POST /api/livekit/voice/leave`
- `GET /api/livekit/voice/channels/{channelID}/state`

## Web Single-Server Mode Behavior

Frontend build args/env:

- `VITE_CLIENT_MODE=single-server-web`
- `VITE_API_BASE_URL=/`
- `VITE_SINGLE_SERVER_BASE_URL=/`

In this mode the app:

- shows only client public key
- shows message that user is not invited
- fetches public admin list from `/api/server-info`
- shows "Add user" action only when client public key is in `adminPublicKeys`
- keeps "Connect via invite link" available

## Voice Channels

- One active voice channel per client identity at a time (server-enforced presence row).
- Joining a voice channel auto-publishes microphone.
- Clients auto-subscribe to all remote audio/video streams in that channel.
- UI exposes toggles for mic, camera, and screen share (with optional system audio).
- Voice presence/state is persisted via SQLite table `voice_presence` and returned by
  `/api/livekit/voice/channels/{channelID}/state`.

## Integration Tests

```bash
cp deploy/.env.test.example deploy/.env.test
make test-integration
make test-e2e
```

Keep test stack running:

```bash
KEEP=1 make test-integration
```

## Notes

- `DB_PATH` overrides SQLite file; relative paths are resolved under `DATA_DIR`.
- `WEB_DIST_DIR` enables backend static file serving if set.
- `LIVEKIT_PUBLIC_URL` controls which URL backend returns to clients in `/api/server-info` and connect responses.
