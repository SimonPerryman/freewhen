package store

import (
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

func isDuplicateName(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		// Matches the partial unique index added in migration 026 plus the
		// legacy table-level UNIQUE constraint name (kept for the brief
		// upgrade window where both could appear in error output).
		if pgErr.Code != "23505" {
			return false
		}
		return strings.Contains(pgErr.ConstraintName, "participants_calendar_name_active_idx") ||
			strings.Contains(pgErr.ConstraintName, "participants_calendar_id_name")
	}
	return false
}
