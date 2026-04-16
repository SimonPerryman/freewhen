# FreeWhen

A dead-simple shared calendar for finding when a group is free.

Create a calendar, share the link, mark your available days. See everyone's availability at a glance.

## Quick Start

### Prerequisites

- Go 1.23+
- Node.js 20+
- A hosted Postgres database (Supabase, Neon, Railway, etc.)

### Setup

```bash
cp .env.example .env
# Edit .env with your DATABASE_URL
```

### Run

```bash
# Backend (port 8080)
make dev-backend

# Frontend (port 3000)
make dev-frontend
```

### Other Commands

```bash
make lint          # Lint both backend and frontend
make fmt           # Format frontend code
make build         # Build both for production
```

## Project Structure

- `backend/` - Go API server
- `frontend/` - Next.js React app
- `docs/` - Project documentation
