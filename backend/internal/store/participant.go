package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

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

// normaliseEmail trims whitespace and lowercases the supplied address. Returns
// nil if the result is empty (so callers can store NULL for blank input).
// Matches the magic-link normalisation in handler/auth.go and migration 023.
func normaliseEmail(email *string) *string {
	if email == nil {
		return nil
	}
	v := strings.ToLower(strings.TrimSpace(*email))
	if v == "" {
		return nil
	}
	return &v
}

func (s *Store) AddParticipant(ctx context.Context, calendarID, name string, userID *string, email *string) (model.Participant, error) {
	return s.AddParticipantWithTokenHash(ctx, calendarID, name, userID, email, nil)
}

// AddParticipantWithTokenHash inserts a participant and, in the same INSERT,
// stores the SHA-256 hex digest of its participant token (anonymous joiners).
// A nil tokenHash leaves the column NULL — used for user-linked participants,
// which authorize via session instead.
func (s *Store) AddParticipantWithTokenHash(ctx context.Context, calendarID, name string, userID *string, email *string, tokenHash *string) (model.Participant, error) {
	var tokenHashVal any
	if tokenHash != nil {
		tokenHashVal = *tokenHash
	}
	var exists bool
	var finalisedDate sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT TRUE, finalised_date::text FROM calendars WHERE id = $1 AND deleted_at IS NULL`, calendarID,
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

	var userIDVal any
	if userID != nil {
		userIDVal = *userID
	}
	var emailVal any
	if normalised := normaliseEmail(email); normalised != nil {
		emailVal = *normalised
	}

	const maxRetries = 3
	for i := 0; i < maxRetries; i++ {
		id := generateID(8)
		var p model.Participant
		var scannedUserID sql.NullString
		var scannedEmail sql.NullString
		err := s.db.QueryRowContext(ctx,
			`INSERT INTO participants (id, calendar_id, name, colour, user_id, email, token_hash) VALUES ($1, $2, $3, $4, $5, $6, $7)
			 RETURNING id, calendar_id, name, colour, user_id, email`,
			id, calendarID, name, colour, userIDVal, emailVal, tokenHashVal,
		).Scan(&p.ID, &p.CalendarID, &p.Name, &p.Colour, &scannedUserID, &scannedEmail)
		if err != nil {
			if isUniqueViolation(err) {
				if isDuplicateName(err) {
					return model.Participant{}, ErrDuplicateName
				}
				if isUserAlreadyJoined(err) {
					return model.Participant{}, ErrUserAlreadyJoined
				}
				continue // ID collision, retry
			}
			return model.Participant{}, fmt.Errorf("insert participant: %w", err)
		}
		if scannedUserID.Valid {
			p.UserID = &scannedUserID.String
		}
		if scannedEmail.Valid {
			e := scannedEmail.String
			p.Email = &e
		}
		// Best-effort auto-match against the owner's invite list. A failure here
		// shouldn't fail the join — the owner can manually relink.
		_ = s.MatchInviteByName(ctx, calendarID, p.ID, p.Name)
		return p, nil
	}
	return model.Participant{}, fmt.Errorf("failed to generate unique participant ID after %d attempts", maxRetries)
}

func (s *Store) GetParticipant(ctx context.Context, id string) (model.Participant, error) {
	var p model.Participant
	var userID sql.NullString
	var email sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, calendar_id, name, colour, user_id, email FROM participants WHERE id = $1 AND deleted_at IS NULL`, id,
	).Scan(&p.ID, &p.CalendarID, &p.Name, &p.Colour, &userID, &email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Participant{}, ErrNotFound
		}
		return model.Participant{}, fmt.Errorf("get participant: %w", err)
	}
	if userID.Valid {
		p.UserID = &userID.String
	}
	if email.Valid {
		e := email.String
		p.Email = &e
	}
	return p, nil
}

