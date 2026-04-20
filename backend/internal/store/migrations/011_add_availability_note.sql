ALTER TABLE availability ADD COLUMN note TEXT;
ALTER TABLE availability ADD CONSTRAINT availability_note_length_chk
    CHECK (note IS NULL OR char_length(note) <= 140);
