ALTER TABLE participants ADD COLUMN user_id TEXT REFERENCES users(id) ON DELETE SET NULL;
CREATE UNIQUE INDEX idx_participants_calendar_user ON participants(calendar_id, user_id) WHERE user_id IS NOT NULL;
