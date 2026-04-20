// Package limits centralises the hard caps applied to public API inputs so
// the database, the owner calendar list, and the realtime fan-out can't be
// cheaply blown up. See plans/input-limits.md for the reasoning behind each
// value.
package limits

import "time"

const (
	MaxCalendarNameLength      = 100
	MaxParticipantNameLength   = 50
	MaxDatesPerSubmission      = 366
	MaxParticipantsPerCalendar = 100
	MaxCalendarsPerUser        = 200
)

// DateWithinRange reports whether d falls inside the acceptable availability /
// finalisation window: one year before now to five years after now. The caller
// passes "now" so tests stay deterministic.
func DateWithinRange(d, now time.Time) bool {
	minDate := now.AddDate(-1, 0, 0)
	maxDate := now.AddDate(5, 0, 0)
	return !d.Before(minDate) && !d.After(maxDate)
}
