# Notifications & Reminders

Status: paused

## Goal

Close the "did anyone respond?" loop. Today, after a creator shares a calendar, the only way to know whether people have filled it in is to keep tapping back. Notifications give signed-in owners and participants a timely nudge when something worth knowing happens — a new response, the calendar being finalised, or a stale calendar waiting on replies — without spamming inboxes.

Tier 2 of `docs/v2-priorities.md` ("Notifications/reminders") with the explicit caveat that this only serves signed-in users. Anonymous participants get nothing.

## Scope

**In**
- Email notifications via Resend (already wired for magic links).
- Two audiences with distinct defaults:
  - **Owner** — notified about activity on calendars they own (digested).
  - **Signed-in participant** — notified when the outcome of a calendar they're in changes (finalise / unfinalise / owner edits).
- Event-driven and scheduled triggers.
- A durable per-user preferences table with one-click unsubscribe from every email.
- Digest / debounce logic to prevent email floods.
- In-app "inbox" on `/my-calendars` showing recent events per calendar (same event source, different renderer).

**Out (deferred)**
- Web Push / PWA push notifications. Needs service-worker + VAPID keys + permission UX. Separate plan.
- SMS / native push. Not in platform scope.
- Per-calendar mute (vs per-user × per-kind). Add if complaints appear.
- Timezone-aware quiet hours. v1 uses UTC quiet hours; per-user TZ is a follow-up.
- Notifying anonymous participants (no channel exists).

## Design Decisions

**Email first, in-app inbox same-pass.** Email is the only durable, universally-reachable channel available today, and Resend is already wired. The in-app inbox falls out almost free because every notification is persisted in an outbox table — rendering the same rows as `/my-calendars` entries costs one extra endpoint and a dot indicator. Doing both together avoids building the event model twice.

**One outbox, many renderers.** Every notifiable event produces a row in `notification_outbox`. Email is one consumer (cron worker drains rows where `channel = 'email'`); the in-app inbox is another (handler queries the same table filtered to `channel = 'inapp'`). One-at-a-time per kind: an event that fans out to both channels becomes two rows. Keeps the schema dumb.

**Digest vs per-event is a per-kind decision, not global.**

| Kind | Mode | Reason |
|---|---|---|
| `owner.new_response` | Digest (2h window) | Chatty; 8 participants marking at once = 1 summary email, not 8 |
| `owner.all_responded` | Per-event | Terminal state; owner wants to know immediately |
| `owner.stale_calendar` | Scheduled (3 days after create, max 2 total) | Not event-driven at all |
| `participant.finalised` | Per-event | Rare, important, time-sensitive |
| `participant.unfinalised` | Per-event | Rare, important |
| `participant.calendar_edited` | Digest (24h window) | Cover rename/emoji/visibility changes without flooding |

**Digests are implemented by delay + coalesce, not by a cron scanning outboxes.** When a digestable event fires, we insert a row with `send_after = now + window` and `status = pending`. The worker only picks rows where `send_after <= now`. On insert, if a pending digest row already exists for the same `(user_id, kind, calendar_id)` tuple, we bump its payload (increment the counter, append the name) and leave `send_after` alone. Result: first event sets the clock, subsequent events piggy-back, worker ships one email at the end of the window.

**Realtime hub is the dispatch point.** The existing `realtime.Hub.Publish` already fires after every relevant mutation (`participant.joined`, `calendar.finalised`, etc.). We layer a `NotificationDispatcher` that subscribes to the hub — on each event, look up "who cares about this?" and insert outbox rows. This reuses the fan-out plumbing and means a future WebSocket swap inherits notifications for free.

Subtlety: hub events today carry only `{type, calendar_id}`. The dispatcher needs at least `actor_user_id` (the participant/user whose action triggered it) to avoid notifying someone about their own action. Extend `realtime.Event` with an optional `ActorUserID string` field. Existing transports serialise it or ignore it; it's informational to the SSE payload too (useful for future "you just did X" suppression on the client).

**Scheduled triggers via a separate cron-style job.** Cloud Run Jobs (or a dead-simple goroutine ticker inside the API process for v1) runs every 15 minutes and:

1. Scans outbox rows where `send_after <= now AND status = pending`, ships them, marks `sent`.
2. Scans calendars older than 3 days with <100% response rate and no `owner.stale_calendar` sent in the last 72h, inserts a fresh outbox row.

Both queries are cheap with the right indexes. Starting in-process; move to Cloud Run Jobs once horizontal scaling matters (same pre-condition as the SSE hub → LISTEN/NOTIFY migration).

