package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/freewhen/backend/internal/limits"
	"github.com/freewhen/backend/internal/model"
)

// ListInvites returns all invites for a calendar ordered by creation time.
func (s *Store) ListInvites(ctx context.Context, calendarID string) ([]model.Invite, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, calendar_id, name, matched_participant_id, created_at
		 FROM calendar_invites
		 WHERE calendar_id = $1
		 ORDER BY created_at ASC, id ASC`, calendarID,
	)
	if err != nil {
		return nil, fmt.Errorf("list invites: %w", err)
	}
	defer func() { _ = rows.Close() }()

	invites := []model.Invite{}
	for rows.Next() {
		var inv model.Invite
		var matched sql.NullString
		if err := rows.Scan(&inv.ID, &inv.CalendarID, &inv.Name, &matched, &inv.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan invite: %w", err)
		}
		if matched.Valid {
			v := matched.String
			inv.MatchedParticipantID = &v
		}
		invites = append(invites, inv)
	}
	return invites, rows.Err()
}

// GetInvite fetches a single invite by id.
func (s *Store) GetInvite(ctx context.Context, inviteID string) (model.Invite, error) {
	var inv model.Invite
	var matched sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, calendar_id, name, matched_participant_id, created_at
		 FROM calendar_invites WHERE id = $1`, inviteID,
	).Scan(&inv.ID, &inv.CalendarID, &inv.Name, &matched, &inv.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Invite{}, ErrInviteNotFound
		}
		return model.Invite{}, fmt.Errorf("get invite: %w", err)
	}
	if matched.Valid {
		v := matched.String
		inv.MatchedParticipantID = &v
	}
	return inv, nil
}

// AddInvitesResult is the outcome of a bulk invite insert.
type AddInvitesResult struct {
	Added   []model.Invite
	Skipped []string // names that collided with existing invites (reason: duplicate)
}

// AddInvites inserts one or more invites on a calendar in a single transaction.
// Callers have already trimmed and validated each name. Duplicates (by lower(trim(name)))
// against existing rows are returned in Skipped with the original name; duplicates within
// the batch itself are deduped silently on the first occurrence's case, later dupes
// returned as Skipped. Enforces MaxInvitesPerCalendar; returns ErrInviteLimitReached if
// the insert would exceed the cap.
func (s *Store) AddInvites(ctx context.Context, calendarID string, names []string) (AddInvitesResult, error) {
	res := AddInvitesResult{Added: []model.Invite{}, Skipped: []string{}}
	if len(names) == 0 {
		return res, nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return res, fmt.Errorf("begin tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	// Verify calendar exists.
	var exists bool
	err = tx.QueryRowContext(ctx, `SELECT TRUE FROM calendars WHERE id = $1 AND deleted_at IS NULL`, calendarID).Scan(&exists)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return res, ErrNotFound
		}
		return res, fmt.Errorf("look up calendar: %w", err)
	}

	var count int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM calendar_invites WHERE calendar_id = $1`, calendarID,
	).Scan(&count); err != nil {
		return res, fmt.Errorf("count invites: %w", err)
	}

	seenInBatch := map[string]bool{}
	for _, name := range names {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" {
			continue
		}
		if seenInBatch[key] {
			res.Skipped = append(res.Skipped, name)
			continue
		}
		seenInBatch[key] = true

		// Pre-check for collision against existing rows — cheaper than catching
		// the unique-violation, which would abort the enclosing tx.
		var collides bool
		if err := tx.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM calendar_invites WHERE calendar_id = $1 AND LOWER(TRIM(name)) = $2)`,
			calendarID, key,
		).Scan(&collides); err != nil {
			return res, fmt.Errorf("check invite collision: %w", err)
		}
		if collides {
			res.Skipped = append(res.Skipped, name)
			continue
		}

		if count >= limits.MaxInvitesPerCalendar {
			return res, ErrInviteLimitReached
		}

		const maxRetries = 3
		inserted := false
		for i := 0; i < maxRetries; i++ {
			id := "inv_" + generateID(10)
			var inv model.Invite
			var matched sql.NullString
			err := tx.QueryRowContext(ctx,
				`INSERT INTO calendar_invites (id, calendar_id, name) VALUES ($1, $2, $3)
				 RETURNING id, calendar_id, name, matched_participant_id, created_at`,
				id, calendarID, name,
			).Scan(&inv.ID, &inv.CalendarID, &inv.Name, &matched, &inv.CreatedAt)
			if err != nil {
				if isUniqueViolation(err) {
					// Only possible cause now is an ID collision (name collision
					// was pre-checked above). Retry with a new ID.
					continue
				}
				return res, fmt.Errorf("insert invite: %w", err)
			}
			if matched.Valid {
				v := matched.String
				inv.MatchedParticipantID = &v
			}
			res.Added = append(res.Added, inv)
			count++
			inserted = true
			break
		}
		if !inserted {
			return res, fmt.Errorf("failed to generate unique invite id")
		}
	}

	if err := tx.Commit(); err != nil {
		return res, fmt.Errorf("commit tx: %w", err)
	}
	committed = true
	return res, nil
}

