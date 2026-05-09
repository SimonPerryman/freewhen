# Notifications MVP — Decision Email + Optional Email Capture

Status: in-progress

## Goal

Close the "did the date get picked?" loop without spamming inboxes or building the full notifications stack. Three thin emails:

1. **Participant finalised email** — sent to participants (other than the owner) who chose to give us an email when a calendar is finalised.
2. **Owner finalised confirmation** — sent to the owner when they finalise their own calendar. Same template family, different copy ("You picked X — here are the export links to forward").
3. **Owner-triggered nudge** — manual "Send reminder" button on the owner's view, only emails participants who left an email, with a copy-link affordance for the rest.

Plus the join-time **optional email capture**: single field at name entry, with one-line helper text. No follow-up nag, no separate prompt.

This is deliberately a fraction of the scope of the existing paused plan in `plans/v2/t2/notifications.md`. That plan stays as the eventual destination (digests, in-app inbox, scheduled stale-calendar nudges, preferences page); this plan ships the minimum that delivers user value and validates the channel.

## Scope

**In**
- Optional `email` column on `participants`.
- Optional email field at join (anon flow). Signed-in joiners reuse their account email automatically.
- Three transactional emails: participant-finalised, owner-finalised-confirmation, owner-triggered-nudge.
- `List-Unsubscribe` and `List-Unsubscribe-Post: List-Unsubscribe=One-Click` headers on every email (no visible unsubscribe link in body).
- Lightweight unsubscribe endpoint that nulls the participant's email on click. Owner unsubscribe = "set my account to email me only for nudges I send" deferred — owners don't get a List-Unsubscribe link on their own confirmation (it's user-triggered, not opt-in).

**Out (deferred to the bigger plan)**
- Calendar-deleted email. Tempting, but adds a fourth email type — see "Graduation tripwire" below.
- Digesting / coalescing.
- In-app inbox / bell.
- Owner notifications on each new response (chatty without digesting).
- Auto stale-calendar reminders (cron / scheduled).
- Preferences page with per-kind toggles.
- Per-user `notification_preferences` table — overkill for three transactional emails.
- Quiet hours.
- Persisted `sent_emails` / outbox table — Resend's dashboard is the source of truth for delivery / bounce status until we want admin UI on it.

## Graduation Tripwire

When **any** of the following becomes true, stop extending this plan and unpause `plans/v2/t2/notifications.md` instead — the bigger plan already designs for them:

- We want a fourth email type (calendar-deleted, calendar-renamed, participant-removed, etc.).
- A user asks for "I want X but not Y" — the moment per-kind preferences are a real question, the preferences table is the right answer.
- We need retries / programmatic bounce handling on top of what Resend's dashboard surfaces.
- We want an in-app surface for "what emails has FreeWhen sent me?".

This is the line. Don't grow this plan past three email types — that's how MVPs become the bigger thing they were meant to defer.

## Design Decisions

**No double opt-in, no consent ticker.** The email is transactional — they typed it into a "tell me when the date's picked" field. Consent is the field itself. Skip checkboxes; they add noise and don't add legal coverage we lack.

**No visible unsubscribe link, but `List-Unsubscribe` header always.** Transactional emails (CAN-SPAM, PECR, GDPR) don't legally need a body unsubscribe link. The header is invisible, costs nothing, surfaces a native Unsubscribe button in Gmail/Apple Mail, and protects sender reputation. Header points at a one-click GET that nulls `participants.email` for that participant id (HMAC-signed token, no auth).

**Helper text, not a separate privacy disclosure.** Field copy is the reassurance:
> Email (optional) — Only used to tell you when a date's picked.
The "only" implicitly rules out marketing without saying "we won't spam you" (which makes people suspicious). One line, no link, no extra row.

**Email lives on `participants`, not a new table.** Anon participants already exist as participant rows; a nullable `email TEXT` column is the smallest possible change. Signed-in participants don't need it stored on the row — we resolve via `participants.user_id → users.email` at send time. (Storing twice would mean re-syncing on email change.)

**Owner nudge is manual-only for v1.** Auto-nudges are tempting but unbounded — every cron firing risks spam, requires quiet hours, requires per-calendar suppression. A button is bounded by definition: the owner pressed it.

**The "copy link for the rest" affordance solves the anon-participant gap honestly.** Owner's nudge UI shows: "Emailed 3 of 7 participants. Copy a link to share with the others ↓". Acknowledges the asymmetry rather than hiding it.

**No PII in logs.** `event=notification_sent kind=finalised participant_id=... calendar_id=...`. Never log the email address. Matches the `make lint-pii` rule.

## Data Model

One migration:

```sql
-- 024_participant_email.sql
ALTER TABLE participants ADD COLUMN email TEXT;
-- No index needed — we never look up participants by email.
-- No unique constraint — same person can join multiple calendars with same email.
```

That's it. No new tables. No preferences. No outbox.

## API Changes

**Modified**
- `POST /calendars/{id}/participants` (join) — accepts optional `email` in body. Trim + lowercase at the boundary (matches the existing magic-link normalisation in migration 023). Reject obviously-invalid shapes (`strings.Contains(@)` is enough for v1 — Resend will bounce the rest, which is fine).
- `PATCH /participants/{id}` (edit name) — also accepts `email` so a user can add/change/remove it after joining.

**New**
- `POST /calendars/{id}/nudge` — owner-only. Server-side: send the reminder email to every participant on the calendar with `email IS NOT NULL`. Returns `{emailed: n, skipped: m}` so the UI can show the "copy link for the rest" affordance. Rate-limit: max one call per 6h per calendar (single `last_nudge_at` column on `calendars`, or just a memory map keyed by calendar id — DB column is honest).
- `GET /unsubscribe?t=<token>` — public. HMAC of `{participant_id, exp}` signed with server secret. Validates, sets `participants.email = NULL`, renders a tiny "You're unsubscribed" HTML page. No re-enable flow (they can edit their participant from the calendar).

## Email Content

Three templates, in `backend/internal/email/templates/`:

**`finalised.tmpl`**
```
Subject: 📅 {Calendar name} — date picked: {date}

Hi {name},

{Owner name} picked {formatted date} for {Calendar name}.

  Add to your calendar: {GCal link}
  Download .ics:        {.ics link}
  View calendar:        {calendar URL}

— FreeWhen
```

**`finalised_owner.tmpl`** — sent to the owner when they finalise their own calendar.
```
Subject: ✅ {Calendar name} — you picked {date}

Hi {Owner name},

You picked {formatted date} for {Calendar name}.

Forward these so everyone has it on their calendar:

  Add to Google Calendar: {GCal link}
  Download .ics:          {.ics link}
  View calendar:          {calendar URL}

— FreeWhen
```

The owner's confirmation does not include `List-Unsubscribe`. They didn't opt in to anything — they pressed Finalise. If they don't want it, they can stop finalising calendars (or we add a per-account toggle when the bigger plan lands).

**`nudge.tmpl`**
```
Subject: 📅 Reminder: {Calendar name} is waiting on you

Hi {name},

{Owner name} is still waiting on availability for {Calendar name}.

  Mark your availability: {calendar URL}

— FreeWhen
```

Both render through a shared layout that injects:
- `List-Unsubscribe: <{base}/unsubscribe?t={token}>`
- `List-Unsubscribe-Post: List-Unsubscribe=One-Click`
- A footer line: "You got this because you joined {Calendar name} on FreeWhen."

No visible unsubscribe link in body (header-only).

## Frontend Changes

### Join form (`JoinSheet` or wherever participant creation lives)

Add one optional input below the name field:

```
Email (optional)
[___________________________]
Only used to tell you when a date's picked.
```

- Helper text uses the existing muted-text style; one line, never wraps on 375px.
- Field is hidden entirely for signed-in users (we already have their email — no need to re-ask).
- `inputMode="email"`, `autocomplete="email"`, `type="email"`.

### Edit-participant sheet

Same field, prefilled. Empty submission clears it (and is the in-app way to unsubscribe).

### Owner view — "Send reminder" button

- Lives near the existing "Who's missing?" panel on the owner's calendar page.
- Disabled if `last_nudge_at` was within 6h, with tooltip "You can send another reminder in Xh".
- After click, toast: "Emailed 3 of 7 participants. Copy a link for the others ↓" — the "↓" expands a row showing the calendar invite URL with a copy button. Reuses the existing `ShareButton` clipboard flow.

### Unsubscribe page

`src/app/unsubscribe/page.tsx` — server-rendered or static, doesn't matter. Reads `?t=` from URL, calls `GET /unsubscribe?t=...`, shows the result. Single page, no auth.

## Backend Implementation

### Phase 1 — Schema & participant email
- Migration 024 adds `participants.email`.
- `model.Participant` gets `Email *string`.
- `store.Participants.Create` and `Update` accept email; normalise (`strings.TrimSpace` + `strings.ToLower`) at the boundary.
- Update existing handlers and tests to accept and round-trip the field.

### Phase 2 — Finalised emails (participant + owner)
- Hook into the existing `calendar.finalised` flow (wherever `service.FinaliseCalendar` lives) — after successful commit, fan out emails in a background goroutine (it's a one-shot, low-volume event; no worker needed yet).
- Two send paths from one trigger:
  - `email.SendFinalised(ctx, to, payload)` for every participant with `email IS NOT NULL` whose `participant.user_id != owner.user_id`.
  - `email.SendFinalisedOwner(ctx, to, payload)` for the owner exactly once.
- Suppression rule: a participant whose email matches the owner's account email is treated as "owner" and gets the owner template, not both. Avoids sending two emails to the same address.
- Log `notification_sent` / `notification_failed` per recipient (no PII). On failure, log + drop — no retry. The user can re-finalise / re-unfinalise to re-send if it really matters; we'll add retry when the bigger plan lands.

### Phase 3 — Nudge endpoint
- Handler validates owner, checks `last_nudge_at`, queries participants with email, fans out via the same email package.
- Migration 025 adds `calendars.last_nudge_at TIMESTAMPTZ NULL`.

### Phase 4 — Unsubscribe
- HMAC helper in `internal/notifications/token.go` (new tiny package): `Sign(participantID) string`, `Verify(token) (participantID, error)`. Server secret read from config (`NOTIFICATIONS_HMAC_SECRET` env var; AGENTS.md "config over environment detection").
- Public handler `GET /unsubscribe`. Renders an HTML page (server-side template) — kept on the API so it works even if the frontend is down.

### Phase 5 — Logs & metrics
- Counters on `/admin/metrics`: `notifications_sent_total{kind}`, `notifications_failed_total{kind}`, `nudges_total`, `unsubscribes_total`.
- Lint-pii sanity: no `email` field anywhere in structured logs.

## Testing

**Backend**
- Migration 024 is idempotent and round-trips email through Create / Get / Update.
- Finalise flow: stub mailer, finalise a calendar with a mix of email/no-email participants, assert exactly the right addresses got called.
- Nudge endpoint: rate limit is enforced (second call within 6h returns 429); only owner can trigger; counts in response are correct.
- Unsubscribe: valid token clears email; expired token returns 400; tampered token returns 400; clicking unsubscribe twice is idempotent.

**Frontend**
- Join form: email field renders for anon flow, hidden for signed-in flow, helper text present and on one line at 375px.
- Edit sheet: email prefilled, blank submission clears it.
- Send-reminder button disabled state and copy-link affordance.

**Manual**
- Send yourself a real finalised email in staging. Check Gmail and Apple Mail render the native Unsubscribe button.
- Click it — confirm `email` is now NULL in the DB and the unsubscribe page shows.
- Inspect logs on `make lint-pii` and after a real send — confirm no email addresses anywhere.

## Build Order

1. Phase 1 — migration + model + handler plumbing (no email sending yet — verifiable by joining and seeing the field round-trip).
2. Phase 4 — unsubscribe token + endpoint + page (build before we send anything so every email we ship has a working unsubscribe from day one).
3. Phase 2 — finalised email. Roll out behind the existing PostHog feature flag plumbing as `notifications-finalised-email`. Default off; flip on for ourselves first; flip on for everyone once one round-trip looks right.
4. Phase 3 — nudge endpoint + button. Same flag-gated rollout: `notifications-owner-nudge`.
5. Phase 5 — metrics + log assertions.
6. Manual QA in staging.

## Resolved Questions

- **Owner gets an email when they finalise their own calendar?** Yes — separate `finalised_owner.tmpl` with copy aimed at "forward these export links". No `List-Unsubscribe` header on it (user-triggered, not opt-in).
- **Calendar deleted while a finalised email is in flight?** Goroutine sends with whatever payload it captured pre-delete; the URLs in the email will 404. Acceptable for v1.
- **Calendar-deleted email to participants?** No. Adding it crosses the graduation tripwire (forces a preferences UI). Defer to the bigger plan.
- **Bounce handling.** v1 just logs and moves on. Resend's dashboard surfaces delivery + bounce status per send. We don't persist sends to our DB at all in v1 — when we do, that's the bigger plan's outbox table.

## Log

- 2026-05-09 — Created plan. Scoped down from the bigger paused `notifications.md` to two transactional emails + optional email capture, behind feature flags. Bigger plan stays as the eventual destination once this validates the channel.
- 2026-05-09 — Added owner finalise-confirmation email (3rd email type). Resolved open questions on bounces, owner emails, and calendar-deletion. Added explicit "graduation tripwire" section listing the conditions that should trigger unpausing the bigger plan instead of extending this one.
- 2026-05-09 15:15 — End-to-end implementation landed (backend + frontend, all 5 phases). Migrations 024 (`participants.email`) and 025 (`calendars.last_nudge_at`); new `internal/notifications` package (HMAC `TokenSigner`, `Service` fanning out finalised + nudge sends, `Counters` snapshotter wired into `/admin/metrics`); `Sender.SendTransactional` adds optional `List-Unsubscribe` headers; service `WithNotifications`/`WithBaseURL` hooks the finalise flow into a fire-and-forget goroutine. New routes `GET /unsubscribe` (public, HTML) and `POST /calendars/{id}/nudge` (owner-only, 6h cooldown). Frontend: optional email field on `JoinForm` (hidden for signed-in joiners), `nudgeCalendar` API + Send-reminder button on `WhosMissingPanel`, `/unsubscribe` page that calls the API. Privacy: `GetCalendar` strips participant emails from the response so they never leak to other viewers — slight UX regression on the edit-prefill plan, noted as acceptable for v1. Flag-gating per the build order is **not yet wired** (the rollout flags `notifications-finalised-email` / `notifications-owner-nudge` aren't checked); for now the channel is gated by whether `NOTIFICATIONS_HMAC_SECRET` is set in the environment. Backend `make lint` and `make test-integration` pass; frontend vitest passes (231/231).
