package store

import "github.com/freewhen/backend/internal/model"

// Sentinel errors are defined in model so handlers don't import store. These
// aliases are the same values, so errors.Is matches across both names.
var (
	ErrNotFound                 = model.ErrNotFound
	ErrDuplicateName            = model.ErrDuplicateName
	ErrCalendarFinalised        = model.ErrCalendarFinalised
	ErrAlreadyLinked            = model.ErrAlreadyLinked
	ErrUserAlreadyJoined        = model.ErrUserAlreadyJoined
	ErrCalendarFull             = model.ErrCalendarFull
	ErrCalendarLimitReached     = model.ErrCalendarLimitReached
	ErrInviteLimitReached       = model.ErrInviteLimitReached
	ErrInviteNameTaken          = model.ErrInviteNameTaken
	ErrInviteNotFound           = model.ErrInviteNotFound
	ErrCalendarUnavailable      = model.ErrCalendarUnavailable
	ErrCannotLeaveOwnedCalendar = model.ErrCannotLeaveOwnedCalendar
)
