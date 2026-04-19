package store

import "errors"

var (
	ErrNotFound           = errors.New("not found")
	ErrDuplicateName      = errors.New("duplicate participant name")
	ErrCalendarFinalised  = errors.New("calendar is finalised")
)
