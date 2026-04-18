package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/freewhen/backend/internal/model"
)

func (s *Store) CreateCalendar(ctx context.Context, name, emoji string) (model.Calendar, error) {
	const maxRetries = 3
	for i := 0; i < maxRetries; i++ {
		id := generateID(8)
		var cal model.Calendar
		var emojiCol sql.NullString
		err := s.db.QueryRowContext(ctx,
			`INSERT INTO calendars (id, name, emoji) VALUES ($1, $2, $3) RETURNING id, name, emoji, created_at`,
			id, name, nullableString(emoji),
		).Scan(&cal.ID, &cal.Name, &emojiCol, &cal.CreatedAt)
		if err != nil {
			if isUniqueViolation(err) {
				continue
			}
			return model.Calendar{}, fmt.Errorf("insert calendar: %w", err)
		}
		cal.Emoji = emojiCol.String
		return cal, nil
	}
	return model.Calendar{}, fmt.Errorf("failed to generate unique calendar ID after %d attempts", maxRetries)
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (s *Store) GetCalendar(ctx context.Context, id string) (model.CalendarWithDetails, error) {
	var cal model.Calendar
	var emojiCol sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, emoji, created_at FROM calendars WHERE id = $1`, id,
	).Scan(&cal.ID, &cal.Name, &emojiCol, &cal.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.CalendarWithDetails{}, ErrNotFound
		}
		return model.CalendarWithDetails{}, fmt.Errorf("get calendar: %w", err)
	}
	cal.Emoji = emojiCol.String

	participants, err := s.getParticipants(ctx, id)
	if err != nil {
		return model.CalendarWithDetails{}, fmt.Errorf("get participants: %w", err)
	}

	availability, err := s.getAvailability(ctx, id)
	if err != nil {
		return model.CalendarWithDetails{}, fmt.Errorf("get availability: %w", err)
	}

	return model.CalendarWithDetails{
		Calendar:     cal,
		Participants: participants,
		Availability: availability,
	}, nil
}

func (s *Store) getParticipants(ctx context.Context, calendarID string) ([]model.Participant, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, calendar_id, name, colour FROM participants WHERE calendar_id = $1`, calendarID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	participants := []model.Participant{}
	for rows.Next() {
		var p model.Participant
		if err := rows.Scan(&p.ID, &p.CalendarID, &p.Name, &p.Colour); err != nil {
			return nil, err
		}
		participants = append(participants, p)
	}
	return participants, rows.Err()
}

func (s *Store) getAvailability(ctx context.Context, calendarID string) ([]model.Availability, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT participant_id, calendar_id, date::text FROM availability WHERE calendar_id = $1`, calendarID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	availability := []model.Availability{}
	for rows.Next() {
		var a model.Availability
		if err := rows.Scan(&a.ParticipantID, &a.CalendarID, &a.Date); err != nil {
			return nil, err
		}
		availability = append(availability, a)
	}
	return availability, rows.Err()
}
