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
	MaxCalendarsPerUser   = 200
	// MaxNoteLength is the per-date note ceiling in Unicode code points. The DB
	// enforces the same rule via a check constraint on availability.note.
	MaxNoteLength = 140

	// CalendarLookupsPerIPPerMinute caps requests per client IP to the
	// /calendars/{id} routes reachable without a session (read, join, vote,
	// unlock, export). For a calendar without a passphrase the ID is the only
	// secret, so these routes are an enumeration oracle. Token bucket: burst
	// of this size, refilling at this rate. A real page view costs ~2; the
	// polling fallback costs 4/min. See
	// plans/tech-debt/calendar-id-entropy-and-lookup-rate-limit.md.
	CalendarLookupsPerIPPerMinute = 120
	// CalendarStreamsPerIPPerMinute caps SSE connects per client IP, in a
	// bucket separate from lookups so a burst of refetches can't starve the
	// stream. Browsers reconnect ~every 3s at worst (20/min) and the client
	// falls back to polling after repeated errors.
	CalendarStreamsPerIPPerMinute = 60
)

// DateWithinRange reports whether d falls inside the acceptable availability /
// finalisation window: one year before now to five years after now. The caller
// passes "now" so tests stay deterministic.
func DateWithinRange(d, now time.Time) bool {
	minDate := now.AddDate(-1, 0, 0)
	maxDate := now.AddDate(5, 0, 0)
	return !d.Before(minDate) && !d.After(maxDate)
}