**Unsubscribe is stateless via signed tokens.** Every email footer includes `List-Unsubscribe: <mailto:...>, <https://freewhen/unsubscribe?t=...>`. The token is an HMAC of `{user_id, kind, exp}` signed with a server secret. One-click GET flips the matching `notification_preferences` row to `false` and renders a confirmation page. No DB write on email send — token is derived, not stored.

Also set the `List-Unsubscribe-Post: List-Unsubscribe=One-Click` header so Gmail/Apple Mail render the built-in unsubscribe button. Resend passes these through.

**Preferences default to opt-in for everything.** Signed-in users who don't touch settings get sensible defaults (all kinds enabled). A single unsubscribe click disables one kind. The `/settings/notifications` page lets them re-enable or granularly tune. Rationale: the signal is a reason they signed in — hiding it behind "opt-in for updates" sabotages the feature.

**Quiet hours: UTC 22:00 → 07:00 for v1.** If a send would fire in that window, shift `send_after` forward to 07:00 UTC the same/next day. Good-enough for a UK-centric product; revisit when user base spans more timezones. Applies to `owner.new_response` digest and `owner.stale_calendar` only — per-event kinds (`finalised`, etc.) ignore quiet hours because they're time-sensitive.

**No PII in logs.** Ship counts (`event=notification_sent kind=owner.new_response calendar_id=... user_id=...`), never email addresses. `user_id` is a UUID, not PII on its own. Matches the `make lint-pii` rule in AGENTS.md.

## Data Model

New tables — all embedded migrations in `internal/store/migrations/`.

```sql
-- One row per user × kind. Default-on; absent row = enabled.
CREATE TABLE notification_preferences (
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind       TEXT        NOT NULL,
    enabled    BOOLEAN     NOT NULL DEFAULT TRUE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, kind)
);

-- Outbox: every notification lands here, workers drain it.
CREATE TABLE notification_outbox (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    calendar_id UUID        NOT NULL REFERENCES calendars(id) ON DELETE CASCADE,
    kind        TEXT        NOT NULL,
    channel     TEXT        NOT NULL,  -- 'email' | 'inapp'
    payload     JSONB       NOT NULL DEFAULT '{}'::jsonb,
    send_after  TIMESTAMPTZ NOT NULL DEFAULT now(),
    status      TEXT        NOT NULL DEFAULT 'pending', -- 'pending' | 'sent' | 'failed' | 'skipped'
    attempts    INT         NOT NULL DEFAULT 0,
    sent_at     TIMESTAMPTZ,
    read_at     TIMESTAMPTZ, -- inapp only; NULL = unread
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_outbox_pending ON notification_outbox (send_after) WHERE status = 'pending';
CREATE INDEX idx_outbox_inapp ON notification_outbox (user_id, created_at DESC) WHERE channel = 'inapp';
-- Coalescing lookup for digest rows.
CREATE UNIQUE INDEX idx_outbox_digest_open
  ON notification_outbox (user_id, kind, calendar_id, channel)
  WHERE status = 'pending';

-- Track the last stale-calendar nudge per (calendar, user) so the scheduler doesn't re-fire.
CREATE TABLE notification_stale_sent (
    calendar_id UUID NOT NULL REFERENCES calendars(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    sent_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    count       INT NOT NULL DEFAULT 1,
    PRIMARY KEY (calendar_id, user_id)
);
```

Notes:
- `idx_outbox_digest_open` is the key piece — enforces "at most one open digest row per `(user, kind, calendar)`". Coalesce logic becomes: `INSERT … ON CONFLICT (user_id, kind, calendar_id, channel) WHERE status = 'pending' DO UPDATE SET payload = <merge>`. (Postgres partial unique indexes do support `ON CONFLICT` when the predicate is specified — verify with a migration test.)
- Payload shape per kind documented in a Go comment alongside the `NotificationKind` constants; not typed at the SQL layer to keep iteration cheap.

## API

**New endpoints**

- `GET /notifications` — list the current user's in-app notifications. Returns `{items: [...], unread_count: n}`. Paginated via `?cursor=<id>&limit=20`. Auth required.
- `POST /notifications/mark-read` — body `{ids: ["..."]}` or `{all: true}`. Marks `read_at = now()`.
- `GET /settings/notifications` — returns merged defaults + user's preference rows: `{preferences: [{kind, enabled}, ...]}`.
- `PUT /settings/notifications` — body `{preferences: [{kind, enabled}]}`. Upserts.
- `GET /unsubscribe?t=<token>` — public (no auth). Validates HMAC, disables the matching preference, renders a "You're unsubscribed from X" page with a one-click re-enable button.

**No changes** to calendar / participant / availability endpoints.

