-- Soft-delete columns + supporting indexes for the Leave / Delete-calendar
-- flow. Live reads filter on `deleted_at IS NULL`; rejoining requires the
-- pre-existing UNIQUE constraints to be expressed as partial indexes so a
-- soft-deleted row no longer collides with a fresh insert.
ALTER TABLE calendars ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ NULL;
ALTER TABLE participants ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ NULL;

CREATE INDEX IF NOT EXISTS calendars_owner_active_idx
    ON calendars (owner_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS participants_calendar_active_idx
    ON participants (calendar_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS participants_user_active_idx
    ON participants (user_id) WHERE deleted_at IS NULL AND user_id IS NOT NULL;

-- Replace the table-level UNIQUE (calendar_id, name) with a partial index that
-- only covers live rows. Create the partial index first so uniqueness is never
-- unenforced during the swap (migrations run single-threaded on boot).
CREATE UNIQUE INDEX IF NOT EXISTS participants_calendar_name_active_idx
    ON participants (calendar_id, name) WHERE deleted_at IS NULL;
ALTER TABLE participants DROP CONSTRAINT IF EXISTS participants_calendar_id_name_key;

-- Same swap for the (calendar_id, user_id) uniqueness from migration 007.
CREATE UNIQUE INDEX IF NOT EXISTS idx_participants_calendar_user_active
    ON participants (calendar_id, user_id) WHERE user_id IS NOT NULL AND deleted_at IS NULL;
DROP INDEX IF EXISTS idx_participants_calendar_user;
