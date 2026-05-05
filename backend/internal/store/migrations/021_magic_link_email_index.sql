-- Index magic_link_tokens.email so we can efficiently delete prior outstanding
-- tokens for an email when a new magic link is issued. Without the index the
-- DELETE in CreateMagicLinkToken does a sequential scan; with it, we look up
-- by the indexed email column.
--
-- Invalidating prior tokens on new issue limits the blast radius if an old
-- link leaks (Slack unfurl, email forwarding) — only the most recent token
-- is ever valid for a given email at a time.

CREATE INDEX idx_magic_link_tokens_email ON magic_link_tokens(email);
