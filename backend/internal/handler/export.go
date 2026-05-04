package handler

import (
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/freewhen/backend/internal/ics"
	"github.com/freewhen/backend/internal/store"
)

// ExportICS serves a .ics file for a finalised calendar. Public — the export
// is intentionally available to anyone (signed-in or not) once finalised, so
// anonymous joiners can also add the event to their own calendar.
func (h *Handler) ExportICS(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	cal, err := h.service.GetCalendarOnly(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "not_found", "calendar not found")
			return
		}
		respondInternalError(r, w, err, "internal_error", "failed to load calendar")
		return
	}
	if cal.FinalisedDate == nil {
		respondError(w, http.StatusNotFound, "not_finalised", "calendar is not finalised")
		return
	}

	date, err := time.Parse("2006-01-02", *cal.FinalisedDate)
	if err != nil {
		respondInternalError(r, w, err, "internal_error", "invalid finalised date")
		return
	}

	payload := ics.Render(ics.Event{
		UID:     cal.ID + "@freewhen",
		Summary: calendarSummary(cal.Emoji, cal.Name),
		Date:    date,
	})

	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="freewhen.ics"`)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(payload))
}

// ExportGoogle redirects to Google Calendar's render URL with the event
// pre-filled. No OAuth, no write scope — Google's own form opens in the
// user's browser and they tap "Save". Public.
func (h *Handler) ExportGoogle(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	cal, err := h.service.GetCalendarOnly(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "not_found", "calendar not found")
			return
		}
		respondInternalError(r, w, err, "internal_error", "failed to load calendar")
		return
	}
	if cal.FinalisedDate == nil {
		respondError(w, http.StatusNotFound, "not_finalised", "calendar is not finalised")
		return
	}

	date, err := time.Parse("2006-01-02", *cal.FinalisedDate)
	if err != nil {
		respondInternalError(r, w, err, "internal_error", "invalid finalised date")
		return
	}

	// Google's render URL expects dates in the form YYYYMMDD/YYYYMMDD for
	// all-day events (end exclusive — matches the iCal convention).
	start := date.Format("20060102")
	end := date.AddDate(0, 0, 1).Format("20060102")

	q := url.Values{}
	q.Set("action", "TEMPLATE")
	q.Set("text", calendarSummary(cal.Emoji, cal.Name))
	q.Set("dates", start+"/"+end)

	http.Redirect(w, r, "https://calendar.google.com/calendar/render?"+q.Encode(), http.StatusFound)
}

// calendarSummary formats the calendar title for export — includes the emoji
// prefix if the calendar has one.
func calendarSummary(emoji, name string) string {
	if emoji != "" {
		return emoji + " " + name
	}
	return name
}
