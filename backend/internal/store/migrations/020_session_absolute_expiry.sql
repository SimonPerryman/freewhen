-- Add an absolute max-age to sessions so `RefreshSession` can't slide
-- `expires_at` forward indefinitely. Without this, an active (or stolen)
-- session refreshes itself every authenticated request and effectively
-- never expires.
--
-- New sessions are created with absolute_expires_at = now + 90d. Refresh
-- clamps expires_at to min(now+30d, absolute_expires_at). When the clamp
-- pins, the user re-authenticates next time the sliding window would have
-- advanced past the absolute.
--
-- Backfill existing rows with created_at + 90 days. Rows older than 90 days
-- will refresh-clamp to a past time and be treated as expired, forcing
-- those users to re-auth — acceptable cost.

ALTER TABLE sessions ADD COLUMN absolute_expires_at TIMESTAMPTZ;

UPDATE sessions SET absolute_expires_at = created_at + INTERVAL '90 days';

ALTER TABLE sessions ALTER COLUMN absolute_expires_at SET NOT NULL;
