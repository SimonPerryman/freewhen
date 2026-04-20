package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/freewhen/backend/internal/model"
)

func (s *Store) CreateCalendar(ctx context.Context, name, emoji, ownerID, visibility string) (model.Calendar, error) {
	const maxRetries = 3
	for i := 0; i < maxRetries; i++ {
		id := generateID(8)
		var cal model.Calendar
		var emojiCol sql.NullString
		err := s.db.QueryRowContext(ctx,
			`INSERT INTO calendars (id, name, emoji, owner_id, visibility) VALUES ($1, $2, $3, $4, $5) RETURNING id, name, emoji, owner_id, visibility, created_at, updated_at`,
			id, name, nullableString(emoji), ownerID, visibility,
		).Scan(&cal.ID, &cal.Name, &emojiCol, &cal.OwnerID, &cal.Visibility, &cal.CreatedAt, &cal.UpdatedAt)
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
		Calendar:         cal,
		Participants:     participants,
		Availability:     availability,
		ParticipantCount: len(participants),
	}, nil
}

// CountCalendarParticipants returns the number of participants in the given calendar.
func (s *Store) CountCalendarParticipants(ctx context.Context, calendarID string) (int, error) {
	return s.participantCount(ctx, calendarID)
}

// CountCalendarsForOwner returns the number of calendars owned by the given user.
// Used by the service layer to enforce the per-owner calendar cap.
func (s *Store) CountCalendarsForOwner(ctx context.Context, ownerID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM calendars WHERE owner_id = $1`, ownerID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count calendars for owner: %w", err)
	}
	return count, nil
}

// ParticipantHasAvailability reports whether the participant has any availability rows.
func (s *Store) ParticipantHasAvailability(ctx context.Context, participantID string) (bool, error) {
	var has bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM availability WHERE participant_id = $1)`,
		participantID,
	).Scan(&has)
	if err != nil {
		return false, fmt.Errorf("participant has availability: %w", err)
	}
	return has, nil
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
		`SELECT id, name, emoji, owner_id, visibility, created_at, updated_at, finalised_date::text, finalised_at
		 FROM calendars WHERE id = $1`, id,
	).Scan(&cal.ID, &cal.Name, &emojiCol, &cal.OwnerID, &cal.Visibility, &cal.CreatedAt, &cal.UpdatedAt, &finalisedDate, &finalisedAt)
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

// UpdateCalendarName renames a calendar. Returns ErrNotFound if the calendar does not exist.
func (s *Store) UpdateCalendarName(ctx context.Context, calendarID, name string) (model.Calendar, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE calendars SET name = $1, updated_at = now() WHERE id = $2`,
		name, calendarID,
	)
	if err != nil {
		return model.Calendar{}, fmt.Errorf("update calendar name: %w", err)
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

// UpdateCalendarVisibility sets the visibility mode on a calendar.
// Returns ErrNotFound if the calendar does not exist.
func (s *Store) UpdateCalendarVisibility(ctx context.Context, calendarID, visibility string) (model.Calendar, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE calendars SET visibility = $1, updated_at = now() WHERE id = $2`,
		visibility, calendarID,
	)
	if err != nil {
		return model.Calendar{}, fmt.Errorf("update calendar visibility: %w", err)
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

// SetFinalisedDate locks the calendar to a single date. Replaces any previous value.
// Returns ErrNotFound if the calendar does not exist.
func (s *Store) SetFinalisedDate(ctx context.Context, calendarID, date string) (model.Calendar, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE calendars SET finalised_date = $1, finalised_at = $2, updated_at = now() WHERE id = $3`,
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
		`UPDATE calendars SET finalised_date = NULL, finalised_at = NULL, updated_at = now() WHERE id = $1`,
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

// TouchCalendar bumps updated_at to now(). Used by mutations that touch child
// tables (participants, availability) so the parent calendar's sort key stays
// current. Returns ErrNotFound if the calendar does not exist.
func (s *Store) TouchCalendar(ctx context.Context, calendarID string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE calendars SET updated_at = now() WHERE id = $1`, calendarID,
	)
	if err != nil {
		return fmt.Errorf("touch calendar: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// CalendarListItem is a calendar plus its participant count, returned by
// ListCalendarsForUser. The caller derives per-user fields (role, etc.) from
// the embedded Calendar.
type CalendarListItem struct {
	model.Calendar
	ParticipantCount int
}

// ListCalendarsForUser returns calendars where the given user is a participant,
// sorted most-recently-active first. Limit caps the result set. The caller is
// responsible for passing limit = pageSize + 1 when detecting has_more.
func (s *Store) ListCalendarsForUser(ctx context.Context, userID string, limit int) ([]CalendarListItem, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT c.id, c.name, c.emoji, c.owner_id, c.visibility, c.created_at, c.updated_at,
		        c.finalised_date::text, c.finalised_at,
		        (SELECT COUNT(*) FROM participants WHERE calendar_id = c.id) AS participant_count
		 FROM calendars c
		 JOIN participants p ON p.calendar_id = c.id
		 WHERE p.user_id = $1
		 ORDER BY c.updated_at DESC, c.id DESC
		 LIMIT $2`,
		userID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list calendars for user: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := []CalendarListItem{}
	for rows.Next() {
		var item CalendarListItem
		var emojiCol sql.NullString
		var finalisedDate sql.NullString
		var finalisedAt sql.NullTime
		if err := rows.Scan(
			&item.ID, &item.Name, &emojiCol, &item.OwnerID, &item.Visibility,
			&item.CreatedAt, &item.UpdatedAt, &finalisedDate, &finalisedAt,
			&item.ParticipantCount,
		); err != nil {
			return nil, fmt.Errorf("scan calendar row: %w", err)
		}
		item.Emoji = emojiCol.String
		if finalisedDate.Valid {
			d := finalisedDate.String
			item.FinalisedDate = &d
		}
		if finalisedAt.Valid {
			t := finalisedAt.Time
			item.FinalisedAt = &t
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate calendar rows: %w", err)
	}
	return items, nil
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
