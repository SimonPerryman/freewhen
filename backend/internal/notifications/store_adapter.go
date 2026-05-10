package notifications

import (
	"context"
	"time"

	"github.com/freewhen/backend/internal/model"
)

// concreteStore is the subset of *store.Store the notifications package needs.
// Defined here (with the field names this package's Store interface expects)
// so we can wrap *store.Store without a circular import.
type concreteStore interface {
	GetCalendar(ctx context.Context, id string) (model.CalendarWithDetails, error)
	GetUser(ctx context.Context, id string) (model.User, error)
	ClearParticipantEmailByID(ctx context.Context, participantID string) error
	GetCalendarLastNudgeAt(ctx context.Context, calendarID string) (*time.Time, error)
	SetCalendarLastNudgeAt(ctx context.Context, calendarID string, t time.Time) error
	IsCalendarSoftDeleted(ctx context.Context, calendarID string) (bool, error)
}

// AdaptStore wraps a *store.Store so it satisfies the local Store interface.
// Kept tiny — exists only to translate two name differences.
func AdaptStore(s concreteStore) Store {
	return &storeAdapter{s: s}
}

type storeAdapter struct{ s concreteStore }

func (a *storeAdapter) GetCalendar(ctx context.Context, id string) (model.CalendarWithDetails, error) {
	return a.s.GetCalendar(ctx, id)
}

func (a *storeAdapter) GetUserByID(ctx context.Context, userID string) (model.User, error) {
	return a.s.GetUser(ctx, userID)
}

func (a *storeAdapter) ClearParticipantEmail(ctx context.Context, participantID string) error {
	return a.s.ClearParticipantEmailByID(ctx, participantID)
}

func (a *storeAdapter) GetCalendarLastNudgeAt(ctx context.Context, calendarID string) (*time.Time, error) {
	return a.s.GetCalendarLastNudgeAt(ctx, calendarID)
}

func (a *storeAdapter) SetCalendarLastNudgeAt(ctx context.Context, calendarID string, t time.Time) error {
	return a.s.SetCalendarLastNudgeAt(ctx, calendarID, t)
}

func (a *storeAdapter) IsCalendarSoftDeleted(ctx context.Context, calendarID string) (bool, error) {
	return a.s.IsCalendarSoftDeleted(ctx, calendarID)
}
