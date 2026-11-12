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
		id := generateID(CalendarIDLength)
		var cal model.Calendar
		var emojiCol sql.NullString
		err := s.db.QueryRowContext(ctx,
			`INSERT INTO calendars (id, name, emoji, owner_id) VALUES ($1, $2, $3, $4) RETURNING id, name, emoji, owner_id, created_at, updated_at, access_epoch`,
			id, name, nullableString(emoji), ownerID,
		).Scan(&cal.ID, &cal.Name, &emojiCol, &cal.OwnerID, &cal.CreatedAt, &cal.UpdatedAt, &cal.AccessEpoch)
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

// CountCalendarParticipants returns the number of participants in the given calendar.
func (s *Store) CountCalendarParticipants(ctx context.Context, calendarID string) (int, error) {
	return s.participantCount(ctx, calendarID)
}

// CountDuplicableParticipants counts the live participants DuplicateCalendar
// would copy from calendarID for newOwnerID: user-linked rows other than the
// new owner's own (anonymous rows are never copied — see DuplicateCalendar).
// Keep the predicate in sync with DuplicateCalendar's copy query.
func (s *Store) CountDuplicableParticipants(ctx context.Context, calendarID, newOwnerID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM participants
		 WHERE calendar_id = $1 AND deleted_at IS NULL
		   AND user_id IS NOT NULL AND user_id <> $2`,
		calendarID, newOwnerID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count duplicable participants: %w", err)
	}
	return count, nil
}

// CountCalendarsForOwner returns the number of live calendars owned by the
// given user. Used by the service layer to enforce the per-owner calendar
// cap — soft-deleted calendars don't count, so deleting frees up quota.
func (s *Store) CountCalendarsForOwner(ctx context.Context, ownerID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM calendars WHERE owner_id = $1 AND deleted_at IS NULL`, ownerID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count calendars for owner: %w", err)
	}
	return count, nil
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
	var passphrase sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, emoji, owner_id, created_at, updated_at, finalised_date::text, finalised_at, passphrase, access_epoch
		 FROM calendars WHERE id = $1 AND deleted_at IS NULL`, id,
	).Scan(&cal.ID, &cal.Name, &emojiCol, &cal.OwnerID, &cal.CreatedAt, &cal.UpdatedAt, &finalisedDate, &finalisedAt, &passphrase, &cal.AccessEpoch)
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
	if passphrase.Valid {
		cal.Passphrase = passphrase.String
	}
	return cal, nil
}

// SetCalendarPassphrase sets or clears the passphrase column and bumps
// access_epoch by 1 in the same UPDATE. Pass nil to clear. The epoch bump
// applies even when clearing so any outstanding tokens are revoked
// immediately. Returns ErrNotFound if the calendar does not exist.
func (s *Store) SetCalendarPassphrase(ctx context.Context, calendarID string, passphrase *string) (model.Calendar, error) {
	var passArg any
	if passphrase == nil {
		passArg = nil
	} else {
		passArg = *passphrase
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE calendars SET passphrase = $1, access_epoch = access_epoch + 1, updated_at = now() WHERE id = $2 AND deleted_at IS NULL`,
		passArg, calendarID,
	)
	if err != nil {
		return model.Calendar{}, fmt.Errorf("set calendar passphrase: %w", err)
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

// UpdateCalendarName renames a calendar. Returns ErrNotFound if the calendar does not exist.
func (s *Store) UpdateCalendarName(ctx context.Context, calendarID, name string) (model.Calendar, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE calendars SET name = $1, updated_at = now() WHERE id = $2 AND deleted_at IS NULL`,
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

// SetFinalisedDate locks the calendar to a single date. Replaces any previous value.
// Returns ErrNotFound if the calendar does not exist.
func (s *Store) SetFinalisedDate(ctx context.Context, calendarID, date string) (model.Calendar, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE calendars SET finalised_date = $1, finalised_at = $2, updated_at = now() WHERE id = $3 AND deleted_at IS NULL`,
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
		`UPDATE calendars SET finalised_date = NULL, finalised_at = NULL, updated_at = now() WHERE id = $1 AND deleted_at IS NULL`,
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
		`UPDATE calendars SET updated_at = now() WHERE id = $1 AND deleted_at IS NULL`, calendarID,
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

// GetCalendarLastNudgeAt returns the timestamp of the most recent
// owner-triggered nudge for a calendar, or nil if none has been sent.
// Returns ErrNotFound when the calendar does not exist.
func (s *Store) GetCalendarLastNudgeAt(ctx context.Context, calendarID string) (*time.Time, error) {
	var last sql.NullTime
	err := s.db.QueryRowContext(ctx,
		`SELECT last_nudge_at FROM calendars WHERE id = $1 AND deleted_at IS NULL`, calendarID,
	).Scan(&last)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get last_nudge_at: %w", err)
	}
	if !last.Valid {
		return nil, nil
	}
	t := last.Time
	return &t, nil
}

// SetCalendarLastNudgeAt records the time the last nudge was sent.
func (s *Store) SetCalendarLastNudgeAt(ctx context.Context, calendarID string, t time.Time) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE calendars SET last_nudge_at = $1 WHERE id = $2 AND deleted_at IS NULL`, t, calendarID,
	)
	if err != nil {
		return fmt.Errorf("set last_nudge_at: %w", err)
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

// SoftDeleteCalendar marks a calendar as deleted by setting deleted_at to
// now() if it is currently live. Idempotent — already-deleted or missing
// rows return nil without error.
func (s *Store) SoftDeleteCalendar(ctx context.Context, calendarID string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE calendars SET deleted_at = now(), updated_at = now()
		 WHERE id = $1 AND deleted_at IS NULL`, calendarID,
	)
	if err != nil {
		return fmt.Errorf("soft delete calendar: %w", err)
	}
	return nil
}