// UpdateInviteName renames an invite. Returns ErrInviteNameTaken if another
// invite on the same calendar has the new name (case-insensitive, trimmed).
func (s *Store) UpdateInviteName(ctx context.Context, inviteID, name string) (model.Invite, error) {
	var inv model.Invite
	var matched sql.NullString
	err := s.db.QueryRowContext(ctx,
		`UPDATE calendar_invites SET name = $1 WHERE id = $2
		 RETURNING id, calendar_id, name, matched_participant_id, created_at`,
		name, inviteID,
	).Scan(&inv.ID, &inv.CalendarID, &inv.Name, &matched, &inv.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Invite{}, ErrInviteNotFound
		}
		if isUniqueViolation(err) {
			return model.Invite{}, ErrInviteNameTaken
		}
		return model.Invite{}, fmt.Errorf("update invite: %w", err)
	}
	if matched.Valid {
		v := matched.String
		inv.MatchedParticipantID = &v
	}
	return inv, nil
}

// DeleteInvite removes an invite. Returns ErrInviteNotFound if missing.
func (s *Store) DeleteInvite(ctx context.Context, inviteID string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM calendar_invites WHERE id = $1`, inviteID)
	if err != nil {
		return fmt.Errorf("delete invite: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return ErrInviteNotFound
	}
	return nil
}

// SetInviteMatch links (or unlinks, with participantID == nil) an invite to a
// participant. The caller is expected to have verified that the participant
// belongs to the same calendar as the invite.
func (s *Store) SetInviteMatch(ctx context.Context, inviteID string, participantID *string) (model.Invite, error) {
	var pidVal any
	if participantID != nil {
		pidVal = *participantID
	}
	var inv model.Invite
	var matched sql.NullString
	err := s.db.QueryRowContext(ctx,
		`UPDATE calendar_invites SET matched_participant_id = $1 WHERE id = $2
		 RETURNING id, calendar_id, name, matched_participant_id, created_at`,
		pidVal, inviteID,
	).Scan(&inv.ID, &inv.CalendarID, &inv.Name, &matched, &inv.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Invite{}, ErrInviteNotFound
		}
		return model.Invite{}, fmt.Errorf("set invite match: %w", err)
	}
	if matched.Valid {
		v := matched.String
		inv.MatchedParticipantID = &v
	}
	return inv, nil
}

// MatchInviteByName looks for an unmatched invite on the given calendar whose
// normalised name (lower(trim(name))) equals the participant's normalised
// name. If exactly one or more match, links the oldest unmatched invite to the
// participant. If no invite matches, it's a no-op. Best-effort — callers may
// log failures but should not fail the parent operation.
func (s *Store) MatchInviteByName(ctx context.Context, calendarID, participantID, name string) error {
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE calendar_invites
		 SET matched_participant_id = $1
		 WHERE id = (
		   SELECT id FROM calendar_invites
		   WHERE calendar_id = $2
		     AND matched_participant_id IS NULL
		     AND LOWER(TRIM(name)) = $3
		   ORDER BY created_at ASC, id ASC
		   LIMIT 1
		 )`,
		participantID, calendarID, key,
	)
	if err != nil {
		return fmt.Errorf("match invite by name: %w", err)
	}
	return nil
}

// ParticipantsWithNoAvailability returns the subset of participants on a
// calendar that have zero availability rows — the "joined but not responded"
// bucket. Cheap one-query read using a LEFT JOIN.
func (s *Store) ParticipantsWithNoAvailability(ctx context.Context, calendarID string) ([]model.Participant, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT p.id, p.calendar_id, p.name, p.colour, p.user_id
		 FROM participants p
		 LEFT JOIN availability a ON a.participant_id = p.id
		 WHERE p.calendar_id = $1 AND p.deleted_at IS NULL
		 GROUP BY p.id
		 HAVING COUNT(a.participant_id) = 0
		 ORDER BY p.name ASC`, calendarID,
	)
	if err != nil {
		return nil, fmt.Errorf("participants without availability: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := []model.Participant{}
	for rows.Next() {
		var p model.Participant
		var uid sql.NullString
		if err := rows.Scan(&p.ID, &p.CalendarID, &p.Name, &p.Colour, &uid); err != nil {
			return nil, fmt.Errorf("scan participant: %w", err)
		}
		if uid.Valid {
			p.UserID = &uid.String
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