## Backend Implementation

### Phase 1 — Schema & model

- Migrations for the three tables above.
- `model.NotificationKind` enum (string consts): `OwnerNewResponse`, `OwnerAllResponded`, `OwnerStaleCalendar`, `ParticipantFinalised`, `ParticipantUnfinalised`, `ParticipantCalendarEdited`.
- `model.NotificationChannel`: `email`, `inapp`.
- `store.Notifications` with the usual CRUD: `InsertOrCoalesce`, `ClaimPending(limit)` (for the worker — `FOR UPDATE SKIP LOCKED`), `MarkSent`, `MarkFailed`, `ListForUser`, `UnreadCount`, `MarkRead`, `GetPreferences`, `UpsertPreferences`, `StaleCalendarCandidates(since)`, `RecordStaleSent`.

### Phase 2 — Dispatcher

New package `internal/notifications/`:

```go
type Dispatcher struct { store Store; hub *realtime.Hub; clock func() time.Time }
func (d *Dispatcher) Start(ctx context.Context) // subscribes to hub, loops
func (d *Dispatcher) dispatch(ctx context.Context, e realtime.Event) error
```

Dispatch rules — one switch on `e.Type`:

- `participant.joined` / `availability.changed` → if actor != owner, insert/coalesce `OwnerNewResponse` for the owner (email + inapp). Check "all responded" after the insert: if response count == participant count, additionally insert `OwnerAllResponded`.
- `calendar.finalised` → insert `ParticipantFinalised` for every signed-in participant except the owner.
- `calendar.unfinalised` → insert `ParticipantUnfinalised` ditto.
- `calendar.updated` → insert/coalesce `ParticipantCalendarEdited`.
- `participant.removed`, `participant.claimed` → no notification (v1 scope).

Each insert checks preferences first; if disabled for that kind, skip with `status='skipped'` so the row stays for audit but never ships. Better than silently dropping.

### Phase 3 — Worker

`internal/notifications/worker.go`:

```go
func (w *Worker) Run(ctx context.Context, tick time.Duration)
```

Every `tick` (default 1 minute in-process, can crank lower in tests):

1. `ClaimPending(batchSize=50)` — returns pending rows with `send_after <= now()` locked in a transaction.
2. For each row, dispatch to the channel handler (`email.Send(...)` or no-op for inapp — inapp is "sent" as soon as the row exists; we mark it sent on insert rather than going through the worker).
3. Mark sent / failed. Exponential backoff on failure: `send_after = now() + 2^attempts minutes`, cap at 30 minutes, `status='failed'` after 5 attempts.

Second pass on the same tick:

4. `StaleCalendarCandidates(createdBefore=now-3d, lastNudgedBefore=now-72h, maxCount=2)` → for each, insert `OwnerStaleCalendar` outbox row.

Worker runs as a goroutine off `server.New`. Shutdown via context cancel.

### Phase 4 — Email templates

`internal/email/`:

- New method per kind: `SendOwnerDigest(to, calendar, responders)`, `SendAllResponded(...)`, `SendStaleCalendar(...)`, `SendFinalised(...)`, etc.
- Shared footer renderer that injects the unsubscribe link + `List-Unsubscribe` headers. Factor template rendering (`html/template`) so copy lives in `.tmpl` files under `internal/email/templates/`, not inline in Go strings.
- One integration test per template: render with a fixture, assert the unsubscribe URL, assert key copy, snapshot the HTML.

### Phase 5 — Handlers

- `internal/handler/notifications.go` — `List`, `MarkRead`.
- `internal/handler/settings.go` (new) — `GetPreferences`, `UpdatePreferences`.
- `internal/handler/unsubscribe.go` — public GET, HTML response. Reuses `magiclink`-style page template.

### Phase 6 — Observability

- Counters on `/admin/metrics`: `notifications_pending`, `notifications_sent_total`, `notifications_failed_total`, `notifications_skipped_total` (by kind via label).
- Structured logs: `event=notification_dispatched kind=... calendar_id=... user_id=...`, `event=notification_sent ...`, `event=notification_failed ... error=...`. No PII (no email address, no calendar name — calendar name could be user-typed and potentially sensitive).

## Frontend Implementation

### Phase 1 — In-app inbox

- `src/components/NotificationBell/NotificationBell.tsx` — bell icon with unread badge, lives in the top nav on signed-in pages. Polls `GET /notifications?limit=5` every 60s (or hooks the SSE stream if we extend it to fire a `notifications.updated` event — nice but not required).
- Clicking opens a dropdown list with one-line summaries. Each row links to the calendar.
- "Mark all read" button at the bottom.

