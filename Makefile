.PHONY: dev dev-backend dev-backend-watch dev-frontend lint lint-backend lint-frontend lint-pii fmt fmt-check build test test-backend test-frontend test-integration test-e2e tf-plan

# Run both backend and frontend in development mode
dev:
	@echo "Start backend and frontend separately:"
	@echo "  make dev-backend"
	@echo "  make dev-frontend"

dev-backend:
	cd backend && go run ./cmd/server

# Auto-restarts on .go/.sql changes. Requires `air` (https://github.com/air-verse/air):
#   go install github.com/air-verse/air@latest
dev-backend-watch:
	cd backend && air

dev-frontend:
	cd frontend && pnpm dev

# Linting — four independent commands, run in parallel.
lint:
	$(MAKE) -j4 lint-backend lint-frontend lint-pii fmt-check

lint-backend:
	cd backend && golangci-lint run ./...

lint-frontend:
	cd frontend && pnpm exec oxlint && pnpm run typecheck

# Catch PII (email/phone/IP) leaking into log calls. See AGENTS.md "Logging".
lint-pii:
	@if grep -rEn '(logger|log)\.(Info|Warn|Error|Debug|info|warn|error|debug)[^)]*"(email|phone|ip_address|client_ip)"' backend/internal backend/cmd frontend/src 2>/dev/null; then \
		echo ""; \
		echo "ERROR: PII field detected in a log call (see lines above)."; \
		echo "Never log emails, phone numbers, or IP addresses. See AGENTS.md > Logging."; \
		exit 1; \
	fi

# Formatting. Backend uses `golangci-lint fmt` which applies the formatters
# enabled in `backend/.golangci.yml` (gofmt + goimports), so the local fix
# matches what CI checks.
fmt:
	cd frontend && pnpm exec oxfmt --write src/
	cd backend && golangci-lint fmt

# Verify the tree is formatted without modifying it. Run as part of `make
# lint` (and therefore CI) so unformatted code can't land on main.
fmt-check:
	cd frontend && pnpm exec oxfmt --check src/

# Testing — backend and frontend run in parallel (same runner, lower wall-clock).
test:
	$(MAKE) -j2 test-backend test-frontend

test-backend:
	cd backend && go test ./...

test-frontend:
	cd frontend && pnpm test

test-integration:
	cd backend && go test -tags=integration ./...

# Playwright end-to-end suite. Boots embedded Postgres + the Go backend
# (with E2E_TEST_MODE=true) and Next.js, then drives Chromium against them.
# See plans/infra/e2e-tests.md and frontend/e2e/.
test-e2e:
	cd frontend && pnpm exec playwright test

# Build
build: build-backend build-frontend

build-backend:
	cd backend && go build -o bin/server ./cmd/server

build-frontend:
	cd frontend && pnpm build

# Surface infra drift locally. Exits 2 if the plan has changes — useful in
# pre-commit hooks or just as a manual check before merging infra edits.
tf-plan:
	terraform -chdir=infra init -input=false
	terraform -chdir=infra plan -detailed-exitcode
