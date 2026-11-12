package service

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/freewhen/backend/internal/limits"
)

// ValidationError is a rejected input. Code and Message are the stable API
// error code and human-readable message the HTTP layer returns with a 400;
// Err, when set, is a sentinel callers can match with errors.Is.
type ValidationError struct {
	Code    string
	Message string
	Err     error
}

func (e *ValidationError) Error() string { return e.Code + ": " + e.Message }

func (e *ValidationError) Unwrap() error { return e.Err }

func invalid(code, message string) *ValidationError {
	return &ValidationError{Code: code, Message: message}
}

// AvailabilityUpdate is a full replacement of a participant's availability.
//
// Notes and Preferred are tri-state: nil means "leave existing values for
// retained dates untouched"; non-nil (even empty) means "this is the full
// set". Every note key and preferred date must also appear in Dates.
type AvailabilityUpdate struct {
	Dates     []string
	Notes     *map[string]string
	Preferred *[]string
}

// validateDates enforces the per-submission date rules shared by
// SetAvailability and ImportAvailability: count cap, YYYY-MM-DD format, and
// the allowed range around now. Returns the dates as a set.
func validateDates(dates []string, now time.Time) (map[string]struct{}, error) {
	if len(dates) > limits.MaxDatesPerSubmission {
		return nil, invalid("too_many_dates", "too many dates in a single submission")
	}
	set := make(map[string]struct{}, len(dates))
	for _, d := range dates {
		parsed, err := time.Parse("2006-01-02", d)
		if err != nil {
			return nil, invalid("invalid_date", "invalid date format: "+d)
		}
		if !limits.DateWithinRange(parsed, now) {
			return nil, invalid("date_out_of_range", "date out of range: "+d)
		}
		set[d] = struct{}{}
	}
	return set, nil
}

// buildAvailabilityOverrides validates an AvailabilityUpdate and converts it
// into the note/preferred override maps the store expects. A nil map means
// "no override" for that column.
func buildAvailabilityOverrides(participantID string, u AvailabilityUpdate, now time.Time) (map[string]string, map[string]bool, error) {
	if participantID == "" {
		return nil, nil, invalid("invalid_request", "participant_id is required")
	}
	dateSet, err := validateDates(u.Dates, now)
	if err != nil {
		return nil, nil, err
	}

	var notes map[string]string
	if u.Notes != nil {
		notes = make(map[string]string, len(*u.Notes))
		for date, raw := range *u.Notes {
			if _, ok := dateSet[date]; !ok {
				return nil, nil, invalid("note_unknown_date", "note key not in dates: "+date)
			}
			if strings.ContainsAny(raw, "\r\n") {
				return nil, nil, invalid("note_newline", "notes must not contain line breaks")
			}
			if raw == "" {
				notes[date] = ""
				continue
			}
			trimmed := strings.TrimSpace(raw)
			if trimmed == "" {
				return nil, nil, invalid("note_whitespace", "notes must not be whitespace-only")
			}
			if utf8.RuneCountInString(trimmed) > limits.MaxNoteLength {
				return nil, nil, invalid("note_too_long", "note exceeds maximum length")
			}
			notes[date] = trimmed
		}
	}

	var preferred map[string]bool
	if u.Preferred != nil {
		preferred = make(map[string]bool, len(u.Dates))
		for _, d := range u.Dates {
			preferred[d] = false
		}
		for _, d := range *u.Preferred {
			if _, ok := dateSet[d]; !ok {
				return nil, nil, invalid("preferred_unknown_date", "preferred date not in dates: "+d)
			}
			preferred[d] = true
		}
	}
	return notes, preferred, nil
}

// validateParticipantName trims and checks a participant display name.
// Both errors wrap ErrInvalidName.
func validateParticipantName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", &ValidationError{Code: "invalid_request", Message: "name is required", Err: ErrInvalidName}
	}
	if len(name) > limits.MaxParticipantNameLength {
		return "", &ValidationError{Code: "name_too_long", Message: "participant name is too long", Err: ErrInvalidName}
	}
	return name, nil
}
