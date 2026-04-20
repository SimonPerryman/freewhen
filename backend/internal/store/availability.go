package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// SetAvailability bulk-replaces a participant's availability rows in a
// transaction. noteOverrides is a per-date override map: a key present in the
// map is applied as the new note (empty string → NULL); a key absent from the
// map preserves the existing note for that date (if any). Pass nil to preserve
// all existing notes ("don't touch" semantics).
func (s *Store) SetAvailability(ctx context.Context, participantID string, dates []string, noteOverrides map[string]string) error {
	var calendarID string
	var finalisedDate sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT p.calendar_id, c.finalised_date::text
		 FROM participants p JOIN calendars c ON c.id = p.calendar_id
		 WHERE p.id = $1`, participantID,
	).Scan(&calendarID, &finalisedDate)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("look up participant: %w", err)
	}
	if finalisedDate.Valid {
		return ErrCalendarFinalised
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	existingNotes, err := selectExistingNotes(ctx, tx, participantID)
	if err != nil {
		return fmt.Errorf("read existing notes: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM availability WHERE participant_id = $1`, participantID,
	); err != nil {
		return fmt.Errorf("delete existing availability: %w", err)
	}

	for _, date := range dates {
		note := resolveNote(date, noteOverrides, existingNotes)
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO availability (participant_id, calendar_id, date, note) VALUES ($1, $2, $3, $4)`,
			participantID, calendarID, date, note,
		); err != nil {
			return fmt.Errorf("insert availability for %s: %w", date, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func selectExistingNotes(ctx context.Context, tx *sql.Tx, participantID string) (map[string]string, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT date::text, note FROM availability WHERE participant_id = $1`, participantID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	notes := map[string]string{}
	for rows.Next() {
		var date string
		var note sql.NullString
		if err := rows.Scan(&date, &note); err != nil {
			return nil, err
		}
		if note.Valid {
			notes[date] = note.String
		}
	}
	return notes, rows.Err()
}

func resolveNote(date string, overrides, existing map[string]string) any {
	if overrides != nil {
		if v, ok := overrides[date]; ok {
			if v == "" {
				return nil
			}
			return v
		}
	}
	if v, ok := existing[date]; ok {
		return v
	}
	return nil
}
