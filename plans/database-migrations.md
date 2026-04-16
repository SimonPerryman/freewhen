# Database Migrations

Status: done

## Goal

Create the Postgres schema for V1 — three tables matching the models already defined in `backend/internal/model/model.go`.

## Plan

### Migration 001: Create tables

**calendars**
- `id` — text, primary key (generate a short random slug in Go, not a UUID)
- `name` — text, not null, default empty string (optional event name)
- `created_at` — timestamptz, not null, default now()

**participants**
- `id` — text, primary key (generated in Go)
- `calendar_id` — text, not null, references calendars(id) on delete cascade
- `name` — text, not null
- `colour` — text, not null
- Unique constraint on (calendar_id, name) — no duplicate names per calendar

**availability**
- `participant_id` — text, not null, references participants(id) on delete cascade
- `calendar_id` — text, not null, references calendars(id) on delete cascade
- `date` — date, not null
- Primary key on (participant_id, date) — one entry per person per day
- Index on calendar_id for fast lookups

### Migration runner

Add a simple migration runner to `main.go` that reads `.sql` files from `migrations/` and applies them on startup. No third-party migration tool — just embed the SQL and run it.

Track applied migrations in a `schema_migrations` table (single `version` text column). On startup, create the table if it doesn't exist, check which versions have been applied, and only run new ones. This prevents re-running migrations on every restart.

### Files to create/modify

- `backend/internal/store/migrations/001_create_tables.sql` — new (under store/ for go:embed)
- `backend/internal/store/migrate.go` — new, migration runner
- `backend/cmd/server/main.go` — modify, call migrate on startup before creating server

### Existing scaffolding we're building on

- `store/store.go` — Store struct with `*sql.DB` already exists
- `main.go` — already connects to Postgres and pings, just needs migrate call added
- `model/model.go` — all structs already defined, schema must match these

## Log

- 2026-04-16 — Created plan
- 2026-04-16 — Implemented. Moved SQL files to `internal/store/migrations/` (go:embed requires files in same or child directory). Migration runner uses embed.FS, tracks applied versions in `schema_migrations` table. main.go calls `store.Migrate(db)` before server creation.
