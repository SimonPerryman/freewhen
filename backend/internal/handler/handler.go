package handler

import (
	"context"
	"net/http"

	"github.com/freewhen/backend/internal/flags"
	"github.com/freewhen/backend/internal/model"
	"github.com/freewhen/backend/internal/service"
	"github.com/freewhen/backend/internal/store"
)

type contextKey struct{ name string }

// UserContextKey is used by the auth middleware to attach the user to the request context.
var UserContextKey = contextKey{name: "user"}

// RequestIDContextKey is used by the request-log middleware to attach the per-request id.
var RequestIDContextKey = contextKey{name: "request_id"}

// RequestIDFromContext returns the per-request id set by middleware, or "" if absent.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(RequestIDContextKey).(string)
	return id
}

// CalendarService defines the operations the handler needs from the service layer.
type CalendarService interface {
	CreateCalendar(ctx context.Context, name, ownerID string) (model.Calendar, error)
	DuplicateCalendar(ctx context.Context, sourceID, newOwnerID, ownerParticipantName, overrideName string, includeParticipants bool) (model.Calendar, model.Participant, error)
	GetCalendar(ctx context.Context, id string) (model.CalendarWithDetails, error)
	GetCalendarOnly(ctx context.Context, id string) (model.Calendar, error)
	AddParticipant(ctx context.Context, calendarID, name string, userID *string, email *string) (model.Participant, error)
	UpdateParticipantEmail(ctx context.Context, calendarID, targetParticipantID string, callerUserID *string, callerParticipantID string, email *string) (model.Participant, error)
	SetAvailability(ctx context.Context, calendarID, participantID string, dates []string, noteOverrides map[string]string, preferredOverrides map[string]bool) error
	ImportAvailability(ctx context.Context, calendarID, participantID string, dates []string) error
	FinaliseCalendar(ctx context.Context, calendarID, date string) (model.Calendar, error)
	UnfinaliseCalendar(ctx context.Context, calendarID string) (model.Calendar, error)
	ClaimParticipant(ctx context.Context, calendarID, participantID, userID string) (model.Participant, error)
	RenameCalendar(ctx context.Context, calendarID, name string) (model.Calendar, error)
	RemoveParticipant(ctx context.Context, calendarID, participantID string) error
	RenameParticipant(ctx context.Context, calendarID, targetParticipantID string, callerUserID *string, callerParticipantID, name string) (model.Participant, error)
	ListUserCalendars(ctx context.Context, userID string) ([]store.CalendarListItem, bool, error)
	UnlockCalendar(ctx context.Context, calendarID, submitted, remoteAddr string) (string, error)
	RotateCalendarPassphrase(ctx context.Context, calendarID string) (string, error)
	ClearCalendarPassphrase(ctx context.Context, calendarID string) error
	AuthorizeAccess(ctx context.Context, cal model.Calendar, viewer service.Viewer, tokenHeader string) error
	GetMissingView(ctx context.Context, calendarID string) (service.MissingView, error)
	AddInvites(ctx context.Context, calendarID string, rawNames []string) (service.AddInvitesResult, error)
	RenameInvite(ctx context.Context, inviteID, name string) (model.Invite, error)
	DeleteInvite(ctx context.Context, inviteID string) error
	ManualMatchInvite(ctx context.Context, calendarID, inviteID string, participantID *string) (model.Invite, error)
	GetInvite(ctx context.Context, inviteID string) (model.Invite, error)
	DeleteCalendar(ctx context.Context, calendarID, userID string) error
	LeaveCalendar(ctx context.Context, calendarID, userID string) error
	IsCalendarSoftDeleted(ctx context.Context, calendarID string) (bool, error)
}

type Handler struct {
	service           CalendarService
	flags             flags.Evaluator
	trustedProxyDepth int
	// frontendURL is the canonical public origin of the FreeWhen frontend
	// (e.g. https://freewhen.app). Used to render the "View on FreeWhen" link
	// embedded in exported events. Empty in environments without a public URL
	// configured — the link is omitted in that case.
	frontendURL string
}

func New(s CalendarService) *Handler {
	return &Handler{service: s, flags: flags.Noop{}}
}

// WithFlags swaps the flag evaluator. Call once during server setup; the
// default is a no-op evaluator so tests don't need to wire PostHog.
func (h *Handler) WithFlags(fe flags.Evaluator) *Handler {
	if fe != nil {
		h.flags = fe
	}
	return h
}

// WithFrontendURL sets the public frontend origin used when rendering links
// back to FreeWhen in exported calendar events. Pass "" to omit the link.
func (h *Handler) WithFrontendURL(url string) *Handler {
	h.frontendURL = url
	return h
}

// WithTrustedProxyDepth sets the number of trailing X-Forwarded-For entries
// to treat as infrastructure hops when extracting the client IP for
// rate-limiting. Default 0 (Cloud Run direct).
func (h *Handler) WithTrustedProxyDepth(n int) *Handler {
	if n >= 0 {
		h.trustedProxyDepth = n
	}
	return h
}

// UserFromContext returns the authenticated user from the request context, or nil.
func UserFromContext(r *http.Request) *model.User {
	u, _ := r.Context().Value(UserContextKey).(*model.User)
	return u
}
