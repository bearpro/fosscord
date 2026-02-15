SHELL := /bin/bash

.PHONY: dev dev-server dev-client fmt test docker-up docker-down

dev:
	@set -a; [ -f .env ] && . ./.env; set +a; \
	trap 'kill 0' EXIT; \
	(cd apps/server && go run ./cmd/server) & \
	(cd apps/client && pnpm tauri dev)

dev-server:
	@set -a; [ -f .env ] && . ./.env; set +a; \
	cd apps/server && go run ./cmd/server

dev-client:
	@set -a; [ -f .env ] && . ./.env; set +a; \
	cd apps/client && pnpm tauri dev

fmt:
	cd apps/server && go fmt ./...
	cd apps/client && pnpm format

test:
	cd apps/server && go test ./...

docker-up:
	docker compose -f deploy/docker-compose.yml up -d

docker-down:
	docker compose -f deploy/docker-compose.yml down