// GetParticipantForAuth loads a live participant plus its stored token hash
// (nil for user-linked rows). Used only by the
// service's participant-write authorization; the hash never leaves the
// backend.
func (s *Store) GetParticipantForAuth(ctx context.Context, id string) (model.Participant, *string, error) {
	var p model.Participant
	var userID, email, tokenHash sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, calendar_id, name, colour, user_id, email, token_hash FROM participants WHERE id = $1 AND deleted_at IS NULL`, id,
	).Scan(&p.ID, &p.CalendarID, &p.Name, &p.Colour, &userID, &email, &tokenHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Participant{}, nil, ErrNotFound
		}
		return model.Participant{}, nil, fmt.Errorf("get participant for auth: %w", err)
	}
	if userID.Valid {
		p.UserID = &userID.String
	}
	if email.Valid {
		e := email.String
		p.Email = &e
	}
	if tokenHash.Valid {
		h := tokenHash.String
		return p, &h, nil
	}
	return p, nil, nil
}

func (s *Store) ClaimParticipant(ctx context.Context, calendarID, participantID, userID string) (model.Participant, error) {
	var p model.Participant
	var scannedUserID sql.NullString
	var scannedEmail sql.NullString
	err := s.db.QueryRowContext(ctx,
		`UPDATE participants SET user_id = $1
		 WHERE id = $2 AND calendar_id = $3 AND user_id IS NULL AND deleted_at IS NULL
		 RETURNING id, calendar_id, name, colour, user_id, email`,
		userID, participantID, calendarID,
	).Scan(&p.ID, &p.CalendarID, &p.Name, &p.Colour, &scannedUserID, &scannedEmail)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Either participant doesn't exist, wrong calendar, or already linked.
			// Check which case it is.
			var existingUserID sql.NullString
			lookupErr := s.db.QueryRowContext(ctx,
				`SELECT user_id FROM participants WHERE id = $1 AND calendar_id = $2 AND deleted_at IS NULL`,
				participantID, calendarID,
			).Scan(&existingUserID)
			if errors.Is(lookupErr, sql.ErrNoRows) {
				return model.Participant{}, ErrNotFound
			}
			if lookupErr == nil && existingUserID.Valid {
				return model.Participant{}, ErrAlreadyLinked
			}
			return model.Participant{}, ErrNotFound
		}
		if isUniqueViolation(err) && isUserAlreadyJoined(err) {
			return model.Participant{}, ErrUserAlreadyJoined
		}
		return model.Participant{}, fmt.Errorf("claim participant: %w", err)
	}
	if scannedUserID.Valid {
		p.UserID = &scannedUserID.String
	}
	if scannedEmail.Valid {
		e := scannedEmail.String
		p.Email = &e
	}
	return p, nil
}

// UpdateParticipantName renames a participant scoped to a calendar. Returns
// ErrNotFound if the participant does not exist on the calendar, and
// ErrDuplicateName if another participant on the same calendar already has
// the requested name.
func (s *Store) UpdateParticipantName(ctx context.Context, calendarID, participantID, name string) (model.Participant, error) {
	var p model.Participant
	var scannedUserID sql.NullString
	var scannedEmail sql.NullString
	err := s.db.QueryRowContext(ctx,
		`UPDATE participants SET name = $1
		 WHERE id = $2 AND calendar_id = $3 AND deleted_at IS NULL
		 RETURNING id, calendar_id, name, colour, user_id, email`,
		name, participantID, calendarID,
	).Scan(&p.ID, &p.CalendarID, &p.Name, &p.Colour, &scannedUserID, &scannedEmail)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Participant{}, ErrNotFound
		}
		if isDuplicateName(err) {
			return model.Participant{}, ErrDuplicateName
		}
		return model.Participant{}, fmt.Errorf("update participant name: %w", err)
	}
	if scannedUserID.Valid {
		p.UserID = &scannedUserID.String
	}
	if scannedEmail.Valid {
		e := scannedEmail.String
		p.Email = &e
	}
	// Rename may bring the participant's name in line with an unmatched invite.
	// Re-run auto-match best-effort.
	_ = s.MatchInviteByName(ctx, calendarID, p.ID, p.Name)
	return p, nil
}

// UpdateParticipantEmail sets the participant's contact email. Pass a pointer
// to "" (or nil) to clear it. Returns ErrNotFound if the row does not exist.
func (s *Store) UpdateParticipantEmail(ctx context.Context, calendarID, participantID string, email *string) (model.Participant, error) {
	var emailVal any
	if normalised := normaliseEmail(email); normalised != nil {
		emailVal = *normalised
	}

	var p model.Participant
	var scannedUserID sql.NullString
	var scannedEmail sql.NullString
	err := s.db.QueryRowContext(ctx,
		`UPDATE participants SET email = $1
		 WHERE id = $2 AND calendar_id = $3 AND deleted_at IS NULL
		 RETURNING id, calendar_id, name, colour, user_id, email`,
		emailVal, participantID, calendarID,
	).Scan(&p.ID, &p.CalendarID, &p.Name, &p.Colour, &scannedUserID, &scannedEmail)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Participant{}, ErrNotFound
		}
		return model.Participant{}, fmt.Errorf("update participant email: %w", err)
	}
	if scannedUserID.Valid {
		p.UserID = &scannedUserID.String
	}
	if scannedEmail.Valid {
		e := scannedEmail.String
		p.Email = &e
	}
	return p, nil
}

// ClearParticipantEmailByID nulls the email column for the given participant
// id, regardless of calendar. Used by the unsubscribe flow where the only
// trustworthy identifier in the URL is the HMAC-signed participant id.
// Idempotent: clearing an already-null row succeeds.
func (s *Store) ClearParticipantEmailByID(ctx context.Context, participantID string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE participants SET email = NULL WHERE id = $1 AND deleted_at IS NULL`,
		participantID,
	)
	if err != nil {
		return fmt.Errorf("clear participant email: %w", err)
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

// DeleteParticipant soft-deletes a participant from a calendar. Availability
// rows are preserved on disk but hidden by the deleted_at filter on the read
// path. Any invite that was matched to this participant is unmatched so the
// owner sees them back in the "missing" list. Returns ErrNotFound if no live
// participant matches.
func (s *Store) DeleteParticipant(ctx context.Context, calendarID, participantID string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE participants SET deleted_at = now()
		 WHERE id = $1 AND calendar_id = $2 AND deleted_at IS NULL`,
		participantID, calendarID,
	)
	if err != nil {
		return fmt.Errorf("delete participant: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	if _, err := s.db.ExecContext(ctx,
		`UPDATE calendar_invites SET matched_participant_id = NULL WHERE matched_participant_id = $1`,
		participantID,
	); err != nil {
		return fmt.Errorf("clear invite match: %w", err)
	}
	return nil
}

// SoftDeleteParticipantsForUser marks every live participant row for the
// given (calendar_id, user_id) as deleted. Used by the Leave-calendar flow.
// Idempotent — returns nil whether or not any rows were affected. Returns
// the number of rows affected so callers can publish realtime events when
// a leave actually happened.
func (s *Store) SoftDeleteParticipantsForUser(ctx context.Context, calendarID, userID string) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE participants SET deleted_at = now()
		 WHERE calendar_id = $1 AND user_id = $2 AND deleted_at IS NULL`,
		calendarID, userID,
	)
	if err != nil {
		return 0, fmt.Errorf("soft delete participants for user: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}
	if rows > 0 {
		// Drop any invite matches pointing at the participants we just
		// soft-deleted so the owner sees them back in the "missing" list.
		if _, err := s.db.ExecContext(ctx,
			`UPDATE calendar_invites SET matched_participant_id = NULL
			 WHERE matched_participant_id IN (
			   SELECT id FROM participants WHERE calendar_id = $1 AND user_id = $2 AND deleted_at IS NOT NULL
			 )`,
			calendarID, userID,
		); err != nil {
			return rows, fmt.Errorf("clear invite matches: %w", err)
		}
	}
	return rows, nil
}

// insertParticipantTx inserts a participant row under the given transaction,
// retrying on ID collision. The colour is picked from the palette by
// paletteIndex modulo palette length — callers supplying sequential indexes
// get the same round-robin assignment as sequential AddParticipant calls.
// Used by DuplicateCalendar to copy a whole participant list atomically.
func insertParticipantTx(ctx context.Context, tx *sql.Tx, calendarID, name string, userID *string, paletteIndex int) (model.Participant, error) {
	colour := colourPalette[paletteIndex%len(colourPalette)]
	var userIDVal any
	if userID != nil {
		userIDVal = *userID
	}
	const maxRetries = 3
	for i := 0; i < maxRetries; i++ {
		id := generateID(8)
		var p model.Participant
		var scannedUserID sql.NullString
		var scannedEmail sql.NullString
		err := tx.QueryRowContext(ctx,
			`INSERT INTO participants (id, calendar_id, name, colour, user_id) VALUES ($1, $2, $3, $4, $5)
			 RETURNING id, calendar_id, name, colour, user_id, email`,
			id, calendarID, name, colour, userIDVal,
		).Scan(&p.ID, &p.CalendarID, &p.Name, &p.Colour, &scannedUserID, &scannedEmail)
		if err != nil {
			if isUniqueViolation(err) {
				if isDuplicateName(err) {
					return model.Participant{}, ErrDuplicateName
				}
				if isUserAlreadyJoined(err) {
					return model.Participant{}, ErrUserAlreadyJoined
				}
				continue
			}
			return model.Participant{}, err
		}
		if scannedUserID.Valid {
			p.UserID = &scannedUserID.String
		}
		if scannedEmail.Valid {
			e := scannedEmail.String
			p.Email = &e
		}
		return p, nil
	}
	return model.Participant{}, fmt.Errorf("failed to generate unique participant ID after %d attempts", maxRetries)
}

func (s *Store) participantCount(ctx context.Context, calendarID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM participants WHERE calendar_id = $1 AND deleted_at IS NULL`, calendarID,
	).Scan(&count)
	return count, err
}

func isUserAlreadyJoined(err error) bool {
	return isUniqueViolation(err) && strings.Contains(err.Error(), "idx_participants_calendar_user_active")
}
