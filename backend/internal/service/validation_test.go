package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/freewhen/backend/internal/limits"
	"github.com/freewhen/backend/internal/model"
)

// testDate is always inside limits.DateWithinRange so date-agnostic tests
// don't rot as the calendar moves on.
var testDate = time.Now().UTC().Format("2006-01-02")

// testDate2 is the day after testDate.
var testDate2 = time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02")

func validationCode(t *testing.T, err error) string {
	t.Helper()
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want *ValidationError", err)
	}
	return ve.Code
}

// availabilityFixture returns a store where anon participant "p1" on "cal1"
// holds token "tok-p1", and records what SetAvailability received.
type availabilityCapture struct {
	called    bool
	dates     []string
	notes     map[string]string
	preferred map[string]bool
}

func availabilityFixture(c *availabilityCapture) *mockStore {
	return &mockStore{
		tokenHashes: map[string]string{"p1": hashParticipantToken("tok-p1")},
		getParticipant: func(_ context.Context, id string) (model.Participant, error) {
			return model.Participant{ID: id, CalendarID: "cal1"}, nil
		},
		setAvailability: func(_ context.Context, _ string, dates []string, notes map[string]string, preferred map[string]bool) error {
			c.called = true
			c.dates, c.notes, c.preferred = dates, notes, preferred
			return nil
		},
	}
}

func notesOf(m map[string]string) *map[string]string { return &m }
func prefOf(s ...string) *[]string                   { return &s }

func TestSetAvailability_ValidationRules(t *testing.T) {
	tooMany := make([]string, limits.MaxDatesPerSubmission+1)
	for i := range tooMany {
		tooMany[i] = testDate
	}
	cases := []struct {
		name   string
		pid    string
		update AvailabilityUpdate
		code   string
	}{
		{"missing participant id", "", AvailabilityUpdate{Dates: []string{testDate}}, "invalid_request"},
		{"too many dates", "p1", AvailabilityUpdate{Dates: tooMany}, "too_many_dates"},
		{"bad date format", "p1", AvailabilityUpdate{Dates: []string{"05/01/2026"}}, "invalid_date"},
		{"date out of range", "p1", AvailabilityUpdate{Dates: []string{"2099-05-01"}}, "date_out_of_range"},
		{"note for unknown date", "p1", AvailabilityUpdate{Dates: []string{testDate}, Notes: notesOf(map[string]string{testDate2: "x"})}, "note_unknown_date"},
		{"note with newline", "p1", AvailabilityUpdate{Dates: []string{testDate}, Notes: notesOf(map[string]string{testDate: "a\nb"})}, "note_newline"},
		{"note with carriage return", "p1", AvailabilityUpdate{Dates: []string{testDate}, Notes: notesOf(map[string]string{testDate: "a\rb"})}, "note_newline"},
		{"whitespace-only note", "p1", AvailabilityUpdate{Dates: []string{testDate}, Notes: notesOf(map[string]string{testDate: "   "})}, "note_whitespace"},
		{"note too long", "p1", AvailabilityUpdate{Dates: []string{testDate}, Notes: notesOf(map[string]string{testDate: strings.Repeat("é", limits.MaxNoteLength+1)})}, "note_too_long"},
		{"preferred for unknown date", "p1", AvailabilityUpdate{Dates: []string{testDate}, Preferred: prefOf(testDate2)}, "preferred_unknown_date"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var c availabilityCapture
			s := newSvc(t, availabilityFixture(&c))
			err := s.SetAvailability(context.Background(), "cal1", tc.pid, ParticipantCaller{Token: "tok-p1"}, tc.update)
			if got := validationCode(t, err); got != tc.code {
				t.Fatalf("code = %q, want %q", got, tc.code)
			}
			if c.called {
				t.Fatal("store must not be written on validation failure")
			}
		})
	}
}

func TestSetAvailability_ValidationRunsBeforeCalendarLoad(t *testing.T) {
	ms := &mockStore{
		getCalendarOnly: func(context.Context, string) (model.Calendar, error) {
			t.Fatal("calendar loaded before input was validated")
			return model.Calendar{}, nil
		},
	}
	s := newSvc(t, ms)
	err := s.SetAvailability(context.Background(), "cal1", "p1", ParticipantCaller{}, AvailabilityUpdate{Dates: []string{"nope"}})
	if validationCode(t, err) != "invalid_date" {
		t.Fatalf("err = %v", err)
	}
}

