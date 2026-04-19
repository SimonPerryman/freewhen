package store

import (
	"context"
	"fmt"
)

func (s *Store) SetAvailability(ctx context.Context, participantID string, dates []string) error {
	var calendarID string
	err := s.db.QueryRowContext(ctx,
		`SELECT calendar_id FROM participants WHERE id = $1`, participantID,
	).Scan(&calendarID)
	if err != nil {
		return fmt.Errorf("look up participant: %w", err)
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
