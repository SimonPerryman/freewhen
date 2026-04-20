package store

import "errors"

var (
	ErrNotFound             = errors.New("not found")
	ErrDuplicateName        = errors.New("duplicate participant name")
	ErrCalendarFinalised    = errors.New("calendar is finalised")
	ErrAlreadyLinked        = errors.New("participant already linked to a user")
	ErrUserAlreadyJoined    = errors.New("user already has a participant in this calendar")
)
