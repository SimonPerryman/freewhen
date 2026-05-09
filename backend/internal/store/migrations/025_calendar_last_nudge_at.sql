-- 025_calendar_last_nudge_at.sql
-- Tracks the last owner-triggered nudge per calendar so the API can enforce
-- a cooldown (default 6h) between nudges. Nullable: never-nudged calendars
-- have no row to compare against.
ALTER TABLE calendars ADD COLUMN IF NOT EXISTS last_nudge_at TIMESTAMPTZ NULL;
