-- Per-participant secret for anonymous participants. Only the SHA-256 hex
-- digest is stored; the raw token is returned once to the joining browser.
-- NULL means a legacy row created before tokens existed (or an anonymous row
-- copied by "Duplicate calendar") — the first successful write mints one
-- (trust-on-first-use). See plans/bugfix/participant-authorization.md.
ALTER TABLE participants ADD COLUMN IF NOT EXISTS token_hash TEXT NULL;
