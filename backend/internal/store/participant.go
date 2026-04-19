package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/freewhen/backend/internal/model"
)

var colourPalette = []string{
	"#E63946", // red
	"#457B9D", // steel blue
	"#2A9D8F", // teal
	"#E9C46A", // saffron
	"#F4A261", // sandy brown
	"#264653", // charcoal
	"#A855F7", // purple
	"#06B6D4", // cyan
	"#EC4899", // pink
	"#84CC16", // lime
}

func (s *Store) AddParticipant(ctx context.Context, calendarID, name string) (model.Participant, error) {
	var exists bool
	var finalisedDate sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT TRUE, finalised_date::text FROM calendars WHERE id = $1`, calendarID,
	).Scan(&exists, &finalisedDate)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Participant{}, ErrNotFound
		}
		return model.Participant{}, fmt.Errorf("look up calendar: %w", err)
	}
	if finalisedDate.Valid {
		return model.Participant{}, ErrCalendarFinalised
	}

	count, err := s.participantCount(ctx, calendarID)
	if err != nil {
		return model.Participant{}, fmt.Errorf("count participants: %w", err)
	}
	colour := colourPalette[count%len(colourPalette)]

	const maxRetries = 3
	for i := 0; i < maxRetries; i++ {
		id := generateID(8)
		var p model.Participant
		err := s.db.QueryRowContext(ctx,
			`INSERT INTO participants (id, calendar_id, name, colour) VALUES ($1, $2, $3, $4)
			 RETURNING id, calendar_id, name, colour`,
			id, calendarID, name, colour,
		).Scan(&p.ID, &p.CalendarID, &p.Name, &p.Colour)
		if err != nil {
			if isUniqueViolation(err) {
				if isDuplicateName(err) {
					return model.Participant{}, ErrDuplicateName
				}
				continue // ID collision, retry
			}
			return model.Participant{}, fmt.Errorf("insert participant: %w", err)
		}
		return p, nil
	}
	return model.Participant{}, fmt.Errorf("failed to generate unique participant ID after %d attempts", maxRetries)
}

func (s *Store) GetParticipant(ctx context.Context, id string) (model.Participant, error) {
	var p model.Participant
	err := s.db.QueryRowContext(ctx,
		`SELECT id, calendar_id, name, colour FROM participants WHERE id = $1`, id,
	).Scan(&p.ID, &p.CalendarID, &p.Name, &p.Colour)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Participant{}, ErrNotFound
		}
		return model.Participant{}, fmt.Errorf("get participant: %w", err)
	}
	return p, nil
}

func (s *Store) participantCount(ctx context.Context, calendarID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM participants WHERE calendar_id = $1`, calendarID,
	).Scan(&count)
	return count, err
}
