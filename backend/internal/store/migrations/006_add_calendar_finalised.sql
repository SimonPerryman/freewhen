ALTER TABLE calendars
    ADD COLUMN finalised_date DATE,
    ADD COLUMN finalised_at   TIMESTAMPTZ;
