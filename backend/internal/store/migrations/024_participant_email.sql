-- 024_participant_email.sql
-- Optional contact email for participants — used for finalised/nudge emails.
-- Nullable, no unique constraint (same person can join multiple calendars
-- with the same address). Never indexed; we never look up by email.
ALTER TABLE participants ADD COLUMN IF NOT EXISTS email TEXT;
