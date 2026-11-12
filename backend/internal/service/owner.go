package service

import (
	"context"
	"errors"

	"github.com/freewhen/backend/internal/model"
)

// requireOwner is the single owner-only authorization decision. It loads
// the calendar and returns it when userID is the owner.
//
// Errors:
//   - model.ErrCalendarNotFound (wraps model.ErrNotFound) when the calendar
//     does not exist or is soft-deleted. Distinct from a child-row
//     ErrNotFound so handlers can map it to 404/410 via the soft-delete
//     lookup while keeping "participant not found" a plain 404.
//   - ErrForbidden when the caller is not the owner (including an empty
//     userID, which never matches a real owner).
func (s *Service) requireOwner(ctx context.Context, calendarID, userID string) (model.Calendar, error) {
	cal, err := s.store.GetCalendarOnly(ctx, calendarID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return model.Calendar{}, model.ErrCalendarNotFound
		}
		return model.Calendar{}, err
	}
	if userID == "" || cal.OwnerID != userID {
		return model.Calendar{}, ErrForbidden
	}
	return cal, nil
}

// AuthorizeCalendarOwner reports whether userID owns calendarID. For callers
// whose owner-only action lives outside this service (the nudge endpoint,
// which is a notifications.Service operation). Same errors as requireOwner.
func (s *Service) AuthorizeCalendarOwner(ctx context.Context, calendarID, userID string) error {
	_, err := s.requireOwner(ctx, calendarID, userID)
	return err
}
