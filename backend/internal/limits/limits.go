// Package limits centralises the hard caps applied to public API inputs so
// the database, the owner calendar list, and the realtime fan-out can't be
// cheaply blown up. See plans/input-limits.md for the reasoning behind each
// value.
package limits

import "time"

const (
	MaxCalendarNameLength      = 100
	MaxParticipantNameLength   = 50
	MaxUserNameLength          = 50
	MaxDatesPerSubmission      = 366
	MaxParticipantsPerCalendar = 100
	// MaxInvitesPerCalendar caps the owner's "Who's missing?" invite list. Kept
	// in lockstep with MaxParticipantsPerCalendar so invites and participants
	// move together if either is tuned.
	MaxInvitesPerCalendar = MaxParticipantsPerCalendar
	MaxCalendarsPerUser        = 200
	// MaxNoteLength is the per-date note ceiling in Unicode code points. The DB
	// enforces the same rule via a check constraint on availability.note.
	MaxNoteLength = 140
)

// DateWithinRange reports whether d falls inside the acceptable availability /
// finalisation window: one year before now to five years after now. The caller
// passes "now" so tests stay deterministic.
func DateWithinRange(d, now time.Time) bool {
	minDate := now.AddDate(-1, 0, 0)
	maxDate := now.AddDate(5, 0, 0)
	return !d.Before(minDate) && !d.After(maxDate)
}
