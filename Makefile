.PHONY: dev api web test race vet typecheck lint build verify

# Keep Go's writable build cache in the temporary workspace on locked-down hosts.
GO_BUILD_CACHE ?= /tmp/orca-go-cache
export GOCACHE = $(GO_BUILD_CACHE)

# Requires PostgreSQL at DATABASE_URL. The web app proxies /api to the Go API.
dev:
	@set -eu; \
	if [ -f .env ]; then set -a; . ./.env; set +a; fi; \
	(cd backend && go run ./cmd/api) & api_pid=$$!; \
	(cd frontend && npm run dev) & web_pid=$$!; \
	trap 'kill $$api_pid $$web_pid 2>/dev/null || true' INT TERM EXIT; \
	wait

api:
	@if [ -f .env ]; then set -a; . ./.env; set +a; fi; cd backend && go run ./cmd/api

web:
	cd frontend && npm run dev

test:
	cd backend && go test ./...

race:
	cd backend && go test -race ./...

vet:
	cd backend && go vet ./...

typecheck:
	cd frontend && npm run typecheck

lint:
	cd frontend && npm run lint

build:
	cd frontend && npm run build

verify: test race vet typecheck lint build
