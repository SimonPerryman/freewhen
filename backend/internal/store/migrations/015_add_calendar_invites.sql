CREATE TABLE calendar_invites (
    id                      TEXT PRIMARY KEY,
    calendar_id             TEXT NOT NULL REFERENCES calendars(id) ON DELETE CASCADE,
    name                    TEXT NOT NULL,
    matched_participant_id  TEXT REFERENCES participants(id) ON DELETE SET NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX calendar_invites_calendar_lower_name_idx
    ON calendar_invites (calendar_id, LOWER(TRIM(name)));

CREATE INDEX calendar_invites_calendar_idx
    ON calendar_invites (calendar_id);

CREATE INDEX calendar_invites_matched_participant_idx
    ON calendar_invites (matched_participant_id)
    WHERE matched_participant_id IS NOT NULL;
