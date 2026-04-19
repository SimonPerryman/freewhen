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

type MetricsHandler struct {
	store MetricsStore
}

func NewMetrics(s MetricsStore) *MetricsHandler {
	return &MetricsHandler{store: s}
}

type metricsResponse struct {
	Users        userMetrics        `json:"users"`
	Calendars    calendarMetrics    `json:"calendars"`
	Participants participantMetrics `json:"participants"`
}

type userMetrics struct {
	Total  int `json:"total"`
	Last7d int `json:"last_7d"`
	Last30d int `json:"last_30d"`
}

type calendarMetrics struct {
	Total    int `json:"total"`
	Last7d   int `json:"last_7d"`
	Active30d int `json:"active_30d"`
}

type participantMetrics struct {
	Total          int     `json:"total"`
	AvgPerCalendar float64 `json:"avg_per_calendar"`
}

func (h *MetricsHandler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	now := time.Now()
	d7 := now.AddDate(0, 0, -7)
	d30 := now.AddDate(0, 0, -30)

	usersTotal, err := h.store.CountUsers(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to fetch metrics")
		return
	}
	users7d, err := h.store.CountUsersSince(ctx, d7)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to fetch metrics")
		return
	}
	users30d, err := h.store.CountUsersSince(ctx, d30)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to fetch metrics")
		return
	}

	calsTotal, err := h.store.CountCalendars(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to fetch metrics")
		return
	}
	cals7d, err := h.store.CountCalendarsSince(ctx, d7)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to fetch metrics")
		return
	}
	active30d, err := h.store.ActiveCalendars(ctx, 30)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to fetch metrics")
		return
	}

	partsTotal, err := h.store.CountParticipants(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to fetch metrics")
		return
	}
	avgPerCal, err := h.store.AvgParticipantsPerCalendar(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to fetch metrics")
		return
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
	})
}
