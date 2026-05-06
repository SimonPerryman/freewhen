-- Replace the user_id-only index with a composite (user_id, created_at DESC)
-- so PruneSessionsForUser's "keep newest N for this user, delete the rest"
-- query is an index scan rather than a heap sort.
--
-- The composite covers user_id equality lookups too, so the original
-- single-column index is redundant.

DROP INDEX IF EXISTS idx_sessions_user_id;
CREATE INDEX idx_sessions_user_id_created_at ON sessions(user_id, created_at DESC);
