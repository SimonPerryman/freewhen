package ics

import (
	"strings"
	"testing"
	"time"
)

func TestRender(t *testing.T) {
	evt := Event{
		UID:     "cal-abc123@freewhen",
		Summary: "Team dinner",
		Date:    time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC),
		DTStamp: time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC),
	}
	out := Render(evt)

	wantLines := []string{
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		"PRODID:-//FreeWhen//EN",
		"BEGIN:VEVENT",
		"UID:cal-abc123@freewhen",
		"DTSTAMP:20260423T120000Z",
		"DTSTART;VALUE=DATE:20260515",
		"DTEND;VALUE=DATE:20260516",
		"SUMMARY:Team dinner",
		"END:VEVENT",
		"END:VCALENDAR",
	}
	for _, line := range wantLines {
		if !strings.Contains(out, line+"\r\n") {
			t.Errorf("missing line %q in:\n%s", line, out)
		}
	}
}

func TestRenderEscapesSpecialChars(t *testing.T) {
	evt := Event{
		UID:     "x",
		Summary: "A, B; C\\D\nE",
		Date:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		DTStamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	out := Render(evt)
	if !strings.Contains(out, `SUMMARY:A\, B\; C\\D\nE`) {
		t.Errorf("expected escaped summary, got:\n%s", out)
	}
}

func TestRenderEndsWithCRLF(t *testing.T) {
	out := Render(Event{UID: "x", Summary: "y", Date: time.Now(), DTStamp: time.Now()})
	if !strings.HasSuffix(out, "\r\n") {
		t.Errorf("output should end with CRLF")
	}
}
