CREATE TABLE calendars (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE participants (
    id          TEXT PRIMARY KEY,
    calendar_id TEXT NOT NULL REFERENCES calendars(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    colour      TEXT NOT NULL,
    UNIQUE (calendar_id, name)
);

CREATE TABLE availability (
    participant_id TEXT NOT NULL REFERENCES participants(id) ON DELETE CASCADE,
    calendar_id    TEXT NOT NULL REFERENCES calendars(id) ON DELETE CASCADE,
    date           DATE NOT NULL,
    PRIMARY KEY (participant_id, date)
);

CREATE INDEX idx_availability_calendar_id ON availability(calendar_id);
