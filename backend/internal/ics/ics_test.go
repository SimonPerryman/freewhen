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

func TestRenderDescription(t *testing.T) {
	evt := Event{
		UID:         "x",
		Summary:     "Dinner",
		Description: "Going: Alice, Bob\n\nNotes:\n• Alice — Have to leave by 9pm",
		Date:        time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		DTStamp:     time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
	}
	out := Render(evt)
	unfolded := strings.ReplaceAll(out, "\r\n ", "")
	if !strings.Contains(unfolded, `DESCRIPTION:Going: Alice\, Bob\n\nNotes:\n• Alice — Have to leave by 9pm`) {
		t.Errorf("expected escaped description, got:\n%s", out)
	}
}

func TestRenderOmitsEmptyDescription(t *testing.T) {
	out := Render(Event{UID: "x", Summary: "Dinner", Date: time.Now(), DTStamp: time.Now()})
	if strings.Contains(out, "DESCRIPTION:") {
		t.Errorf("empty description should be omitted, got:\n%s", out)
	}
}

func TestRenderFoldsLongDescription(t *testing.T) {
	long := strings.Repeat("abcdefghij", 12) // 120 octets
	out := Render(Event{
		UID:         "x",
		Summary:     "y",
		Description: long,
		Date:        time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		DTStamp:     time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
	})
	// Every line in the output (except CRLF terminators) must be ≤ 75 octets.
	for _, line := range strings.Split(out, "\r\n") {
		if len(line) > 75 {
			t.Errorf("line exceeds 75 octets (%d): %q", len(line), line)
		}
	}
	// Continuation lines start with a single space.
	if !strings.Contains(out, "\r\n ") {
		t.Errorf("expected folded continuation with leading space, got:\n%s", out)
	}
	// Unfolded content must be preserved when joined back.
	unfolded := strings.ReplaceAll(out, "\r\n ", "")
	if !strings.Contains(unfolded, "DESCRIPTION:"+long) {
		t.Errorf("unfolded description missing; out:\n%s", out)
	}
}

func TestFoldRespectsUTF8Boundary(t *testing.T) {
	// Build a line where the 75th octet would split a multibyte rune.
	// "•" is 3 bytes (0xE2 0x80 0xA2). Pad so the bullet straddles octet 75.
	prefix := strings.Repeat("a", 73)
	line := "DESCRIPTION:" + prefix + "•end" // way over 75 octets total
	out := foldLine(line)
	// No fold split should occur inside the multibyte rune; the "•" must
	// remain intact in the output.
	if !strings.Contains(strings.ReplaceAll(out, "\r\n ", ""), "•end") {
		t.Errorf("multibyte rune got split; out:\n%s", out)
	}
}

func TestRenderEndsWithCRLF(t *testing.T) {
	out := Render(Event{UID: "x", Summary: "y", Date: time.Now(), DTStamp: time.Now()})
	if !strings.HasSuffix(out, "\r\n") {
		t.Errorf("output should end with CRLF")
	}
}
