package service

import (
	"context"
	"errors"
	"strings"
	"unicode"

	"github.com/freewhen/backend/internal/limits"
	"github.com/freewhen/backend/internal/model"
	"github.com/freewhen/backend/internal/store"
)

// Invite validation / mutation errors.
var (
	ErrInviteNameRequired       = errors.New("invite name required")
	ErrInviteNameTooLong        = errors.New("invite name too long")
	ErrParticipantNotOnCalendar = errors.New("participant not on calendar")
)

// AddInvitesResult is the service-level response for a bulk add. It splits the
// input into three buckets so the frontend can report partial success without
// reimplementing the parsing rules.
type AddInvitesResult struct {
	Added   []model.Invite `json:"added"`
	Skipped []SkippedName  `json:"skipped"`
	Invalid []InvalidName  `json:"invalid"`
}

// SkippedName is an invite that couldn't be inserted because an invite with
// the same normalised name already exists (or the same name appears twice in
// the same paste).
type SkippedName struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// InvalidName is a line that failed server-side validation (length,
// control-chars, empty after trim). Reported back so the owner can fix the
// offending line without losing the whole paste.
type InvalidName struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// MissingView is the shape returned by GET /calendars/{id}/missing.
type MissingView struct {
	Invites            []model.Invite   `json:"invites"`
	NotJoined          []NotJoinedEntry `json:"not_joined"`
	JoinedNotResponded []JoinedEntry    `json:"joined_not_responded"`
}

type NotJoinedEntry struct {
	InviteID string `json:"invite_id"`
	Name     string `json:"name"`
}

type JoinedEntry struct {
	ParticipantID string `json:"participant_id"`
	Name          string `json:"name"`
}

// inviteOnCalendar loads an invite and verifies it belongs to calendarID.
// An invite on another calendar is reported as ErrInviteNotFound so ids
// can't be probed across calendars.
func (s *Service) inviteOnCalendar(ctx context.Context, calendarID, inviteID string) (model.Invite, error) {
	inv, err := s.store.GetInvite(ctx, inviteID)
	if err != nil {
		return model.Invite{}, err
	}
	if inv.CalendarID != calendarID {
		return model.Invite{}, model.ErrInviteNotFound
	}
	return inv, nil
}

// ParseBulkNames splits a paste-style input on both commas and newlines. Empty
// segments and leading/trailing whitespace are trimmed. Used by AddInvites and
// exposed for tests.
func ParseBulkNames(raw []string) []string {
	out := []string{}
	for _, s := range raw {
		// Split each entry on both commas and newlines.
		parts := strings.FieldsFunc(s, func(r rune) bool {
			return r == ',' || r == '\n' || r == '\r'
		})
		for _, p := range parts {
			name := strings.TrimSpace(p)
			if name != "" {
				out = append(out, name)
			}
		}
	}
	return out
}

// validateName checks that a name is non-empty, within the length cap, and
// contains no control characters. Whitespace-only and empty inputs are
// collapsed to a single "required" error.
func validateInviteName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ErrInviteNameRequired
	}
	if len(trimmed) > limits.MaxParticipantNameLength {
		return ErrInviteNameTooLong
	}
	for _, r := range trimmed {
		if unicode.IsControl(r) {
			return ErrInviteNameRequired
		}
	}
	return nil
}

// AddInvites parses, validates, and inserts a batch of invite names. Invalid
// lines are reported in Invalid; already-present names in Skipped; fresh rows
// in Added. The whole operation goes through a single store tx so the cap is
// enforced atomically. Owner-only.
func (s *Service) AddInvites(ctx context.Context, calendarID, userID string, rawNames []string) (AddInvitesResult, error) {
	res := AddInvitesResult{Added: []model.Invite{}, Skipped: []SkippedName{}, Invalid: []InvalidName{}}
	if _, err := s.requireOwner(ctx, calendarID, userID); err != nil {
		return res, err
	}
	if len(rawNames) == 0 {
		return res, invalid("names_required", "names is required")
	}
	parsed := ParseBulkNames(rawNames)

	// Validate and dedupe within the batch (case-insensitive on trimmed name).
	// Later duplicates are reported as "duplicate" in Skipped.
	seen := map[string]bool{}
	valid := []string{}
	for _, name := range parsed {
		if err := validateInviteName(name); err != nil {
			reason := "name_required"
			if errors.Is(err, ErrInviteNameTooLong) {
				reason = "name_too_long"
			}
			res.Invalid = append(res.Invalid, InvalidName{Name: name, Reason: reason})
			continue
		}
		key := strings.ToLower(strings.TrimSpace(name))
		if seen[key] {
			res.Skipped = append(res.Skipped, SkippedName{Name: name, Reason: "duplicate"})
			continue
		}
		seen[key] = true
		valid = append(valid, strings.TrimSpace(name))
	}

	if len(valid) == 0 {
		return res, nil
	}

	storeRes, err := s.store.AddInvites(ctx, calendarID, valid)
	if err != nil {
		return res, err
	}
	res.Added = append(res.Added, storeRes.Added...)
	for _, n := range storeRes.Skipped {
		res.Skipped = append(res.Skipped, SkippedName{Name: n, Reason: "duplicate"})
	}
	return res, nil
}

