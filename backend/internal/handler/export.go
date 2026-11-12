package handler

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/freewhen/backend/internal/calprovider"
	"github.com/freewhen/backend/internal/ics"
	"github.com/freewhen/backend/internal/model"
	"github.com/freewhen/backend/internal/service"
	"github.com/freewhen/backend/internal/store"
)

// ExportICS serves a .ics file for a finalised calendar. Available to anyone
// (signed-in or not) who can read the calendar, so anonymous joiners can also
// add the event to their own calendar. Passphrase-protected calendars apply
// the same gate as GET /calendars/{id}: owner session, or a valid access
// token via the X-Calendar-Access header or `?access=` query param (plain
// link navigations can't set headers).
func (h *Handler) ExportICS(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	cal, ok := h.loadExportableCalendar(w, r, id)
	if !ok {
		return
	}
	if cal.Calendar.FinalisedDate == nil {
		respondError(w, http.StatusNotFound, "not_finalised", "calendar is not finalised")
		return
	}

	date, err := time.Parse("2006-01-02", *cal.Calendar.FinalisedDate)
	if err != nil {
		respondInternalError(r, w, err, "internal_error", "invalid finalised date")
		return
	}

	payload := ics.Render(ics.Event{
		UID:         cal.Calendar.ID + "@freewhen",
		Summary:     calendarSummary(cal.Calendar.Emoji, cal.Calendar.Name),
		Description: calendarEventDescription(cal, h.frontendURL),
		Date:        date,
	})

	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="freewhen.ics"`)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(payload))
}

// ExportToProvider redirects to the given calendar provider's "add event"
// deep link with the event pre-filled. No OAuth, no write scope, no API call.
// Gated like ExportICS. Provider name comes from the `{provider}` path param.
func (h *Handler) ExportToProvider(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	provider, ok := h.calProviders.Get(r.PathValue("provider"))
	if !ok {
		respondError(w, http.StatusNotFound, "not_found", "unknown calendar provider")
		return
	}

	cal, ok := h.loadExportableCalendar(w, r, id)
	if !ok {
		return
	}
	if cal.Calendar.FinalisedDate == nil {
		respondError(w, http.StatusNotFound, "not_finalised", "calendar is not finalised")
		return
	}

	date, err := time.Parse("2006-01-02", *cal.Calendar.FinalisedDate)
	if err != nil {
		respondInternalError(r, w, err, "internal_error", "invalid finalised date")
		return
	}

	target := provider.AddEventURL(calprovider.ExportEvent{
		Title:       calendarSummary(cal.Calendar.Emoji, cal.Calendar.Name),
		Description: calendarEventDescription(cal, h.frontendURL),
		Date:        date,
		AllDay:      true,
	})
	http.Redirect(w, r, target, http.StatusFound)
}

// loadExportableCalendar loads the calendar with details and enforces the
// passphrase gate. Writes the error response and returns ok=false on failure.
func (h *Handler) loadExportableCalendar(w http.ResponseWriter, r *http.Request, id string) (model.CalendarWithDetails, bool) {
	cal, err := h.service.GetCalendar(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "not_found", "calendar not found")
			return model.CalendarWithDetails{}, false
		}
		respondInternalError(r, w, err, "internal_error", "failed to load calendar")
		return model.CalendarWithDetails{}, false
	}

	viewer := service.Viewer{}
	if u := UserFromContext(r); u != nil {
		uid := u.ID
		viewer.UserID = &uid
	}
	token := r.Header.Get(AccessHeader)
	if token == "" {
		token = r.URL.Query().Get("access")
	}
	if err := h.service.AuthorizeAccess(r.Context(), cal.Calendar, viewer, token); err != nil {
		if errors.Is(err, service.ErrPassphraseRequired) {
			respondError(w, http.StatusUnauthorized, "passphrase_required", "passphrase required")
			return model.CalendarWithDetails{}, false
		}
		respondInternalError(r, w, err, "internal_error", "failed to authorize access")
		return model.CalendarWithDetails{}, false
	}
	return cal, true
}

// calendarSummary formats the calendar title for export — includes the emoji
// prefix if the calendar has one.
func calendarSummary(emoji, name string) string {
	if emoji != "" {
		return emoji + " " + name
	}
	return name
}

// calendarEventDescription renders the long-form event body included in
// exported events: who's coming, any per-day notes, and a link back to the
// calendar in FreeWhen so the event keeps a trail to the source.
//
// Filters availability to the finalised date and joins to participants by ID.
// Participants are sorted alphabetically (stable; no implicit owner-first
// surprise). The Notes section is omitted entirely when no one left a note.
func calendarEventDescription(cal model.CalendarWithDetails, frontendBase string) string {
	if cal.Calendar.FinalisedDate == nil {
		return ""
	}
	finalDate := *cal.Calendar.FinalisedDate

	names := make(map[string]string, len(cal.Participants))
	for _, p := range cal.Participants {
		names[p.ID] = p.Name
	}

	type attendee struct {
		name string
		note string
	}
	var attendees []attendee
	for _, av := range cal.Availability {
		if av.Date != finalDate {
			continue
		}
		name, ok := names[av.ParticipantID]
		if !ok {
			continue
		}
		a := attendee{name: name}
		if av.Note != nil {
			a.note = *av.Note
		}
		attendees = append(attendees, a)
	}
	sort.SliceStable(attendees, func(i, j int) bool {
		return strings.ToLower(attendees[i].name) < strings.ToLower(attendees[j].name)
	})

	var b strings.Builder
	if len(attendees) > 0 {
		going := make([]string, len(attendees))
		for i, a := range attendees {
			going[i] = a.name
		}
		fmt.Fprintf(&b, "Going: %s\n", strings.Join(going, ", "))

		var notes []attendee
		for _, a := range attendees {
			if a.note != "" {
				notes = append(notes, a)
			}
		}
		if len(notes) > 0 {
			b.WriteString("\nNotes:\n")
			for _, a := range notes {
				fmt.Fprintf(&b, "• %s — %s\n", a.name, a.note)
			}
		}
		b.WriteString("\n")
	}

	if frontendBase != "" {
		base := strings.TrimRight(frontendBase, "/")
		fmt.Fprintf(&b, "View on FreeWhen: %s/calendars/%s\n", base, cal.Calendar.ID)
		fmt.Fprintf(&b, "Created with FreeWhen — %s\n", base)
	}

	return strings.TrimRight(b.String(), "\n")
}
