package handler

import (
	"context"
	"net/http"

	"github.com/freewhen/backend/internal/calprovider"
	"github.com/freewhen/backend/internal/model"
	"github.com/freewhen/backend/internal/service"
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
	AddParticipant(ctx context.Context, calendarID, name string, userID *string, email *string) (model.Participant, string, error)
	UpdateParticipantEmail(ctx context.Context, calendarID, targetParticipantID string, caller service.ParticipantCaller, email *string) (model.Participant, error)
	SetAvailability(ctx context.Context, calendarID, participantID string, caller service.ParticipantCaller, update service.AvailabilityUpdate) error
	ImportAvailability(ctx context.Context, calendarID, participantID string, caller service.ParticipantCaller, dates []string) error
	FinaliseCalendar(ctx context.Context, calendarID, userID, date string) (model.Calendar, error)
	UnfinaliseCalendar(ctx context.Context, calendarID, userID string) (model.Calendar, error)
	ClaimParticipant(ctx context.Context, calendarID, participantID string, caller service.ParticipantCaller) (model.Participant, error)
	RenameCalendar(ctx context.Context, calendarID, userID, name string) (model.Calendar, error)
	RemoveParticipant(ctx context.Context, calendarID, userID, participantID string) error
	RenameParticipant(ctx context.Context, calendarID, targetParticipantID string, caller service.ParticipantCaller, name string, email *string) (model.Participant, error)
	ListUserCalendars(ctx context.Context, userID string) ([]model.CalendarListItem, bool, error)
	UnlockCalendar(ctx context.Context, calendarID, submitted, remoteAddr string) (string, error)
	RotateCalendarPassphrase(ctx context.Context, calendarID, userID string) (string, error)
	ClearCalendarPassphrase(ctx context.Context, calendarID, userID string) error
	AuthorizeAccess(ctx context.Context, cal model.Calendar, viewer service.Viewer, tokenHeader string) error
	GetMissingView(ctx context.Context, calendarID, userID string) (service.MissingView, error)
	AddInvites(ctx context.Context, calendarID, userID string, rawNames []string) (service.AddInvitesResult, error)
	RenameInvite(ctx context.Context, calendarID, userID, inviteID, name string) (model.Invite, error)
	DeleteInvite(ctx context.Context, calendarID, userID, inviteID string) error
	ManualMatchInvite(ctx context.Context, calendarID, userID, inviteID string, participantID *string) (model.Invite, error)
	DeleteCalendar(ctx context.Context, calendarID, userID string) error
	LeaveCalendar(ctx context.Context, calendarID, userID string) error
	IsCalendarSoftDeleted(ctx context.Context, calendarID string) (bool, error)
}

type Handler struct {
	service           CalendarService
	trustedProxyDepth int
	// frontendURL is the canonical public origin of the FreeWhen frontend
	// (e.g. https://freewhen.app). Used to render the "View on FreeWhen" link
	// embedded in exported events. Empty in environments without a public URL
	// configured — the link is omitted in that case.
	frontendURL string
	// calProviders is the registry of configured calendar providers used by
	// the export deep-link endpoint. Nil-safe — an empty registry returns 404
	// for any provider name.
	calProviders *calprovider.Registry
}

func New(s CalendarService) *Handler {
	return &Handler{service: s, calProviders: calprovider.NewRegistry()}
}

// WithCalendarProviders attaches the calendar-provider registry used by the
// export deep-link endpoint.
func (h *Handler) WithCalendarProviders(r *calprovider.Registry) *Handler {
	if r != nil {
		h.calProviders = r
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
