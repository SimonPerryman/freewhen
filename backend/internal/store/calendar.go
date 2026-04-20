package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/freewhen/backend/internal/model"
)

func (s *Store) CreateCalendar(ctx context.Context, name, emoji, ownerID string) (model.Calendar, error) {
	const maxRetries = 3
	for i := 0; i < maxRetries; i++ {
		id := generateID(8)
		var cal model.Calendar
		var emojiCol sql.NullString
		err := s.db.QueryRowContext(ctx,
			`INSERT INTO calendars (id, name, emoji, owner_id) VALUES ($1, $2, $3, $4) RETURNING id, name, emoji, owner_id, created_at`,
			id, name, nullableString(emoji), ownerID,
		).Scan(&cal.ID, &cal.Name, &emojiCol, &cal.OwnerID, &cal.CreatedAt)
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
	cal, err := s.getCalendarRow(ctx, id)
	if err != nil {
		return model.CalendarWithDetails{}, err
	}

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

// GetCalendarOnly returns the calendar row without details. Used by handlers
// that need ownership/finalised checks without paying for participants+availability.
func (s *Store) GetCalendarOnly(ctx context.Context, id string) (model.Calendar, error) {
	return s.getCalendarRow(ctx, id)
}

func (s *Store) getCalendarRow(ctx context.Context, id string) (model.Calendar, error) {
	var cal model.Calendar
	var emojiCol sql.NullString
	var finalisedDate sql.NullString
	var finalisedAt sql.NullTime
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, emoji, owner_id, created_at, finalised_date::text, finalised_at
		 FROM calendars WHERE id = $1`, id,
	).Scan(&cal.ID, &cal.Name, &emojiCol, &cal.OwnerID, &cal.CreatedAt, &finalisedDate, &finalisedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Calendar{}, ErrNotFound
		}
		return model.Calendar{}, fmt.Errorf("get calendar: %w", err)
	}
	cal.Emoji = emojiCol.String
	if finalisedDate.Valid {
		d := finalisedDate.String
		cal.FinalisedDate = &d
	}
	if finalisedAt.Valid {
		t := finalisedAt.Time
		cal.FinalisedAt = &t
	}
	return cal, nil
}

// SetFinalisedDate locks the calendar to a single date. Replaces any previous value.
// Returns ErrNotFound if the calendar does not exist.
func (s *Store) SetFinalisedDate(ctx context.Context, calendarID, date string) (model.Calendar, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE calendars SET finalised_date = $1, finalised_at = $2 WHERE id = $3`,
		date, time.Now().UTC(), calendarID,
	)
	if err != nil {
		return model.Calendar{}, fmt.Errorf("set finalised date: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return model.Calendar{}, fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return model.Calendar{}, ErrNotFound
	}
	return s.getCalendarRow(ctx, calendarID)
}

// ClearFinalisedDate reopens a finalised calendar. Idempotent: clearing an
// already-unfinalised calendar is a no-op that still returns the calendar.
// Returns ErrNotFound if the calendar does not exist.
func (s *Store) ClearFinalisedDate(ctx context.Context, calendarID string) (model.Calendar, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE calendars SET finalised_date = NULL, finalised_at = NULL WHERE id = $1`,
		calendarID,
	)
	if err != nil {
		return model.Calendar{}, fmt.Errorf("clear finalised date: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return model.Calendar{}, fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return model.Calendar{}, ErrNotFound
	}
	return s.getCalendarRow(ctx, calendarID)
}

func (s *Store) getParticipants(ctx context.Context, calendarID string) ([]model.Participant, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, calendar_id, name, colour, user_id FROM participants WHERE calendar_id = $1`, calendarID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	participants := []model.Participant{}
	for rows.Next() {
		var p model.Participant
		var userID sql.NullString
		if err := rows.Scan(&p.ID, &p.CalendarID, &p.Name, &p.Colour, &userID); err != nil {
			return nil, err
		}
		if userID.Valid {
			p.UserID = &userID.String
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
	defer func() { _ = rows.Close() }()

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
