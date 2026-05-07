-- Audit F15: normalize stored email addresses to lower-case + trimmed.
--
-- Application code now lower-cases + trims at every boundary (SendMagicLink,
-- MagicLinkVerify, GoogleCallback, test-login). This migration backfills any
-- pre-existing rows that were stored with mixed case so the column invariant
-- ("email is always already canonical") holds for both new and old data.
--
-- The UNIQUE constraint on users.email will surface case-variant collisions
-- by failing the migration. If that happens in prod, the operator picks the
-- canonical row, deletes the duplicates (or merges by hand), and re-runs.
-- This is preferred over a silent merge.

UPDATE users
SET email = lower(btrim(email))
WHERE email <> lower(btrim(email));

UPDATE magic_link_tokens
SET email = lower(btrim(email))
WHERE email <> lower(btrim(email));
