package model

import "time"

type Calendar struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Emoji     string    `json:"emoji,omitempty"`
	OwnerID   string    `json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
}

type Participant struct {
	ID         string `json:"id"`
	CalendarID string `json:"calendar_id"`
	Name       string `json:"name"`
	Colour     string `json:"colour"`
}

type Availability struct {
	ParticipantID string `json:"participant_id"`
	CalendarID    string `json:"calendar_id"`
	Date          string `json:"date"`
}

type CalendarWithDetails struct {
	Calendar     Calendar       `json:"calendar"`
	Participants []Participant  `json:"participants"`
	Availability []Availability `json:"availability"`
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

type Session struct {
	Token     string
	UserID    string
	ExpiresAt time.Time
	CreatedAt time.Time
}