// IsCalendarSoftDeleted reports whether the row exists in the calendars
// table but has a non-null deleted_at. Returns false (and nil) if the row
// is missing entirely. Used by mutation handlers to distinguish 404 (never
// existed / typo) from 410 (existed once, gone now).
func (s *Store) IsCalendarSoftDeleted(ctx context.Context, calendarID string) (bool, error) {
	var deletedAt sql.NullTime
	err := s.db.QueryRowContext(ctx,
		`SELECT deleted_at FROM calendars WHERE id = $1`, calendarID,
	).Scan(&deletedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("check calendar soft-delete: %w", err)
	}
	return deletedAt.Valid, nil
}

// ListCalendarsForUser returns calendars where the given user is a participant,
// sorted most-recently-active first. Limit caps the result set. The caller is
// responsible for passing limit = pageSize + 1 when detecting has_more.
func (s *Store) ListCalendarsForUser(ctx context.Context, userID string, limit int) ([]model.CalendarListItem, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT c.id, c.name, c.emoji, c.owner_id, c.created_at, c.updated_at,
		        c.finalised_date::text, c.finalised_at,
		        (SELECT COUNT(*) FROM participants WHERE calendar_id = c.id AND deleted_at IS NULL) AS participant_count
		 FROM calendars c
		 JOIN participants p ON p.calendar_id = c.id
		 WHERE p.user_id = $1 AND p.deleted_at IS NULL AND c.deleted_at IS NULL
		 ORDER BY c.updated_at DESC, c.id DESC
		 LIMIT $2`,
		userID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list calendars for user: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := []model.CalendarListItem{}
	for rows.Next() {
		var item model.CalendarListItem
		var emojiCol sql.NullString
		var finalisedDate sql.NullString
		var finalisedAt sql.NullTime
		if err := rows.Scan(
			&item.ID, &item.Name, &emojiCol, &item.OwnerID,
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

// DuplicateCalendar creates a new calendar copied from sourceID.
//
// Copies name (or overrideName if non-empty) and emoji. Does NOT
// copy availability, finalised state, or passphrase. The owner is auto-joined
// as a participant. When includeParticipants is true, every non-owner source
// participant is recreated as a stub (same name, same user_id, new id).
//
// The whole operation runs in a single transaction so a failed participant
// insert (e.g. hitting the per-calendar cap) rolls the new calendar back
// rather than leaving a half-duplicated calendar behind.
func (s *Store) DuplicateCalendar(
	ctx context.Context,
	sourceID, newOwnerID, overrideName, ownerParticipantName string,
	includeParticipants bool,
) (model.Calendar, model.Participant, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Calendar{}, model.Participant{}, fmt.Errorf("begin tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	// Load source within the tx.
	var source model.Calendar
	var emojiCol sql.NullString
	var finalisedDate sql.NullString
	var finalisedAt sql.NullTime
	var passphrase sql.NullString
	err = tx.QueryRowContext(ctx,
		`SELECT id, name, emoji, owner_id, created_at, updated_at, finalised_date::text, finalised_at, passphrase, access_epoch
		 FROM calendars WHERE id = $1 AND deleted_at IS NULL`, sourceID,
	).Scan(&source.ID, &source.Name, &emojiCol, &source.OwnerID, &source.CreatedAt, &source.UpdatedAt, &finalisedDate, &finalisedAt, &passphrase, &source.AccessEpoch)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Calendar{}, model.Participant{}, ErrNotFound
		}
		return model.Calendar{}, model.Participant{}, fmt.Errorf("get source calendar: %w", err)
	}
	source.Emoji = emojiCol.String

	// Service layer supplies the final name (including "(copy)" suffix and
	// length clipping). Store trusts the caller.
	newName := overrideName

	// Insert new calendar with retry on ID collision.
	var newCal model.Calendar
	const maxRetries = 3
	for i := 0; i < maxRetries; i++ {
		id := generateID(CalendarIDLength)
		var emojiOut sql.NullString
		err = tx.QueryRowContext(ctx,
			`INSERT INTO calendars (id, name, emoji, owner_id) VALUES ($1, $2, $3, $4)
			 RETURNING id, name, emoji, owner_id, created_at, updated_at, access_epoch`,
			id, newName, nullableString(source.Emoji), newOwnerID,
		).Scan(&newCal.ID, &newCal.Name, &emojiOut, &newCal.OwnerID, &newCal.CreatedAt, &newCal.UpdatedAt, &newCal.AccessEpoch)
		if err == nil {
			newCal.Emoji = emojiOut.String
			break
		}
		if isUniqueViolation(err) && i < maxRetries-1 {
			continue
		}
		return model.Calendar{}, model.Participant{}, fmt.Errorf("insert duplicate calendar: %w", err)
	}

	// Insert owner participant (first row → first colour in palette).
	ownerParticipant, err := insertParticipantTx(ctx, tx, newCal.ID, ownerParticipantName, &newOwnerID, 0)
	if err != nil {
		return model.Calendar{}, model.Participant{}, fmt.Errorf("insert owner participant: %w", err)
	}

	if includeParticipants {
		// Copy only user-linked participants. Anonymous rows are authorized by
		// a participant token issued at join; a copied row would have no token
		// (uneditable forever) while squatting on the name and blocking the
		// real person from rejoining. They can rejoin the copy via its link.
		rows, err := tx.QueryContext(ctx,
			`SELECT id, name, user_id FROM participants
			 WHERE calendar_id = $1 AND deleted_at IS NULL AND user_id IS NOT NULL
			 ORDER BY id`, sourceID,
		)
		if err != nil {
			return model.Calendar{}, model.Participant{}, fmt.Errorf("list source participants: %w", err)
		}
		type src struct {
			name   string
			userID string
		}
		var sources []src
		for rows.Next() {
			var id, name, userID string
			if err := rows.Scan(&id, &name, &userID); err != nil {
				_ = rows.Close()
				return model.Calendar{}, model.Participant{}, fmt.Errorf("scan source participant: %w", err)
			}
			sources = append(sources, src{name: name, userID: userID})
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return model.Calendar{}, model.Participant{}, fmt.Errorf("iterate source participants: %w", err)
		}
		_ = rows.Close()

		idx := 1 // owner was index 0
		for _, p := range sources {
			// Skip the source owner row — the new owner is already inserted.
			if p.userID == newOwnerID {
				continue
			}
			uid := p.userID
			if _, err := insertParticipantTx(ctx, tx, newCal.ID, p.name, &uid, idx); err != nil {
				return model.Calendar{}, model.Participant{}, fmt.Errorf("copy participant: %w", err)
			}
			idx++
		}
	}

	if err := tx.Commit(); err != nil {
		return model.Calendar{}, model.Participant{}, fmt.Errorf("commit tx: %w", err)
	}
	committed = true
	return newCal, ownerParticipant, nil
}

func (s *Store) getParticipants(ctx context.Context, calendarID string) ([]model.Participant, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, calendar_id, name, colour, user_id, email FROM participants WHERE calendar_id = $1 AND deleted_at IS NULL`, calendarID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	participants := []model.Participant{}
	for rows.Next() {
		var p model.Participant
		var userID sql.NullString
		var email sql.NullString
		if err := rows.Scan(&p.ID, &p.CalendarID, &p.Name, &p.Colour, &userID, &email); err != nil {
			return nil, err
		}
		if userID.Valid {
			p.UserID = &userID.String
		}
		if email.Valid {
			e := email.String
			p.Email = &e
		}
		participants = append(participants, p)
	}
	return participants, rows.Err()
}

func (s *Store) getAvailability(ctx context.Context, calendarID string) ([]model.Availability, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT a.participant_id, a.calendar_id, a.date::text, a.note, a.preferred
		 FROM availability a
		 JOIN participants p ON p.id = a.participant_id
		 WHERE a.calendar_id = $1 AND p.deleted_at IS NULL`, calendarID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	availability := []model.Availability{}
	for rows.Next() {
		var a model.Availability
		var note sql.NullString
		if err := rows.Scan(&a.ParticipantID, &a.CalendarID, &a.Date, &note, &a.Preferred); err != nil {
			return nil, err
		}
		if note.Valid {
			n := note.String
			a.Note = &n
		}
		availability = append(availability, a)
	}
	return availability, rows.Err()
}