// RenameInvite renames an existing invite on calendarID after server-side
// validation. Owner-only.
func (s *Service) RenameInvite(ctx context.Context, calendarID, userID, inviteID, name string) (model.Invite, error) {
	if _, err := s.requireOwner(ctx, calendarID, userID); err != nil {
		return model.Invite{}, err
	}
	if _, err := s.inviteOnCalendar(ctx, calendarID, inviteID); err != nil {
		return model.Invite{}, err
	}
	trimmed := strings.TrimSpace(name)
	if err := validateInviteName(trimmed); err != nil {
		return model.Invite{}, err
	}
	return s.store.UpdateInviteName(ctx, inviteID, trimmed)
}

// DeleteInvite removes an invite. Matched participants are untouched (ON DELETE
// SET NULL on matched_participant_id handles the reverse direction).
// Owner-only. Returns ErrInviteNotFound only when the invite was never on
// this calendar; losing a race with a concurrent delete is success.
func (s *Service) DeleteInvite(ctx context.Context, calendarID, userID, inviteID string) error {
	if _, err := s.requireOwner(ctx, calendarID, userID); err != nil {
		return err
	}
	if _, err := s.inviteOnCalendar(ctx, calendarID, inviteID); err != nil {
		return err
	}
	if err := s.store.DeleteInvite(ctx, inviteID); err != nil && !errors.Is(err, model.ErrInviteNotFound) {
		return err
	}
	return nil
}

// ManualMatchInvite links or unlinks (participantID == nil) an invite.
// Owner-only. If a participant ID is supplied, it must belong to the same
// calendar as the invite; otherwise ErrParticipantNotOnCalendar is returned.
func (s *Service) ManualMatchInvite(ctx context.Context, calendarID, userID, inviteID string, participantID *string) (model.Invite, error) {
	if _, err := s.requireOwner(ctx, calendarID, userID); err != nil {
		return model.Invite{}, err
	}
	if _, err := s.inviteOnCalendar(ctx, calendarID, inviteID); err != nil {
		return model.Invite{}, err
	}
	if participantID != nil {
		p, err := s.store.GetParticipant(ctx, *participantID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return model.Invite{}, ErrParticipantNotOnCalendar
			}
			return model.Invite{}, err
		}
		if p.CalendarID != calendarID {
			return model.Invite{}, ErrParticipantNotOnCalendar
		}
	}
	return s.store.SetInviteMatch(ctx, inviteID, participantID)
}

// GetMissingView composes the two derived buckets from a single pair of store
// reads. "Not joined" = invites with no matched participant; "Joined but not
// responded" = participants with zero availability rows. The same "has any
// availability" signal powers submit-to-view, so this bucket stays in lockstep
// with that reveal check. Owner-only.
func (s *Service) GetMissingView(ctx context.Context, calendarID, userID string) (MissingView, error) {
	view := MissingView{
		Invites:            []model.Invite{},
		NotJoined:          []NotJoinedEntry{},
		JoinedNotResponded: []JoinedEntry{},
	}
	if _, err := s.requireOwner(ctx, calendarID, userID); err != nil {
		return view, err
	}

	invites, err := s.store.ListInvites(ctx, calendarID)
	if err != nil {
		return view, err
	}
	view.Invites = invites
	for _, inv := range invites {
		if inv.MatchedParticipantID == nil {
			view.NotJoined = append(view.NotJoined, NotJoinedEntry{InviteID: inv.ID, Name: inv.Name})
		}
	}

	quiet, err := s.store.ParticipantsWithNoAvailability(ctx, calendarID)
	if err != nil {
		return view, err
	}
	for _, p := range quiet {
		view.JoinedNotResponded = append(view.JoinedNotResponded, JoinedEntry{ParticipantID: p.ID, Name: p.Name})
	}
	return view, nil
}
