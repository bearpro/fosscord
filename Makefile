SHELL := /bin/bash

TEST_ENV_FILE ?= deploy/.env.test
TEST_ENV_EXAMPLE ?= deploy/.env.test.example
TEST_PROJECT ?= fosscord-test
TEST_COMPOSE := docker compose -p $(TEST_PROJECT) -f deploy/docker-compose.test.yml --env-file $(TEST_ENV_FILE)
KEEP ?= 0

.PHONY: dev dev-server dev-client fmt test docker-up docker-down ensure-test-env up-test down-test wait-test test-integration test-e2e

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

ensure-test-env:
	@if [ ! -f "$(TEST_ENV_FILE)" ]; then \
		cp "$(TEST_ENV_EXAMPLE)" "$(TEST_ENV_FILE)"; \
		echo "created $(TEST_ENV_FILE) from $(TEST_ENV_EXAMPLE)"; \
	fi

up-test: ensure-test-env
	$(TEST_COMPOSE) up -d --build

down-test: ensure-test-env
	$(TEST_COMPOSE) down -v

wait-test: ensure-test-env
	@set -eu; \
	set -a; . "$(TEST_ENV_FILE)"; set +a; \
	timeout="$${WAIT_TIMEOUT:-60}"; \
	host_timeout="$${WAIT_HOST_TIMEOUT:-5}"; \
	host_backend_url="$${API_BASE_URL:-http://localhost:8080}/health"; \
	host_livekit_url="$${LIVEKIT_PUBLIC_URL:-http://localhost:7880}"; \
	if ./scripts/wait-http.sh "$$host_backend_url" "$$host_timeout" "backend health (host)" >/dev/null 2>&1; then \
		echo "ready: backend health (host)"; \
	else \
		echo "host backend check failed, falling back to in-container probe"; \
		start="$$(date +%s)"; \
		while ! $(TEST_COMPOSE) exec -T server curl -fsS --max-time 2 http://127.0.0.1:8080/health >/dev/null 2>&1; do \
			now="$$(date +%s)"; \
			if [ $$((now - start)) -ge "$$timeout" ]; then \
				echo "timeout waiting for server health after $${timeout}s" >&2; \
				exit 1; \
			fi; \
			sleep 1; \
		done; \
		echo "ready: server health (container)"; \
	fi; \
	if ./scripts/wait-http.sh "$$host_livekit_url" "$$host_timeout" "livekit endpoint (host)" >/dev/null 2>&1; then \
		echo "ready: livekit endpoint (host)"; \
	else \
		echo "host livekit check failed, falling back to in-container tcp probe"; \
		start="$$(date +%s)"; \
		while ! $(TEST_COMPOSE) exec -T livekit nc -z 127.0.0.1 7880 >/dev/null 2>&1; do \
			now="$$(date +%s)"; \
			if [ $$((now - start)) -ge "$$timeout" ]; then \
				echo "timeout waiting for livekit tcp port after $${timeout}s" >&2; \
				exit 1; \
			fi; \
			sleep 1; \
		done; \
		echo "ready: livekit tcp port (container)"; \
	fi

test-integration: ensure-test-env
	@set -euo pipefail; \
	cleanup() { \
		if [ "$(KEEP)" != "1" ]; then \
			$(MAKE) down-test; \
		else \
			echo "KEEP=1 set, leaving test environment running"; \
		fi; \
	}; \
	trap cleanup EXIT; \
	$(MAKE) up-test; \
	$(MAKE) wait-test; \
	$(TEST_COMPOSE) exec -T server sh -c "cd /workspace && API_BASE_URL=http://127.0.0.1:8080 go test ./... -tags=integration"

test-e2e: ensure-test-env
	@set -a; . "$(TEST_ENV_FILE)"; set +a; \
	pnpm --dir tests/e2e-node test
