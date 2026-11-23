# Richer Google Calendar / .ics export event

Status: done

## Goal

When a user taps "Add to Google Calendar" (or downloads the .ics) on a finalised calendar, the resulting event has only a title (e.g. `🎉 Sam's Birthday`). That's a missed opportunity:

- **No attribution back to FreeWhen.** Once the event lands in someone's personal calendar, the trail to the tool is gone — no chance of repeat use.
- **No context about who's coming.** People scanning their calendar weeks later have to remember (or dig) to find out who else is there.
- **No notes.** Caveats people attached to the day ("after 6pm", "can host") were the basis for picking the date and are exactly the thing the host re-reads later — but they live only inside the FreeWhen UI.

This plan enriches the exported event body (Google Calendar `details` param + ICS `DESCRIPTION`) with attendees, notes, and a link back to the calendar so repeat-use friction drops.

## Scope decisions (already made)

- **Skip the `Preferred` signal.** Once the date is locked in, who flagged it as their top pick is no longer interesting. Don't render stars, don't mention it.
- **Always include the link back to FreeWhen.** Driving repeat use is the main reason this plan exists. Calendar is already gated by passphrase / access epoch, so leaking the URL via a shared event is fine.
- **Include per-day notes when present.** Each note rendered as `<Name>: <note>`. Omit the whole "Notes" section if no participant left a note for the finalised date.
- **All-day event shape stays the same.** No change to date handling, transparency, or UID — only the description (and the ICS gets a new `DESCRIPTION` line).

## Plan

1. **Switch handlers to the full calendar load.**
   - `backend/internal/handler/export.go` — both `ExportICS` and `ExportGoogle` currently call `service.GetCalendarOnly`. Switch to `service.GetCalendar` so we get `Participants` + `Availability`. Exports are rare; the heavier query is fine.

2. **Build a shared description string.**
   - New helper (e.g. `calendarEventDescription(cal model.CalendarWithDetails, frontendBase string) string`) co-located with `calendarSummary` in `export.go`, or moved into a small `internal/exportfmt` package if it grows.
   - Inputs: the `CalendarWithDetails` and the frontend base URL (see step 4).
   - Filter `Availability` to rows where `Date == *cal.FinalisedDate`. Join to `Participants` by `ParticipantID` to get names.
   - Render shape:
     ```
     Going: Alice, Bob, Carla

     Notes:
     • Alice — Have to leave by 9pm
     • Bob — Can host

     View on FreeWhen: https://freewhen.simonperryman.co.uk/calendars/<id>
     Created with FreeWhen — https://freewhen.simonperryman.co.uk
     ```
     - Sort participants alphabetically by name (stable, no implicit "owner first" surprise).
     - Skip the `Notes:` block entirely if no one left a note.
     - If zero participants are available for the finalised date (shouldn't happen but defend), render only the FreeWhen attribution lines.

3. **Wire the description into both exports.**
   - `ExportICS` — pass through to `ics.Render`. Add a `Description` field to `ics.Event` and emit a `DESCRIPTION:` line (with the existing `escapeText` helper, which already handles newlines per RFC 5545 §3.3.11). Note: `DESCRIPTION` lines longer than 75 octets must be folded per RFC 5545 §3.1 — add a small line-folder in `ics.go` and apply it to the description (and ideally other long lines too). Without folding, Google Calendar will still import but stricter clients (Apple Calendar, Outlook desktop) may truncate or warn.
   - `ExportGoogle` — add `q.Set("details", description)` before encoding. Google's render URL accepts plain text with `\n` newlines (auto-linkified URLs); no HTML needed.

4. **Frontend base URL config.**
   - Need a canonical public URL for the calendar link. Add a `FRONTEND_BASE_URL` env var to `internal/config/config.go` (e.g. `https://freewhen.simonperryman.co.uk` in prod, `http://localhost:3000` in dev). Read once at startup; pass into the handler via the existing handler struct.
   - 12-factor: explicit env var, no `NODE_ENV`-style branching. Document in `docs/infra-notes.md` and add to Terraform Cloud Run env in `infra/`.

5. **Tests.**
   - `backend/internal/handler/calendar_integration_test.go` (or a new `export_test.go` if the file's getting busy) — extend the existing export test cases:
     - Finalised calendar with 3 participants, 2 of whom have notes → assert Going line, Notes block, both URLs present in the Google redirect query and ICS body.
     - Finalised calendar with no notes → assert no `Notes:` header in either output.
     - Existing not-finalised / not-found cases should still pass unchanged.
   - `backend/internal/ics/ics_test.go` — add cases for `DESCRIPTION` rendering, escape behaviour on commas/semicolons/newlines inside notes, and line-folding of long descriptions (≥ 75 octets).

6. **Manual verification.**
   - `make dev-backend` + `make dev-frontend`; finalise a test calendar with multiple participants and at least one note; tap "Add to Google Calendar" on mobile and desktop; download the .ics and import into Apple Calendar to confirm description renders and the FreeWhen URL is clickable.

## Out of scope

- Embedding richer HTML in the description (Google `details` is text-only).
- Adding the calendar emoji as a Google Calendar event colour (`color` param uses Google's fixed palette IDs, not arbitrary emoji — separate plan if we want it).
- Including a `LOCATION` field — FreeWhen has no concept of location yet.
- Surfacing the `Preferred` flag (explicitly cut, see Scope decisions).
- Updating the description after finalisation if participants edit notes — once the event is in someone's calendar it's frozen; re-export if you want fresh data.

## Log

- 2026-05-10 22:01 — Created plan
- 2026-05-10 22:32 — Implemented. Reused the existing `FRONTEND_URL` env var
  (already canonical, plumbed through `cfg.FrontendURL`) rather than adding a
  new `FRONTEND_BASE_URL` — keeps the surface area down and avoids two env
  vars meaning the same thing. Handler gets a `WithFrontendURL` builder.
  Both exports now call `GetCalendar` and share a `calendarEventDescription`
  helper (Going / Notes / FreeWhen link). ICS gets RFC 5545 §3.1 line folding
  with UTF-8 boundary safety; `DESCRIPTION` only emitted when non-empty.
  Tests: ics escape + omit + fold + UTF-8-boundary cases; integration tests
  for full description, no-notes branch, and Google `details` query param.
