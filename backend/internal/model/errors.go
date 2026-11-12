package model

import (
	"errors"
	"fmt"
)

// Domain sentinel errors. They live in model (not store) so the HTTP layer
// can branch on them without importing the persistence package; store
// returns them and keeps package-level aliases for its own callers.
var (
	ErrNotFound             = errors.New("not found")
	ErrDuplicateName        = errors.New("duplicate participant name")
	ErrCalendarFinalised    = errors.New("calendar is finalised")
	ErrAlreadyLinked        = errors.New("participant already linked to a user")
	ErrUserAlreadyJoined    = errors.New("user already has a participant in this calendar")
	ErrCalendarFull         = errors.New("calendar is full")
	ErrCalendarLimitReached = errors.New("owner has reached the calendar limit")
	ErrInviteLimitReached   = errors.New("calendar invite list is full")
	ErrInviteNameTaken      = errors.New("invite name already exists on this calendar")
	ErrInviteNotFound       = errors.New("invite not found")
	// ErrCalendarUnavailable is returned by mutating handlers when the
	// caller refers to a calendar that has been soft-deleted out from under
	// them (or to which the caller no longer has a live participant row).
	// Distinguished from ErrNotFound so the HTTP layer can map it to 410.
	ErrCalendarUnavailable = errors.New("calendar unavailable")
	// ErrCannotLeaveOwnedCalendar is returned by Leave when the caller is
	// the calendar's owner. Owners must Delete instead.
	ErrCannotLeaveOwnedCalendar = errors.New("cannot leave owned calendar")
	// ErrUserExists is returned by user creation when the email is already
	// taken (typically a concurrent sign-in that inserted first).
	ErrUserExists = errors.New("user with this email already exists")
)

// ErrCalendarNotFound is returned when the calendar itself (as opposed to a
// child row such as a participant or invite) does not exist or is
// soft-deleted. It wraps ErrNotFound, so errors.Is(err, ErrNotFound) still
// holds; handlers that must tell "calendar missing" (404/410) apart from
// "participant missing" (404) check this first.
var ErrCalendarNotFound = fmt.Errorf("calendar %w", ErrNotFound)

// CalendarListItem is a calendar plus its participant count, returned by
// the "my calendars" listing. The caller derives per-user fields (role,
// etc.) from the embedded Calendar.
type CalendarListItem struct {
	Calendar
	ParticipantCount int
}
