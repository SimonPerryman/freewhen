// Package ics generates minimal iCalendar (.ics) payloads for a single
// all-day event. Hand-rolled rather than pulling in a library — we only
// ever emit one VEVENT with a fixed shape.
package ics

import (
	"fmt"
	"strings"
	"time"
)

// Event describes the single all-day event we serialise.
type Event struct {
	// UID is a globally-unique identifier for the event. Required by RFC 5545.
	UID string
	// Summary is the event title shown in the user's calendar.
	Summary string
	// Description is the long-form body shown in the event details. Newlines
	// permitted; escaping and line folding are applied at render time.
	Description string
	// Date is the local date the event falls on. We emit DTSTART;VALUE=DATE
	// and DTEND as the day after (per RFC 5545 "DTEND is exclusive" rule).
	Date time.Time
	// DTStamp is the file-creation timestamp. Defaults to now() if zero.
	DTStamp time.Time
}

// Render returns the .ics file bytes for evt. Line endings are CRLF per the spec.
func Render(evt Event) string {
	stamp := evt.DTStamp
	if stamp.IsZero() {
		stamp = time.Now().UTC()
	}

	var b strings.Builder
	write := func(line string) { b.WriteString(foldLine(line)); b.WriteString("\r\n") }

	write("BEGIN:VCALENDAR")
	write("VERSION:2.0")
	write("PRODID:-//FreeWhen//EN")
	write("CALSCALE:GREGORIAN")
	write("METHOD:PUBLISH")
	write("BEGIN:VEVENT")
	write("UID:" + evt.UID)
	write(fmt.Sprintf("DTSTAMP:%s", stamp.UTC().Format("20060102T150405Z")))
	write(fmt.Sprintf("DTSTART;VALUE=DATE:%s", evt.Date.Format("20060102")))
	write(fmt.Sprintf("DTEND;VALUE=DATE:%s", evt.Date.AddDate(0, 0, 1).Format("20060102")))
	write("SUMMARY:" + escapeText(evt.Summary))
	if evt.Description != "" {
		write("DESCRIPTION:" + escapeText(evt.Description))
	}
	write("TRANSP:TRANSPARENT")
	write("END:VEVENT")
	write("END:VCALENDAR")
	return b.String()
}

// foldLine applies RFC 5545 §3.1 line folding: any line longer than 75 octets
// is split, with each continuation line prefixed by a single space. Folding is
// octet-based, but we must not split inside a UTF-8 multibyte sequence — so we
// step back to a rune boundary if the 75th octet lands mid-character.
func foldLine(line string) string {
	const limit = 75
	if len(line) <= limit {
		return line
	}
	var b strings.Builder
	b.Grow(len(line) + len(line)/limit*3)
	for len(line) > limit {
		split := limit
		// Walk back to a UTF-8 rune boundary (continuation bytes are 10xxxxxx).
		for split > 0 && line[split]&0xC0 == 0x80 {
			split--
		}
		b.WriteString(line[:split])
		b.WriteString("\r\n ")
		line = line[split:]
	}
	b.WriteString(line)
	return b.String()
}

// escapeText applies RFC 5545 §3.3.11 text escaping: backslash, comma,
// semicolon, and line breaks must be escaped.
func escapeText(s string) string {
	r := strings.NewReplacer(
		`\`, `\\`,
		",", `\,`,
		";", `\;`,
		"\r\n", `\n`,
		"\n", `\n`,
		"\r", `\n`,
	)
	return r.Replace(s)
}
