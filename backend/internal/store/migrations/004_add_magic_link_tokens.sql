CREATE TABLE magic_link_tokens (
    token_hash  TEXT PRIMARY KEY,
    email       TEXT NOT NULL,
    return_url  TEXT NOT NULL DEFAULT '/',
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_magic_link_tokens_expires_at ON magic_link_tokens(expires_at);
