# Seatlane: Go + sqlc slices gated by bridge-en, React UI in web/.
#
#   make dev     Go API (with the dev clock) on :8080 + Vite on :5173 (proxies /api)
#   make build   web/dist + bin/seatlane (the server serves web/dist)
#   make seed    dev accounts organizer@example.test + admin@example.test in $(DB) (prints the password)
#   make check   bridge-en -check: go.mod pin, refusals, golden .en, F-ID cross-checks
#   make test    go test ./...

GO        ?= go
BRIDGE_EN ?= bridge-en
DB        ?= seatlane.db
BRIDGE_VERSION = $(shell $(GO) list -m -f '{{.Version}}' github.com/pierre10101/go-ai-bridge)

.PHONY: dev build run seed check test generate tools web-install clean

dev: web/node_modules
	@echo "API on http://localhost:8080 (dev clock: POST /__dev/advance?seconds=N), UI on http://localhost:5173"
	@trap 'kill 0' INT TERM EXIT; \
	$(GO) run ./cmd/server -dev-clock -db $(DB) -web '' & \
	cd web && npm run dev

build: web/node_modules
	cd web && npm run build
	$(GO) build -o bin/seatlane ./cmd/server

run: build
	./bin/seatlane -db $(DB) -web web/dist

# Local development only: the server refuses -seed-dev-accounts without SEATLANE_DEV=1.
seed:
	SEATLANE_DEV=1 $(GO) run ./cmd/server -db $(DB) -seed-dev-accounts

check:
	$(BRIDGE_EN) -check features/*/

test:
	$(GO) vet ./...
	$(GO) test ./...

# After changing schema.sql or queries/*.sql: regenerate db/ and the English, then review the .en diff.
generate:
	sqlc generate
	for s in features/*/; do $(BRIDGE_EN) -write $$s; done

# The bridge-en binary of the version go.mod pins (bridge-en refuses any other).
tools:
	$(GO) install github.com/pierre10101/go-ai-bridge/cmd/bridge-en@$(BRIDGE_VERSION)

web-install web/node_modules: web/package-lock.json
	cd web && npm ci
	@touch web/node_modules

clean:
	rm -rf bin web/dist
