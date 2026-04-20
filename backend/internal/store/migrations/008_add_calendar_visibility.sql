ALTER TABLE calendars ADD COLUMN visibility TEXT NOT NULL DEFAULT 'open'
    CHECK (visibility IN ('open', 'submit_to_view'));
