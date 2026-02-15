# Fosscord

Base monorepo scaffold for a desktop client and backend server.

## Contents

- `apps/client`: Tauri v2 + SvelteKit (SSR disabled, static adapter)
- `apps/server`: Go HTTP API (`chi`)
- `deploy`: docker-compose files and LiveKit config
- `tests/e2e-node`: headless Node e2e harness (Vitest)

## Prerequisites

- Go 1.22+
- Node.js 20+ (22 recommended) and `pnpm`
- Rust toolchain (`rustup`, `cargo`)
- Tauri system prerequisites for your OS
- Docker + Docker Compose

If `rustup` does not have a default toolchain yet:

```bash
rustup toolchain install stable
rustup default stable
```

## Quick Start

1. (Optional) Enter the dev shell from `flake.nix`:

```bash
nix develop
```

2. Prepare environment variables:

```bash
cp .env.example .env
```

3. Start LiveKit:

```bash
docker compose -f deploy/docker-compose.yml up -d
```

4. Install client dependencies:

```bash
cd apps/client
pnpm install
cd ../..
```

5. Start backend + Tauri dev with one command:

```bash
make dev
```

## Alternative Run (separate terminals)

```bash
# backend
cd apps/server
go run ./cmd/server

# client
cd apps/client
pnpm tauri dev
```

## Smoke Test

```bash
curl http://localhost:8080/health
curl http://localhost:8080/api/server-info
```

Expected:

- `GET /health` -> `200 {"status":"ok"}`
- `GET /api/server-info` -> `200 {"name", "publicKeyFingerprintEmoji", "livekitUrl"}`
- `POST /api/livekit/token` -> `501` stub

In UI:

- `/` shows server list (currently one Local Server)
- `/server` shows backend status and server info

## Integration Tests

1. Create test env file:

```bash
cp deploy/.env.test.example deploy/.env.test
```

2. Run Go integration tests against dockerized backend + LiveKit:

```bash
make test-integration
```

3. Keep test environment running after tests:

```bash
KEEP=1 make test-integration
```

4. Run headless Node e2e harness:

```bash
cd tests/e2e-node
pnpm install
pnpm test
```

or from repo root:

```bash
pnpm test:e2e
```

## Test Commands

- `make up-test`: start test compose (`deploy/docker-compose.test.yml`)
- `make wait-test`: poll backend + LiveKit readiness
- `make down-test`: stop and remove test compose resources
- `make test-integration`: up -> wait -> `go test -tags=integration` -> down (unless `KEEP=1`)

## Structure

```text
apps/
  client/
    src/
    src-tauri/
  server/
    cmd/server/main.go
    integration/
    internal/httpapi/
    internal/config/
deploy/
  docker-compose.yml
  docker-compose.test.yml
  livekit.yaml
tests/
  e2e-node/
```
