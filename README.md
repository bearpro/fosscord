# Fosscord

Desktop-first scaffold with:

- `apps/client`: Tauri v2 + SvelteKit
- `apps/server`: Go backend with handshake and channel APIs
- `deploy`: LiveKit and integration test compose files
- `tests/e2e-node`: headless Node API harness (Vitest)

## Prerequisites

- Go 1.24+
- Node.js 20+ (22 recommended) + `pnpm`
- Rust toolchain (`rustup`, `cargo`)
- Tauri system prerequisites for your OS
- Docker + Docker Compose

## Quick Start

1. Prepare env:

```bash
cp .env.example .env
```

2. Start LiveKit (dev):

```bash
docker compose -f deploy/docker-compose.yml up -d
```

3. Install client dependencies:

```bash
cd apps/client
pnpm install
cd ../..
```

4. Run backend + desktop client:

```bash
make dev
```

## Handshake Flow (Current Step)

### Backend data files

The backend creates/uses data under `DATA_DIR` (default `data`):

- `server.db` (SQLite; stores server identity + invites)
- `server_config.json` (server name + channels, kept outside DB)

### SQLite migrations

- SQL migration files are stored in `apps/server/internal/serverstate/migrations/`
- Applied migrations are tracked in SQLite table `schema_migrations`
- On startup, the backend applies all pending `*_up.sql` migrations automatically

### APIs

- `GET /health`
- `GET /api/server-info`
- `GET /api/channels`
- `POST /api/admin/invites` (requires `Authorization: Bearer $ADMIN_TOKEN`)
- `POST /api/connect/begin`
- `POST /api/connect/finish`
- `POST /api/livekit/token` (stub, 501)

### Manual invite + connect smoke

1. Generate a client keypair in the desktop app (`/setup`) and copy `publicKey`.
2. Create invite:

```bash
curl -X POST http://localhost:8080/api/admin/invites \
  -H 'content-type: application/json' \
  -H 'authorization: Bearer devadmin' \
  -d '{"clientPublicKey":"<base64-ed25519-pub>","label":"manual"}'
```

3. Paste returned `inviteLink` into "Add server" in `/servers`.
4. After successful handshake, open `/server/:id` to view channels.

## Client Routes

- `/setup`: generate/import local identity keypair
- `/servers`: list connected servers + add server by invite link
- `/server/:id`: server status and channel list

## Integration Tests

Create test env once:

```bash
cp deploy/.env.test.example deploy/.env.test
```

Run Go integration suite (dockerized backend + LiveKit):

```bash
make test-integration
```

Keep environment running for debugging:

```bash
KEEP=1 make test-integration
```

Run Node API harness:

```bash
make test-e2e
```

or from repo root:

```bash
pnpm test:e2e
```

Or directly:

```bash
cd tests/e2e-node
pnpm install
E2E_API_BASE_URL=http://127.0.0.1:8080 E2E_LIVEKIT_URL=http://127.0.0.1:7880 pnpm test
```

## Test Compose Commands

- `make up-test`
- `make wait-test`
- `make down-test`
- `make test-integration`
- `make test-e2e`

## Notes

- `ADMIN_TOKEN` is required for `/api/admin/invites` in dev/test.
- `DB_PATH` can override SQLite location; if relative, it is resolved under `DATA_DIR`.
- No real LiveKit token issuance yet (`/api/livekit/token` returns 501).
- Live voice/data-channel scenarios are planned for the next step.
