-- Hash session tokens at rest. Previously `sessions.token` held the raw
-- 32-byte random token, which means DB read access = account takeover for
-- every signed-in user. Switch to storing sha256(token) (hex-encoded TEXT,
-- mirroring the `magic_link_tokens.token_hash` pattern from migration 004).
--
-- We can't backfill — we don't have the plaintext tokens, only their values
-- as previously stored. TRUNCATE the existing rows; users get logged out
-- once and re-authenticate via magic link. Acceptable cost given the user
-- base size and the security upgrade.

TRUNCATE TABLE sessions;

ALTER TABLE sessions DROP CONSTRAINT sessions_pkey;
ALTER TABLE sessions DROP COLUMN token;
ALTER TABLE sessions ADD COLUMN token_hash TEXT NOT NULL;
ALTER TABLE sessions ADD PRIMARY KEY (token_hash);
