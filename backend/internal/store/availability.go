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
//
// preferredOverrides works the same way for the boolean vote column: nil
// preserves every row's existing `preferred` value; non-nil replaces it for
// every date in `dates` using `overrides[date]` (defaulting to false for
// absent keys).
//
// The "calendar is finalised" check runs inside the transaction against a
// share-locked calendar row (see lockOpenCalendarForParticipant), so a
// concurrent finalise either commits first (and this write is rejected) or
// waits for this write to commit. Rows are inserted in one batched statement.
func (s *Store) SetAvailability(ctx context.Context, participantID string, dates []string, noteOverrides map[string]string, preferredOverrides map[string]bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	calendarID, err := lockOpenCalendarForParticipant(ctx, tx, participantID)
	if err != nil {
		return err
	}

	existingNotes, existingPreferred, err := selectExistingAvailability(ctx, tx, participantID)
	if err != nil {
		return fmt.Errorf("read existing availability: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM availability WHERE participant_id = $1`, participantID,
	); err != nil {
		return fmt.Errorf("delete existing availability: %w", err)
	}

	if len(dates) > 0 {
		notes := make([]*string, len(dates))
		preferred := make([]bool, len(dates))
		for i, date := range dates {
			notes[i] = resolveNote(date, noteOverrides, existingNotes)
			preferred[i] = resolvePreferred(date, preferredOverrides, existingPreferred)
		}
		// One round-trip regardless of how many dates (up to
		// limits.MaxDatesPerSubmission). The arrays are zipped by unnest.
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO availability (participant_id, calendar_id, date, note, preferred)
			 SELECT $1, $2, t.date::date, t.note, t.preferred
			 FROM unnest($3::text[], $4::text[], $5::bool[]) AS t(date, note, preferred)`,
			participantID, calendarID, dates, notes, preferred,
		); err != nil {
			return fmt.Errorf("insert availability: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// lockOpenCalendarForParticipant resolves the participant's calendar inside
// tx and takes a FOR SHARE lock on the calendar row. SetFinalisedDate's
// UPDATE needs a conflicting row lock, so finalise and availability writes
// serialise on the calendar: whichever commits first wins, and a write that
// lost the race sees the finalised row (READ COMMITTED re-reads the latest
// version after waiting for the lock). Shared locks don't block each other,
// so concurrent writers on the same calendar still proceed in parallel.
//
// Returns ErrNotFound for a missing/soft-deleted participant or calendar and
// ErrCalendarFinalised when the calendar is finalised.
func lockOpenCalendarForParticipant(ctx context.Context, tx *sql.Tx, participantID string) (string, error) {
	var calendarID string
	var finalisedDate sql.NullString
	err := tx.QueryRowContext(ctx,
		`SELECT c.id, c.finalised_date::text
		 FROM participants p JOIN calendars c ON c.id = p.calendar_id
		 WHERE p.id = $1 AND p.deleted_at IS NULL AND c.deleted_at IS NULL
		 FOR SHARE OF c`, participantID,
	).Scan(&calendarID, &finalisedDate)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("look up participant: %w", err)
	}
	if finalisedDate.Valid {
		return "", ErrCalendarFinalised
	}
	return calendarID, nil
}

func selectExistingAvailability(ctx context.Context, tx *sql.Tx, participantID string) (map[string]string, map[string]bool, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT date::text, note, preferred FROM availability WHERE participant_id = $1`, participantID,
	)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()

	notes := map[string]string{}
	preferred := map[string]bool{}
	for rows.Next() {
		var date string
		var note sql.NullString
		var pref bool
		if err := rows.Scan(&date, &note, &pref); err != nil {
			return nil, nil, err
		}
		if note.Valid {
			notes[date] = note.String
		}
		if pref {
			preferred[date] = true
		}
	}
	return notes, preferred, rows.Err()
}

func resolvePreferred(date string, overrides, existing map[string]bool) bool {
	if overrides != nil {
		return overrides[date]
	}
	return existing[date]
}

// resolveNote returns the note to store for date; nil means NULL.
func resolveNote(date string, overrides, existing map[string]string) *string {
	if overrides != nil {
		if v, ok := overrides[date]; ok {
			if v == "" {
				return nil
			}
			return &v
		}
	}
	if v, ok := existing[date]; ok {
		return &v
	}
	return nil
}
