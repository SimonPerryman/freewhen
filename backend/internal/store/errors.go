package store

import "errors"

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
)
