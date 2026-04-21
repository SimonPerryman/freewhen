package model

import "time"

const (
	VisibilityOpen          = "open"
	VisibilitySubmitToView  = "submit_to_view"
)

func IsValidVisibility(v string) bool {
	return v == VisibilityOpen || v == VisibilitySubmitToView
}

type Calendar struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Emoji         string     `json:"emoji,omitempty"`
	Theme         string     `json:"theme,omitempty"`
	OwnerID       string     `json:"owner_id"`
	Visibility    string     `json:"visibility"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	FinalisedDate *string    `json:"finalised_date,omitempty"`
	FinalisedAt   *time.Time `json:"finalised_at,omitempty"`
	// Passphrase is the server-generated plaintext gating this calendar. Empty
	// when no passphrase is set. Never serialised here directly — handlers must
	// only expose it to the calendar owner (see handler/calendar.go).
	Passphrase string `json:"-"`
	// AccessEpoch is bumped on every passphrase change. Baked into issued access
	// tokens; stale-epoch tokens are rejected.
	AccessEpoch int `json:"-"`
}

// HasPassphrase reports whether the calendar is gated by a passphrase.
func (c Calendar) HasPassphrase() bool { return c.Passphrase != "" }

type Participant struct {
	ID         string  `json:"id"`
	CalendarID string  `json:"calendar_id"`
	Name       string  `json:"name"`
	Colour     string  `json:"colour"`
	UserID     *string `json:"user_id,omitempty"`
}

type Availability struct {
	ParticipantID string  `json:"participant_id"`
	CalendarID    string  `json:"calendar_id"`
	Date          string  `json:"date"`
	Note          *string `json:"note,omitempty"`
	Preferred     bool    `json:"preferred,omitempty"`
}

type CalendarWithDetails struct {
	Calendar         Calendar       `json:"calendar"`
	Participants     []Participant  `json:"participants"`
	Availability     []Availability `json:"availability"`
	ParticipantCount int            `json:"participantCount"`
}

type User struct {
	ID         string    `json:"id"`
	Email      string    `json:"email"`
	Name       string    `json:"name"`
	IsAdmin    bool      `json:"is_admin"`
	Provider   string    `json:"-"`
	ProviderID string    `json:"-"`
	CreatedAt  time.Time `json:"-"`
}

type Invite struct {
	ID                   string    `json:"id"`
	CalendarID           string    `json:"calendar_id"`
	Name                 string    `json:"name"`
	MatchedParticipantID *string   `json:"matched_participant_id"`
	CreatedAt            time.Time `json:"created_at"`
}

type Session struct {
	Token     string
	UserID    string
	ExpiresAt time.Time
	CreatedAt time.Time
}
