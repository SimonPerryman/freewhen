package notifications

import (
	"fmt"
	"html"

	"github.com/freewhen/backend/internal/model"
)

// All templates render a single layout: a header line, a body block, and a
// footer attribution. Plaintext-style HTML — every Resend client renders it,
// and we don't have to ship images or CSS that get eaten by Outlook.

func layout(body, footer string) string {
	return `<div style="font-family: system-ui, sans-serif; max-width: 480px; margin: 0 auto; padding: 32px 20px; color: #111;">` +
		body +
		`<p style="color: #999; font-size: 12px; margin-top: 32px;">` + footer + `</p>` +
		`</div>`
}

func renderFinalised(ev FinaliseEvent, p model.Participant, unsubscribeURL string) string {
	calName := html.EscapeString(ev.Calendar.Name)
	body := fmt.Sprintf(`
  <h1 style="font-size: 22px; margin: 0 0 16px;">📅 %s</h1>
  <p style="line-height: 1.5; margin: 0 0 12px;">Hi %s,</p>
  <p style="line-height: 1.5; margin: 0 0 16px;"><strong>%s</strong> picked <strong>%s</strong> for %s.</p>
  <ul style="list-style: none; padding: 0; line-height: 1.8;">
    <li>📌 <a href="%s">Add to your calendar</a></li>
    <li>⬇️ <a href="%s">Download .ics</a></li>
    <li>👀 <a href="%s">View calendar</a></li>
  </ul>`,
		calName,
		html.EscapeString(p.Name),
		html.EscapeString(ev.OwnerName),
		html.EscapeString(ev.FormattedDate),
		calName,
		html.EscapeString(ev.GCalURL),
		html.EscapeString(ev.ICSURL),
		html.EscapeString(ev.CalendarURL),
	)
	footer := fmt.Sprintf("You got this because you joined %s on FreeWhen. <a href=\"%s\">Unsubscribe</a>",
		calName, html.EscapeString(unsubscribeURL))
	return layout(body, footer)
}

func renderFinalisedOwner(ev FinaliseEvent) string {
	calName := html.EscapeString(ev.Calendar.Name)
	body := fmt.Sprintf(`
  <h1 style="font-size: 22px; margin: 0 0 16px;">✅ %s</h1>
  <p style="line-height: 1.5; margin: 0 0 12px;">Hi %s,</p>
  <p style="line-height: 1.5; margin: 0 0 16px;">You picked <strong>%s</strong> for %s.</p>
  <p style="line-height: 1.5; margin: 0 0 12px;">Forward these so everyone has it on their calendar:</p>
  <ul style="list-style: none; padding: 0; line-height: 1.8;">
    <li>📌 <a href="%s">Add to Google Calendar</a></li>
    <li>⬇️ <a href="%s">Download .ics</a></li>
    <li>👀 <a href="%s">View calendar</a></li>
  </ul>`,
		calName,
		html.EscapeString(ev.OwnerName),
		html.EscapeString(ev.FormattedDate),
		calName,
		html.EscapeString(ev.GCalURL),
		html.EscapeString(ev.ICSURL),
		html.EscapeString(ev.CalendarURL),
	)
	footer := fmt.Sprintf("You got this because you finalised %s on FreeWhen.", calName)
	return layout(body, footer)
}

func renderNudge(cal model.Calendar, p model.Participant, ownerName, calendarURL, unsubscribeURL string) string {
	calName := html.EscapeString(cal.Name)
	body := fmt.Sprintf(`
  <h1 style="font-size: 22px; margin: 0 0 16px;">📅 Reminder</h1>
  <p style="line-height: 1.5; margin: 0 0 12px;">Hi %s,</p>
  <p style="line-height: 1.5; margin: 0 0 16px;"><strong>%s</strong> is still waiting on availability for %s.</p>
  <p><a href="%s" style="display: inline-block; padding: 10px 20px; background: #111; color: #fff; text-decoration: none; border-radius: 8px; font-weight: 500;">Mark your availability</a></p>`,
		html.EscapeString(p.Name),
		html.EscapeString(ownerName),
		calName,
		html.EscapeString(calendarURL),
	)
	footer := fmt.Sprintf("You got this because you joined %s on FreeWhen. <a href=\"%s\">Unsubscribe</a>",
		calName, html.EscapeString(unsubscribeURL))
	return layout(body, footer)
}

// UnsubscribePage renders the static "you've been unsubscribed" landing page.
// Returned as a complete HTML document so the API can serve it without a
// dependency on the frontend being up.
func UnsubscribePage(success bool) string {
	if success {
		return `<!doctype html><html lang="en"><head><meta charset="utf-8"><title>Unsubscribed</title><meta name="viewport" content="width=device-width,initial-scale=1"></head>` +
			`<body style="font-family:system-ui,sans-serif;max-width:480px;margin:64px auto;padding:0 20px;color:#111;">` +
			`<h1 style="font-size:22px;">You're unsubscribed</h1>` +
			`<p style="line-height:1.5;color:#555;">We won't email you about this calendar again. You can edit your participant on FreeWhen if you change your mind.</p>` +
			`</body></html>`
	}
	return `<!doctype html><html lang="en"><head><meta charset="utf-8"><title>Link expired</title><meta name="viewport" content="width=device-width,initial-scale=1"></head>` +
		`<body style="font-family:system-ui,sans-serif;max-width:480px;margin:64px auto;padding:0 20px;color:#111;">` +
		`<h1 style="font-size:22px;">This unsubscribe link is invalid or expired.</h1>` +
		`<p style="line-height:1.5;color:#555;">Open FreeWhen and edit your participant to clear your email.</p>` +
		`</body></html>`
}