### Phase 2 — Preferences page

- `src/app/settings/notifications/page.tsx`. Checklist, one per kind, grouped by audience (Owner / Participant). Saves on change (debounced PUT).

### Phase 3 — Unsubscribe page

- `src/app/unsubscribe/page.tsx` (note: unsubscribe renders server-side via the backend, but we also want a frontend route for the re-enable flow). Reads `?t=` from the URL, confirms.

### Phase 4 — Analytics

- `notification_opened` `{kind}` on email click-through (tracked via UTM or a redirect endpoint — `/n/<id>` marks read + 302s to the calendar).
- `notification_unsubscribed` `{kind}`.
- `notification_preferences_changed` `{kind, enabled}`.

## Testing

### Backend

- Store tests: coalescing (two consecutive inserts update one row, not two), `ClaimPending` honours `send_after`, `StaleCalendarCandidates` excludes finalised / fully-responded / recently-nudged calendars.
- Dispatcher tests: recording hub, fire each event type, assert exactly the expected outbox rows appear. Assert actor-is-owner suppresses the owner notification.
- Worker tests: fake clock, fake email sender, assert retry backoff, assert failed rows eventually mark `status=failed` after 5 attempts.
- Template tests: render each email template, assert unsubscribe URL validates against the HMAC verifier.
- Integration test: spin up full server with a stub email sender (captures calls), drive through a realistic flow (create calendar, three participants mark dates, assert owner receives exactly one digest email after the 4h window; advance fake clock to test).

### Frontend

- Bell component: unread count renders, mark-read hides badge.
- Preferences page: toggle persists, reloading restores state.
- Unsubscribe page: confirmation + re-enable round-trip.

### Manual

- Send yourself (real email) one of each kind in staging. Check Gmail / Apple Mail render the built-in Unsubscribe button.
- Verify quiet-hours shift: create a calendar at 23:00 UTC, mark from another browser, confirm the digest arrives after 07:00, not in the middle of the night.
- Kill the worker mid-flight; restart; assert pending rows eventually drain (nothing lost, no duplicate sends).

## Build Order

1. Phase 1 — migrations + store layer + model constants.
2. Phase 2 — dispatcher (no worker yet — rows just pile up, easy to inspect).
3. Phase 3 — worker with real email send.
4. Phase 4 — templates + unsubscribe tokens.
5. Phase 5 — handlers (list, prefs, unsubscribe).
6. Phase 6 — metrics + logs.
7. Frontend inbox + preferences + unsubscribe page.
8. Analytics.
9. Manual QA.
10. Roll out with **only `ParticipantFinalised` enabled by default**; watch for a week; enable `OwnerNewResponse` digest; enable `OwnerStaleCalendar` last. Gives time to catch any email-deliverability or coalescing bugs before the noisier kinds ship.

## Resolved Questions

All resolved with the same meta-principle: **ship the obvious default, instrument, revisit.** Don't let these delay Phase 1.

- **Digest window for `OwnerNewResponse`:** Start at **2h**, not 4h. Most groups fill a calendar in one sitting so the first digest still coalesces, and timeliness matters more than saving one email. Log coalesce counts per digest and tune from real data after a week.
- **`ParticipantCalendarEdited`:** Ship the kind, but **default-off** at launch (opt-in via preferences). Rename/emoji churn is noise; visibility changes are rare. Splitting into separate kinds later is a one-commit follow-up since the dispatcher already switches on `e.Type` — don't pre-split.
- **SSE piggy-back for inbox freshness:** Skip for v1. 60s poll is fine, and the calendar-page SSE stream is scoped per-calendar — piggy-backing bleeds concerns. Build a dedicated `GET /notifications/events` stream only if/when a user complains the bell is stale.
- **Transactional vs marketing sender domain:** Same domain for v1. None of these are marketing — they're user-triggered or user-pending-action. Revisit only if the stale-calendar nudge (the most marketing-shaped kind) dents deliverability.
- **Rate-limit on `/unsubscribe`:** Don't build upfront. HMAC + 90-day expiry means worst-case an attacker with a valid tuple unsubscribes someone (annoying, reversible in one click). Do emit a structured log line on every unsubscribe so abuse is visible; add a per-IP limit only if logs show spray attempts.

## Log

- 2026-04-20 21:50 — Created plan.
- 2026-04-21 — Resolved all five open questions; reduced digest window 4h → 2h; defaulted `ParticipantCalendarEdited` to off at launch.
- 2026-04-21 — Paused. Plan is complete and ready to implement, but deferred in favour of lighter Tier 3 QoL items. Revisit when a clear week is available.
