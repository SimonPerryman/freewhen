package handler

import (
	"context"
	"net/http"
	"time"
)

// MetricsStore defines the data operations the metrics handler needs.
type MetricsStore interface {
	CountUsers(ctx context.Context) (int, error)
	CountUsersSince(ctx context.Context, t time.Time) (int, error)
	CountCalendars(ctx context.Context) (int, error)
	CountCalendarsSince(ctx context.Context, t time.Time) (int, error)
	CountParticipants(ctx context.Context) (int, error)
	AvgParticipantsPerCalendar(ctx context.Context) (float64, error)
	ActiveCalendars(ctx context.Context, days int) (int, error)
}

// RealtimeMetrics reports live-stream hub counts.
type RealtimeMetrics interface {
	ActiveSubscribers() int
	ActiveCalendars() int
}

// NotificationCounters reports per-kind transactional email counts for the
// admin metrics endpoint. Implemented by *notifications.Counters.
type NotificationCounters interface {
	Snapshot() map[string]uint64
}

type MetricsHandler struct {
	store    MetricsStore
	realtime RealtimeMetrics
	notif    NotificationCounters
}

func NewMetrics(s MetricsStore, rt RealtimeMetrics) *MetricsHandler {
	return &MetricsHandler{store: s, realtime: rt}
}

// WithNotificationCounters wires the notifications counter snapshotter so
// the /admin/metrics response includes notification send/fail totals.
func (h *MetricsHandler) WithNotificationCounters(c NotificationCounters) *MetricsHandler {
	h.notif = c
	return h
}

type metricsResponse struct {
	Users         userMetrics        `json:"users"`
	Calendars     calendarMetrics    `json:"calendars"`
	Participants  participantMetrics `json:"participants"`
	Realtime      realtimeMetrics    `json:"realtime"`
	Notifications map[string]uint64  `json:"notifications,omitempty"`
}

type userMetrics struct {
	Total   int `json:"total"`
	Last7d  int `json:"last_7d"`
	Last30d int `json:"last_30d"`
}

type calendarMetrics struct {
	Total     int `json:"total"`
	Last7d    int `json:"last_7d"`
	Active30d int `json:"active_30d"`
}

type participantMetrics struct {
	Total          int     `json:"total"`
	AvgPerCalendar float64 `json:"avg_per_calendar"`
}

type realtimeMetrics struct {
	ActiveSubscribers int `json:"active_subscribers"`
	ActiveCalendars   int `json:"active_calendars"`
}

func (h *MetricsHandler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	now := time.Now()
	d7 := now.AddDate(0, 0, -7)
	d30 := now.AddDate(0, 0, -30)

	usersTotal, err := h.store.CountUsers(ctx)
	if err != nil {
		respondInternalError(r, w, err, "internal_error", "failed to fetch metrics")
		return
	}
	users7d, err := h.store.CountUsersSince(ctx, d7)
	if err != nil {
		respondInternalError(r, w, err, "internal_error", "failed to fetch metrics")
		return
	}
	users30d, err := h.store.CountUsersSince(ctx, d30)
	if err != nil {
		respondInternalError(r, w, err, "internal_error", "failed to fetch metrics")
		return
	}

	calsTotal, err := h.store.CountCalendars(ctx)
	if err != nil {
		respondInternalError(r, w, err, "internal_error", "failed to fetch metrics")
		return
	}
	cals7d, err := h.store.CountCalendarsSince(ctx, d7)
	if err != nil {
		respondInternalError(r, w, err, "internal_error", "failed to fetch metrics")
		return
	}
	active30d, err := h.store.ActiveCalendars(ctx, 30)
	if err != nil {
		respondInternalError(r, w, err, "internal_error", "failed to fetch metrics")
		return
	}

	partsTotal, err := h.store.CountParticipants(ctx)
	if err != nil {
		respondInternalError(r, w, err, "internal_error", "failed to fetch metrics")
		return
	}
	avgPerCal, err := h.store.AvgParticipantsPerCalendar(ctx)
	if err != nil {
		respondInternalError(r, w, err, "internal_error", "failed to fetch metrics")
		return
	}

	var rtSubs, rtCals int
	if h.realtime != nil {
		rtSubs = h.realtime.ActiveSubscribers()
		rtCals = h.realtime.ActiveCalendars()
	}

	respondJSON(w, http.StatusOK, metricsResponse{
		Users: userMetrics{
			Total:   usersTotal,
			Last7d:  users7d,
			Last30d: users30d,
		},
		Calendars: calendarMetrics{
			Total:     calsTotal,
			Last7d:    cals7d,
			Active30d: active30d,
		},
		Participants: participantMetrics{
			Total:          partsTotal,
			AvgPerCalendar: avgPerCal,
		},
		Realtime: realtimeMetrics{
			ActiveSubscribers: rtSubs,
			ActiveCalendars:   rtCals,
		},
		Notifications: notificationsSnapshot(h.notif),
	})
}

func notificationsSnapshot(c NotificationCounters) map[string]uint64 {
	if c == nil {
		return nil
	}
	return c.Snapshot()
}