func TestSetAvailability_Overrides(t *testing.T) {
	cases := []struct {
		name          string
		update        AvailabilityUpdate
		wantNotes     map[string]string // nil = expect nil map (don't touch)
		wantPreferred map[string]bool   // nil = expect nil map (don't touch)
	}{
		{
			name:   "absent notes and preferred stay nil",
			update: AvailabilityUpdate{Dates: []string{testDate}},
		},
		{
			name:      "notes are trimmed and only cover supplied keys",
			update:    AvailabilityUpdate{Dates: []string{testDate, testDate2}, Notes: notesOf(map[string]string{testDate: "  only after 6pm  "})},
			wantNotes: map[string]string{testDate: "only after 6pm"},
		},
		{
			name:      "empty note is an explicit clear",
			update:    AvailabilityUpdate{Dates: []string{testDate}, Notes: notesOf(map[string]string{testDate: ""})},
			wantNotes: map[string]string{testDate: ""},
		},
		{
			name:      "note at the length boundary is accepted",
			update:    AvailabilityUpdate{Dates: []string{testDate}, Notes: notesOf(map[string]string{testDate: strings.Repeat("é", limits.MaxNoteLength)})},
			wantNotes: map[string]string{testDate: strings.Repeat("é", limits.MaxNoteLength)},
		},
		{
			name:          "preferred builds a full map over dates",
			update:        AvailabilityUpdate{Dates: []string{testDate, testDate2}, Preferred: prefOf(testDate2)},
			wantPreferred: map[string]bool{testDate: false, testDate2: true},
		},
		{
			name:          "empty preferred clears every date",
			update:        AvailabilityUpdate{Dates: []string{testDate, testDate2}, Preferred: prefOf()},
			wantPreferred: map[string]bool{testDate: false, testDate2: false},
		},
		{
			name:          "duplicate preferred dates are deduped",
			update:        AvailabilityUpdate{Dates: []string{testDate}, Preferred: prefOf(testDate, testDate)},
			wantPreferred: map[string]bool{testDate: true},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var c availabilityCapture
			s := newSvc(t, availabilityFixture(&c))
			if err := s.SetAvailability(context.Background(), "cal1", "p1", ParticipantCaller{Token: "tok-p1"}, tc.update); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !c.called {
				t.Fatal("store not called")
			}
			if (tc.wantNotes == nil) != (c.notes == nil) {
				t.Fatalf("notes = %v, want %v", c.notes, tc.wantNotes)
			}
			if len(c.notes) != len(tc.wantNotes) {
				t.Fatalf("notes = %v, want %v", c.notes, tc.wantNotes)
			}
			for k, v := range tc.wantNotes {
				if got, ok := c.notes[k]; !ok || got != v {
					t.Fatalf("notes[%s] = %q (present=%v), want %q", k, got, ok, v)
				}
			}
			if (tc.wantPreferred == nil) != (c.preferred == nil) {
				t.Fatalf("preferred = %v, want %v", c.preferred, tc.wantPreferred)
			}
			if len(c.preferred) != len(tc.wantPreferred) {
				t.Fatalf("preferred = %v, want %v", c.preferred, tc.wantPreferred)
			}
			for k, v := range tc.wantPreferred {
				if got, ok := c.preferred[k]; !ok || got != v {
					t.Fatalf("preferred[%s] = %v (present=%v), want %v", k, got, ok, v)
				}
			}
		})
	}
}

func TestImportAvailability_ValidationRules(t *testing.T) {
	tooMany := make([]string, limits.MaxDatesPerSubmission+1)
	for i := range tooMany {
		tooMany[i] = testDate
	}
	cases := []struct {
		name  string
		pid   string
		dates []string
		code  string
	}{
		{"missing participant id", "", []string{testDate}, "invalid_request"},
		{"too many dates", "p1", tooMany, "too_many_dates"},
		{"bad date format", "p1", []string{"2026/05/01"}, "invalid_date"},
		{"date out of range", "p1", []string{"1999-01-01"}, "date_out_of_range"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newSvc(t, &mockStore{}) // any store call would panic
			err := s.ImportAvailability(context.Background(), "cal1", tc.pid, ParticipantCaller{}, tc.dates)
			if got := validationCode(t, err); got != tc.code {
				t.Fatalf("code = %q, want %q", got, tc.code)
			}
		})
	}
}

func TestRenameParticipant_NameRules(t *testing.T) {
	cases := []struct {
		name  string
		input string
		code  string
	}{
		{"empty", "", "invalid_request"},
		{"whitespace only", "   ", "invalid_request"},
		{"too long", strings.Repeat("x", limits.MaxParticipantNameLength+1), "name_too_long"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Validation runs before the calendar is loaded: a finalised
			// calendar must not mask a bad name (handler order preserved).
			date := testDate
			ms := authFixture()
			ms.getCalendarOnly = func(_ context.Context, id string) (model.Calendar, error) {
				return model.Calendar{ID: id, OwnerID: "owner1", FinalisedDate: &date}, nil
			}
			s := newSvc(t, ms)
			_, err := s.RenameParticipant(context.Background(), "cal1", "anon", ParticipantCaller{Token: "tok-anon"}, tc.input, nil)
			if got := validationCode(t, err); got != tc.code {
				t.Fatalf("code = %q, want %q", got, tc.code)
			}
			if !errors.Is(err, ErrInvalidName) {
				t.Fatalf("err = %v, want it to wrap ErrInvalidName", err)
			}
		})
	}
}

func TestRenameParticipant_TrimsName(t *testing.T) {
	ms := authFixture()
	var stored string
	ms.updateParticipantName = func(_ context.Context, calendarID, pid, name string) (model.Participant, error) {
		stored = name
		return model.Participant{ID: pid, CalendarID: calendarID, Name: name}, nil
	}
	s := newSvc(t, ms)
	limitName := strings.Repeat("x", limits.MaxParticipantNameLength)
	if _, err := s.RenameParticipant(context.Background(), "cal1", "anon", ParticipantCaller{Token: "tok-anon"}, "  "+limitName+"  ", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stored != limitName {
		t.Fatalf("stored name = %q, want trimmed %q", stored, limitName)
	}
}
