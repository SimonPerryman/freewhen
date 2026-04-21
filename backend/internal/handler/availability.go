package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/freewhen/backend/internal/limits"
	"github.com/freewhen/backend/internal/store"
)

func (h *Handler) SetAvailability(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	calendarID := r.PathValue("id")

	if ok, _ := h.requireCalendarAccess(w, r, calendarID); !ok {
		return
	}

	var req struct {
		ParticipantID string             `json:"participant_id"`
		Dates         []string           `json:"dates"`
		Notes         *map[string]string `json:"notes"`
		Preferred     *[]string          `json:"preferred"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_json", "invalid JSON")
		return
	}

	if req.ParticipantID == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "participant_id is required")
		return
	}

	if len(req.Dates) > limits.MaxDatesPerSubmission {
		respondError(w, http.StatusBadRequest, "too_many_dates", "too many dates in a single submission")
		return
	}

	dateSet := make(map[string]struct{}, len(req.Dates))
	now := time.Now().UTC()
	for _, d := range req.Dates {
		parsed, err := time.Parse("2006-01-02", d)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid_date", "invalid date format: "+d)
			return
		}
		if !limits.DateWithinRange(parsed, now) {
			respondError(w, http.StatusBadRequest, "date_out_of_range", "date out of range: "+d)
			return
		}
		dateSet[d] = struct{}{}
	}

	var noteOverrides map[string]string
	if req.Notes != nil {
		noteOverrides = make(map[string]string, len(*req.Notes))
		for date, raw := range *req.Notes {
			if _, ok := dateSet[date]; !ok {
				respondError(w, http.StatusBadRequest, "note_unknown_date", "note key not in dates: "+date)
				return
			}
			if strings.ContainsAny(raw, "\r\n") {
				respondError(w, http.StatusBadRequest, "note_newline", "notes must not contain line breaks")
				return
			}
			if raw == "" {
				noteOverrides[date] = ""
				continue
			}
			trimmed := strings.TrimSpace(raw)
			if trimmed == "" {
				respondError(w, http.StatusBadRequest, "note_whitespace", "notes must not be whitespace-only")
				return
			}
			if utf8.RuneCountInString(trimmed) > limits.MaxNoteLength {
				respondError(w, http.StatusBadRequest, "note_too_long", "note exceeds maximum length")
				return
			}
			noteOverrides[date] = trimmed
		}
	}

	var preferredOverrides map[string]bool
	if req.Preferred != nil {
		preferredOverrides = make(map[string]bool, len(req.Dates))
		for _, d := range req.Dates {
			preferredOverrides[d] = false
		}
		for _, d := range *req.Preferred {
			if _, ok := dateSet[d]; !ok {
				respondError(w, http.StatusBadRequest, "preferred_unknown_date", "preferred date not in dates: "+d)
				return
			}
			preferredOverrides[d] = true
		}
	}

	err := h.service.SetAvailability(r.Context(), calendarID, req.ParticipantID, req.Dates, noteOverrides, preferredOverrides)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "not_found", "participant not found")
			return
		}
		if errors.Is(err, store.ErrCalendarFinalised) {
			respondError(w, http.StatusConflict, "already_finalised", "calendar is finalised")
			return
		}
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to set availability")
		return
	}

	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
