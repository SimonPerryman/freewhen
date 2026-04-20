ALTER TABLE calendars ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

UPDATE calendars SET updated_at = COALESCE(finalised_at, created_at);

CREATE INDEX idx_calendars_updated_at ON calendars(updated_at DESC);

CREATE INDEX idx_participants_user_id ON participants(user_id) WHERE user_id IS NOT NULL;
