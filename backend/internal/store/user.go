package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/freewhen/backend/internal/model"
)

// userCols is the fully-qualified SELECT list for user rows. Kept in one
// place so every query stays aligned with the scan helper.
const userCols = "id, email, name, is_admin, provider, provider_id, created_at, calendar_import_consent_ack_at"

// scanUser reads a row in userCols order into u.
func scanUser(row interface {
	Scan(dest ...any) error
}, u *model.User) error {
	var ack sql.NullTime
	if err := row.Scan(&u.ID, &u.Email, &u.Name, &u.IsAdmin, &u.Provider, &u.ProviderID, &u.CreatedAt, &ack); err != nil {
		return err
	}
	if ack.Valid {
		t := ack.Time
		u.CalendarImportConsentAckAt = &t
	}
	return nil
}

func (s *Store) CreateUser(ctx context.Context, email, name, provider, providerID string) (model.User, error) {
	const maxRetries = 3
	for i := 0; i < maxRetries; i++ {
		id := generateID(8)
		var u model.User
		row := s.db.QueryRowContext(ctx,
			`INSERT INTO users (id, email, name, provider, provider_id)
			 VALUES ($1, $2, $3, $4, $5)
			 RETURNING `+userCols,
			id, email, name, provider, providerID,
		)
		if err := scanUser(row, &u); err != nil {
			if isUniqueViolation(err) {
				continue
			}
			return model.User{}, fmt.Errorf("insert user: %w", err)
		}
		return u, nil
	}
	return model.User{}, fmt.Errorf("failed to generate unique user ID after %d attempts", maxRetries)
}

func (s *Store) FindUserByEmail(ctx context.Context, email string) (model.User, error) {
	var u model.User
	row := s.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE email = $1`, email)
	if err := scanUser(row, &u); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.User{}, ErrNotFound
		}
		return model.User{}, fmt.Errorf("find user by email: %w", err)
	}
	return u, nil
}

func (s *Store) GetUser(ctx context.Context, id string) (model.User, error) {
	var u model.User
	row := s.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE id = $1`, id)
	if err := scanUser(row, &u); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.User{}, ErrNotFound
		}
		return model.User{}, fmt.Errorf("get user: %w", err)
	}
	return u, nil
}

func (s *Store) UpdateUserName(ctx context.Context, userID, name string) (model.User, error) {
	var u model.User
	row := s.db.QueryRowContext(ctx,
		`UPDATE users SET name = $1 WHERE id = $2 RETURNING `+userCols,
		name, userID,
	)
	if err := scanUser(row, &u); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.User{}, ErrNotFound
		}
		return model.User{}, fmt.Errorf("update user name: %w", err)
	}
	return u, nil
}

func (s *Store) FindUserByProviderID(ctx context.Context, provider, providerID string) (model.User, error) {
	var u model.User
	row := s.db.QueryRowContext(ctx,
		`SELECT `+userCols+` FROM users WHERE provider = $1 AND provider_id = $2`,
		provider, providerID,
	)
	if err := scanUser(row, &u); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.User{}, ErrNotFound
		}
		return model.User{}, fmt.Errorf("find user by provider ID: %w", err)
	}
	return u, nil
}

// SetCalendarImportConsentAck records that the user has acknowledged the
// privacy interstitial. Idempotent — always writes the current time,
// overwriting any earlier ack.
func (s *Store) SetCalendarImportConsentAck(ctx context.Context, userID string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET calendar_import_consent_ack_at = $1 WHERE id = $2`,
		time.Now().UTC(), userID,
	)
	if err != nil {
		return fmt.Errorf("set consent ack: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// MergeAvailabilityDates adds the given dates to a participant's availability
// without removing existing rows. Used by the "Accept all suggestions"
// shortcut — imports must never silently delete hand-entered rows.
func (s *Store) MergeAvailabilityDates(ctx context.Context, participantID string, dates []string) error {
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

	for _, date := range dates {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO availability (participant_id, calendar_id, date)
			 VALUES ($1, $2, $3)
			 ON CONFLICT (participant_id, date) DO NOTHING`,
			participantID, calendarID, date,
		); err != nil {
			return fmt.Errorf("insert availability for %s: %w", date, err)
		}
	}
	return tx.Commit()
}
