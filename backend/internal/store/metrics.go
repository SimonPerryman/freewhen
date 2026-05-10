package store

import (
	"context"
	"fmt"
	"time"
)

func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return n, nil
}

func (s *Store) CountUsersSince(ctx context.Context, t time.Time) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE created_at >= $1`, t).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count users since: %w", err)
	}
	return n, nil
}

func (s *Store) CountCalendars(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM calendars WHERE deleted_at IS NULL`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count calendars: %w", err)
	}
	return n, nil
}

func (s *Store) CountCalendarsSince(ctx context.Context, t time.Time) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM calendars WHERE created_at >= $1 AND deleted_at IS NULL`, t).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count calendars since: %w", err)
	}
	return n, nil
}

func (s *Store) CountParticipants(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM participants WHERE deleted_at IS NULL`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count participants: %w", err)
	}
	return n, nil
}

func (s *Store) AvgParticipantsPerCalendar(ctx context.Context) (float64, error) {
	var avg *float64
	err := s.db.QueryRowContext(ctx,
		`SELECT AVG(cnt) FROM (SELECT COUNT(*) AS cnt FROM participants WHERE deleted_at IS NULL GROUP BY calendar_id) sub`,
	).Scan(&avg)
	if err != nil {
		return 0, fmt.Errorf("avg participants per calendar: %w", err)
	}
	if avg == nil {
		return 0, nil
	}
	return *avg, nil
}

// ActiveCalendars counts calendars created in the last N days that have at least one availability entry.
func (s *Store) ActiveCalendars(ctx context.Context, days int) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(DISTINCT c.id)
		 FROM calendars c
		 JOIN participants p ON p.calendar_id = c.id
		 JOIN availability a ON a.participant_id = p.id
		 WHERE c.created_at >= NOW() - MAKE_INTERVAL(days => $1)
		   AND c.deleted_at IS NULL AND p.deleted_at IS NULL`,
		days,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("active calendars: %w", err)
	}
	return n, nil
}
