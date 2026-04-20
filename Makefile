.PHONY: dev dev-backend dev-frontend lint lint-backend lint-frontend lint-pii fmt build test test-backend test-frontend test-integration

# Run both backend and frontend in development mode
dev:
	@echo "Start backend and frontend separately:"
	@echo "  make dev-backend"
	@echo "  make dev-frontend"

dev-backend:
	cd backend && go run ./cmd/server

dev-frontend:
	cd frontend && pnpm dev

# Linting
lint: lint-backend lint-frontend lint-pii

lint-backend:
	cd backend && golangci-lint run ./...

lint-frontend:
	cd frontend && pnpm exec oxlint

# Catch PII (email/phone/IP) leaking into log calls. See AGENTS.md "Logging".
lint-pii:
	@if grep -rEn '(logger|log)\.(Info|Warn|Error|Debug|info|warn|error|debug)[^)]*"(email|phone|ip_address|client_ip)"' backend/internal backend/cmd frontend/src 2>/dev/null; then \
		echo ""; \
		echo "ERROR: PII field detected in a log call (see lines above)."; \
		echo "Never log emails, phone numbers, or IP addresses. See AGENTS.md > Logging."; \
		exit 1; \
	fi

# Formatting
fmt:
	cd frontend && pnpm exec oxfmt --write src/

# Testing
test: test-backend test-frontend

test-backend:
	cd backend && go test ./...

test-frontend:
	cd frontend && pnpm test

test-integration:
	cd backend && go test -tags=integration ./...

# Build
build: build-backend build-frontend

build-backend:
	cd backend && go build -o bin/server ./cmd/server

build-frontend:
	cd frontend && pnpm build
