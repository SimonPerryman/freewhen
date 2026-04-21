ALTER TABLE calendars ADD COLUMN passphrase TEXT;
ALTER TABLE calendars ADD COLUMN access_epoch INT NOT NULL DEFAULT 0;
COMMENT ON COLUMN calendars.passphrase IS
    'Server-generated passphrase (word-word-word-word). Stored plaintext — deliberate tradeoff; see plans/password-protected-calendars.md.';
