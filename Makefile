.PHONY: dev dev-backend dev-frontend lint lint-backend lint-frontend fmt build

# Run both backend and frontend in development mode
dev:
	@echo "Start backend and frontend separately:"
	@echo "  make dev-backend"
	@echo "  make dev-frontend"

dev-backend:
	cd backend && go run ./cmd/server

dev-frontend:
	cd frontend && npm run dev

# Linting
lint: lint-backend lint-frontend

lint-backend:
	cd backend && golangci-lint run ./...

lint-frontend:
	cd frontend && npx oxlint

# Formatting
fmt:
	cd frontend && npx oxfmt --write src/

# Build
build: build-backend build-frontend

build-backend:
	cd backend && go build -o bin/server ./cmd/server

build-frontend:
	cd frontend && npm run build
