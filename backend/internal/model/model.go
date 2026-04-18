package model

import "time"

type Calendar struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Emoji     string    `json:"emoji,omitempty"`
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
