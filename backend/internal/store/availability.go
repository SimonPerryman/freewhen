package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func (s *Store) SetAvailability(ctx context.Context, participantID string, dates []string) error {
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

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM availability WHERE participant_id = $1`, participantID,
	); err != nil {
		return fmt.Errorf("delete existing availability: %w", err)
	}

	for _, date := range dates {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO availability (participant_id, calendar_id, date) VALUES ($1, $2, $3)`,
			participantID, calendarID, date,
		); err != nil {
			return fmt.Errorf("insert availability for %s: %w", date, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}
