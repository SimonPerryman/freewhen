# Calendar Integrations

Status: done

## Goal

Let signed-in users (a) **import** their Google Calendar busy times into a FreeWhen calendar so they don't re-enter what their own calendar already knows, and (b) **export** the finalised date back into their calendar once the group has picked a day. Close the loop at both ends: calendar-in → FreeWhen → calendar-out.

Tier 3 in `docs/v2-priorities.md` — high value, high complexity. This plan scopes it down to the smallest slice that delivers both ends without building a synchronisation engine.

## Scope

**In (v1)**
- **Import** free/busy from Google Calendar for a chosen date range. User reviews and accepts before anything is written to their participant row.
- **Export** the finalised date as:
  - `.ics` file download (universal — Apple Calendar, Outlook, Google).
  - "Add to Google Calendar" deep link (`calendar.google.com/render?action=TEMPLATE`) — no OAuth, no write scope, opens Google's own prefilled form.
- Signed-in participants only. Anonymous joiners see neither feature.
- Apple Calendar / Outlook / Fastmail covered passively via `.ics`.

**Out (deferred)**
- **Two-way sync.** No webhooks, no polling, no continuous mirror. An import is a one-shot snapshot; re-importing is allowed but replaces nothing silently.
- **Writing events to Google via API.** The deep link avoids asking for `calendar.events` write scope and keeps the permissions story simple.
- **Apple/CalDAV import.** CalDAV auth is fiddly (app-specific passwords, no OAuth). `.ics` one-off import URLs don't exist in Apple's UI. Revisit when anyone asks.
- **Microsoft / Outlook import.** Graph API is a separate integration; same shape as Google, reuse the code path when there's demand.
- **Storing refresh tokens.** Access tokens live only in memory for the length of the import request, then are discarded. This restricts privacy blast radius and dodges the token-rotation problem.
- **Event details.** We query FreeBusy only — we never see titles, attendees, or locations. This is the central privacy promise and it shapes the design.

## Design Decisions

**Import renders as inline suggestions on the real calendar grid, not a separate preview page.** The flow is: user clicks "Import from Google" → OAuth round-trip → we compute a per-day free/busy vector → land back on the calendar page with the grid now showing four cell states (empty / suggested-free / suggested-busy / confirmed). Each tap on a suggestion commits that one cell through the existing availability endpoint — same gesture as hand-tapping a blank day. Suggestions that are never tapped never become real availability. Rationale: a preview page is a different screen with different rules the user has to learn; inline is "just more cells, rendered differently" and collapses import into the app's existing single gesture. See `plans/v2/t3/mocks/calendar-integrations-cell-states.html` for the visual language and `plans/v2/t3/mocks/calendar-integrations-flow.html` for the end-to-end flow (entry → consent → mid-import → expired edge → export).

**Suggestions are client-only, ephemeral, and never persisted.** The backend delivers the vector once (via a short-lived `HttpOnly` cookie, consumed by a single endpoint call on page load) and forgets it. Refresh the page after an import and the un-tapped suggestions are gone — you'd have to re-run the import. This is deliberate: suggestions are guidance, not data. No `availability` row exists until the user taps.

**Four cell states, not three.** Empty · suggested-free (dashed accent border + tint + corner dot) · confirmed-free (solid accent fill) · suggested-busy (diagonal hatch + strikethrough + event-block marker, still tappable as an override). Suggested-busy is the non-obvious one: Google said you had a meeting, but tapping it says "actually, I'm free that day". Keeps the happy path single-gesture in both directions.

**Import-mode chrome: a progress chip and a key strip, both ephemeral.** While suggestions exist, the calendar header shows a pulsing chip (`Google · 6 of 8 left`) and a one-line key below it (`◌ Tap to confirm · ▨ Tap to override · ● Confirmed`). As the user taps through, the counter ticks down; when it hits zero, both chip and key fade out and the calendar returns to its normal chrome. This gives mobile users the legend they need (no hover) without permanently accreting UI.

**"Accept all suggestions" as a secondary affordance.** For users who trust the import and don't want to tap fourteen cells, a small "Accept all" action in the chip commits every outstanding `suggested-free` day in one round-trip (ignores `suggested-busy` — overrides must be deliberate). This is the old Apply button, demoted from "the only path" to "the shortcut".

**"Free" heuristic: a day is free if it has any contiguous free block of ≥ 2 hours during 09:00–21:00 local time.** Configurable later; fixed for v1. Pure "no busy blocks at all" is too strict (most people have at least a dentist booking somewhere). "Any free minute" is too loose. 2h/working-evening window matches the mental model of "I could do something that day".

**Timezone: use the user's browser timezone, passed as a query param.** The backend doesn't own a timezone concept yet. Sending the IANA tz string (`Europe/London`) on the import request keeps the server stateless about it. Store nothing; each import is self-contained.

**Incremental OAuth scope, not upfront.** Sign-in stays `openid email profile`. The Calendar scope (`https://www.googleapis.com/auth/calendar.freebusy`) is requested only on the first import, via a separate OAuth round-trip with `include_granted_scopes=true`. Rationale: most users never import; asking for Calendar access at sign-up tanks conversion and looks creepy.

**No refresh token persistence.** Use `access_type=online` (not `offline`), do the FreeBusy call inside the callback handler while the token is still fresh, discard. If the user imports again next week, they'll round-trip Google again — silent if they haven't revoked consent (no consent screen re-prompt because the scope is already granted), noisy otherwise. We trade one extra redirect for zero stored secrets.

**FreeBusy response is not persisted raw.** The backend computes the per-day boolean vector in-memory, sends it to the frontend as JSON, and forgets it. The raw `{start, end}` busy intervals never hit the database. This is the privacy commitment — even a full DB dump reveals nothing about a user's external calendar beyond what they chose to paste into FreeWhen.

**Export uses `.ics` + Google deep link, never Calendar API writes.** Two reasons: (1) writing events needs `calendar.events` scope, which is a big ask for a one-tap feature; (2) the Google render URL achieves the same UX (user sees the event in *their* Calendar, can adjust before saving) without any token. `.ics` covers everyone else. This is the deliberate "cheap version" — users who expected auto-insertion will tell us and we'll build it then.

**Export is available to anyone, signed in or not, once finalised.** Even anonymous joiners deserve the "add this to my calendar" payoff. The export endpoint keys off calendar state (`finalised_date IS NOT NULL`), not auth.

**Import is a single-calendar operation.** No "import across all my FreeWhen calendars" batch. Each calendar gets its own import action. Cross-calendar batching is a retention feature, not an MVP feature.

**Rate limit import at the user level.** 10 imports/hour/user. FreeBusy is cheap but we don't want a compromised account to be a scraping tool. No request-throttling middleware exists today (`internal/limits/` is constants + input validation, not request rate limiting), so add a small in-memory token bucket in a new `internal/ratelimit/` package keyed on `user_id`. Lives in-process — fine for single-instance Cloud Run; revisit if we ever scale horizontally.

**Suggestion cookie discovery: always-fetch, tolerate 404.** No client-side cookie reading exists in the frontend today (the auth session is HttpOnly and sent automatically; per-calendar access tokens use localStorage, not cookies). Adding a non-HttpOnly marker cookie just to gate one fetch introduces a new pattern for one feature. Cheaper to unconditionally call `GET /calendars/{id}/import/suggestions` once on calendar mount and treat 404 as "no pending import".

**Primary calendar only in v1.** Hardcode `"primary"`. A picker means inserting a second screen into the OAuth round-trip ("we got your calendars, now pick which ones"), which breaks the single-gesture promise the inline-suggestions UX is built around. Mitigation: connect-flow copy says "reads your primary Google Calendar" so multi-calendar users self-identify and ask. Revisit when anyone does.

**All-day events count as busy; rely on the override state.** A flight or holiday reading as "busy all day" in FreeBusy correctly rejects the day ~90% of the time. The 10% case (an all-day "Birthday: Mum" reminder) is exactly what `suggested-busy → tap to override` was designed for. No special-casing in `ComputeFreeDays`.

**Skip past days server-side.** `ComputeFreeDays` filters out any date before "today in the user's tz" before returning. Suggesting "yesterday, free" has no meaningful UX. If every day in the range is past, the existing "no suggestions" path handles it.

**`.ics` SUMMARY uses the calendar's own title.** Rather than a hardcoded English string like "FreeWhen event", the export uses `{calendar.title}` — the user named it, use their name for it. Sidesteps i18n entirely. Users can rename inside their calendar app in two taps if they want something different.

**Privacy interstitial: shown once per user, before the first OAuth.** A FreeWhen-voice screen between "Import from Google" and Google's consent screen. Communicates the trust commitments (free/busy only, never event details, nothing stored) in our voice — Google's own consent string is terse and users have been trained to skim it. Tracked via `users.calendar_import_consent_ack_at TIMESTAMPTZ`; null → show, set → skip straight to OAuth. Subsequent imports become near-silent (Google won't re-prompt either because scope is already granted) which is the payoff for sitting through it once. Not shown every time — that erodes trust by implying "we keep wanting you to agree". Not never — first import is a friction moment regardless, use it for the trust win. The ack also serves as our informed-consent record (logged as `{event:"calendar_consent_given", user_id, timestamp}`, no PII).

## API Changes

### New endpoints

| Method | Path | Auth | Description |
|---|---|---|---|
| `POST` | `/auth/google/calendar/consent-ack` | required | Records that the user has seen the privacy interstitial. Sets `users.calendar_import_consent_ack_at = now()`. Idempotent. Called by the interstitial's "Continue" button right before the redirect to `/auth/google/calendar/connect`. |
| `GET` | `/auth/google/calendar/connect` | required | Start incremental OAuth: redirect to Google with `calendar.freebusy` scope and `include_granted_scopes=true`. Accepts `?calendar_id=&return_url=` in state. Caller is expected to have ack'd consent first (frontend reads `me.calendar_import_consent_ack_at` to decide whether to show the interstitial; backend does not gate). |
| `GET` | `/auth/google/calendar/callback` | required | Exchange code, fetch FreeBusy for `[calendar.start_date, calendar.end_date]`, compute per-day vector, write it to a short-lived `HttpOnly` cookie (scoped to the calendar ID, 5-min expiry), redirect straight to `/calendars/{id}`. Vector never hits the DB, never sits in a URL. |
| `GET` | `/calendars/{id}/import/suggestions` | required | Reads the suggestion cookie, returns `{ free: ["2026-04-21", ...], busy: ["2026-04-23", ...] }`, and clears the cookie in the same response. Called once by the calendar page on mount if the cookie is present. Returns 404 if no cookie. |
| `POST` | `/calendars/{id}/availability/import` | required | "Accept all suggestions" shortcut. Body: `{ days: ["2026-04-21", ...] }`. Adds those days to the caller's availability (does not replace — a non-destructive merge). Individual taps go through the existing availability endpoint; this one only exists so the chip's "Accept all" action is one round-trip instead of N. |
| `GET` | `/calendars/{id}/export.ics` | none | 200 `text/calendar` if `finalised_date` is set, 404 otherwise. |
| `GET` | `/calendars/{id}/export/google` | none | 302 to `https://calendar.google.com/calendar/render?action=TEMPLATE&...` pre-filled from calendar title + finalised date. |

### Data model

**One new column, no new tables.** Migration `016_add_calendar_import_consent_ack.sql` adds `users.calendar_import_consent_ack_at TIMESTAMPTZ NULL`. Used to gate the interstitial: null = show on next import, set = skip straight to OAuth. Doubles as our informed-consent record.

The import result itself is just existing `availability` rows. The FreeBusy vector is ephemeral.

Optional migration `017_add_availability_source.sql` — adds `availability.source TEXT NOT NULL DEFAULT 'manual'` with values `'manual' | 'import:google'`. Purpose: observability and a future "re-run my import" button that only clears imported rows, not hand-edited ones. **Defer to v1.1** if it adds friction — the feature works without it.

## Backend Implementation

### Phase 1: Export (ship first, it's almost free)
1. **`internal/ics/ics.go`** — a 40-line `.ics` generator. `VCALENDAR` + single `VEVENT` with `DTSTART;VALUE=DATE`, `DTEND;VALUE=DATE` (next day), `SUMMARY`, `UID`, `DTSTAMP`. No deps — hand-rolled text is simpler than pulling in a library for one event type.
2. **`internal/handler/export.go`** — `ExportICS` and `ExportGoogle`. Both: load calendar, 404 if not finalised or not found, otherwise build output. The Google URL just uses `url.Values` and a fixed base URL.
3. **Route registration** in `internal/server/server.go`. Both endpoints public.
4. **Integration tests** — finalised-calendar happy path, not-finalised 404, unknown calendar 404.

### Phase 2: Import
1. **Config additions** — reuse existing `GOOGLE_CLIENT_ID`/`GOOGLE_CLIENT_SECRET`. Add `GOOGLE_CALENDAR_REDIRECT_URL` (= `https://api.<domain>/auth/google/calendar/callback`).
2. **`internal/google/freebusy.go`** — thin wrapper over `POST https://www.googleapis.com/calendar/v3/freeBusy`. Input: access token, time range, primary calendar id `"primary"`. Output: `[]TimeRange` (local to the function). No dep on the full `google.golang.org/api/calendar/v3` client — one HTTP call, one struct.
3. **`internal/availability/freeday.go`** — pure function `ComputeFreeDays(busy []TimeRange, tz string, start, end civil.Date) []civil.Date` applying the 2h/09–21 rule. Pure = easy to unit test with table-driven cases (no meetings, all-day block, 30-min block mid-afternoon, overnight travel, DST-crossing day).
4. **`internal/handler/calendar_integration.go`** — OAuth connect/callback + suggestion-consumption handlers, mirroring the sign-in flow in `auth.go:45-55` but with the calendar scope. Callback: exchange → FreeBusy → ComputeFreeDays → write `import_suggestions_{calendar_id}` cookie (JSON payload with `free` + `busy` arrays, `HttpOnly`, `Secure`, `SameSite=Lax`, 5-min expiry) → redirect to `FRONTEND_URL/calendars/{id}`. Separate `GET /calendars/{id}/import/suggestions` handler: reads the cookie, returns the JSON, clears the cookie via `Set-Cookie` with `Max-Age=0`. Keeps the vector single-use.
   Also add `POST /auth/google/calendar/consent-ack` — sets `users.calendar_import_consent_ack_at = now()`, idempotent, returns 204. Logs `{event:"calendar_consent_given", user_id, timestamp}`.
5. **`internal/handler/availability.go`** — add `ImportAvailability` handler for the "Accept all" shortcut. Reads body, validates dates fall inside calendar range, verifies participant membership, upserts as a non-destructive merge (never deletes existing rows). Reuses existing availability store methods. Individual cell taps continue to use the existing per-cell availability endpoint — this handler is purely the batch fast-path.
6. **Rate limiter** — simple per-user token bucket middleware on the import endpoints.
7. **Unit tests** — `ComputeFreeDays` cases, ICS generation bytes, Google URL construction. **Integration tests** — end-to-end import with a mocked FreeBusy response; export happy path.

### Phase 3 (deferred): availability source tracking
Migration `016_add_availability_source.sql`. Plumb through store + handler. Exposed in the UI as a small "imported" pill on relevant days, with a "re-import" button that only touches imported rows.

## Frontend Implementation

### Phase 1: Export
1. **`ExportButtons` component** — appears on the finalised banner (see `plans/v2/t1/finalise-a-date.md`). Two buttons: "Add to Google Calendar" (opens `/calendars/{id}/export/google` in new tab) and "Download .ics". Mobile-first; stack vertically on narrow screens.
2. **No state, no tests beyond the component render.**

### Phase 2: Import (inline suggestions)
1. **Entry point** — on the calendar page, a small "Import from Google" action in the toolbar. Only visible if `isAuthenticated`. No gating on `googleLinked` (see Phase 2.1 note below).
2. **Connect flow** — click checks the user's `calendar_import_consent_ack_at`. If null, render the privacy interstitial (FreeWhen-voice screen: what we see / what we keep / what changes for the group, plus the "tapping a suggestion makes it part of your availability" caveat). On Continue: `POST /auth/google/calendar/consent-ack`, then redirect to `/auth/google/calendar/connect?calendar_id=X&return_url=/calendars/X`. If `consent_ack_at` is set, skip the interstitial and redirect immediately. After the OAuth round-trip, user lands back on the calendar page — no new route.
3. **Suggestion hydration** — the calendar page mounts, checks for the `import_suggestions_{id}` cookie's presence (a separate non-HttpOnly marker cookie set alongside, or just always attempt the fetch and tolerate 404), then calls `GET /calendars/{id}/import/suggestions` exactly once. Response gets merged into local React state as `suggestions = { free: Set<date>, busy: Set<date> }`. The server clears the cookie in the response.
4. **Cell rendering** — `CalendarGrid` cell logic extends from two states (empty / confirmed) to four. Computed per cell: if the user has a confirmed availability row → `confirmed`; else if date is in `suggestions.free` → `suggested-free`; else if date is in `suggestions.busy` → `suggested-busy`; else `empty`. Visual styles per `plans/v2/t3/mocks/calendar-integrations-cell-states.html`.
5. **Tap handlers** — tap on `suggested-free` or `suggested-busy` removes the date from its suggestion set AND fires the existing availability endpoint to mark the day free. Tap on `confirmed` behaves as today (un-mark). Tap on `empty` behaves as today (mark free).
6. **Import chrome** — while `suggestions.free.size + suggestions.busy.size > 0`: render the pulsing chip (`Google · N of M left`) and the one-line key strip above the grid. When the count hits zero, unmount both. Counter is derived — no separate state to keep in sync.
7. **"Accept all" shortcut** — a small "Accept all" button inside the chip. On click: `POST /calendars/{id}/availability/import` with `days = Array.from(suggestions.free)`, clear `suggestions.free` on success. `suggested-busy` entries are left alone — overrides must stay deliberate.
8. **Empty / edge states** — no suggestions returned (chip + key never render), suggestions expired before user returned (same outcome — chip + key never render, user sees a toast "Import expired, try again"), not yet a participant (toolbar entry point hidden until they join).
9. **Tests** — cell state resolution (all four paths), tap removes from suggestions + calls availability endpoint, chip counter ticks down, chrome unmounts at zero, "Accept all" clears only free suggestions.

### Phase 2.1: magic-link users
Already handled by not gating the entry point on `googleLinked` — the OAuth round-trip works the same way regardless of how the user originally signed in. Keep an eye on this in testing; if magic-link users hit friction (e.g. Google refusing to link a second email), add a one-line hint in the connect flow.

## Privacy, Legal & Permissions

The design itself is genuinely low-risk — no token storage, no event-detail access, no persisted FreeBusy data. But "low-risk design" is not the same as "no compliance work". This section enumerates both the technical commitments and the paperwork trail.

### Technical commitments (enforced by the design)

- **Scope requested:** `https://www.googleapis.com/auth/calendar.freebusy` only. We will *never* ask for `calendar.readonly` or `calendar.events` as part of this feature.
- **Data retention:** raw FreeBusy response is held in memory during the callback request and in the ephemeral suggestions cookie (5 min, single-use). Neither hits the database. The only persisted artifact is the per-day availability rows the user explicitly taps to confirm.
- **Token retention:** `access_type=online`, no refresh token requested or stored. Access token discarded as soon as the FreeBusy call returns.
- **No PII in logs** (per AGENTS.md): log `{event:"calendar_import", user_id, calendar_id, days_imported}` and `{event:"calendar_consent_given", user_id, timestamp}`. Never log email, tokens, busy intervals, or event counts that could correlate back to calendar contents.
- **Revocation:** users can revoke FreeWhen's Calendar access at `myaccount.google.com/permissions`. We do not need a custom revoke endpoint because we don't store tokens. **Caveat:** revoking Google access does not unmark days the user has already confirmed — once tapped, those become normal availability rows owned by the user inside FreeWhen, the same as any hand-tapped row. The interstitial copy should make this clear.

### GDPR / UK GDPR

- **Lawful basis: consent (Art. 6(1)(a)).** Captured by the privacy interstitial's "Continue" action; recorded as `users.calendar_import_consent_ack_at` plus a `calendar_consent_given` log event. Sufficient as informed-consent evidence.
- **Article 13 information duties.** Met partly by the interstitial (purpose, what's processed, what's kept) and partly by the privacy policy (controller identity, legal basis, retention specifics, recipients, rights, complaint route). The interstitial is the user-visible summary; the privacy policy is the authoritative record.
- **Data minimisation (Art. 5(1)(c))** and **storage limitation (Art. 5(1)(e))** are both satisfied by the design — we collect only free/busy and store nothing.
- **Right to withdraw consent.** Two paths: revoke at `myaccount.google.com/permissions` (cuts off future imports) and delete the FreeWhen account (removes the consent ack and any availability rows). No custom withdrawal endpoint needed.
- **Records of processing (Art. 30).** Add a line to the ROPA when one's needed: "Google Calendar free/busy data, processed transiently for availability suggestions, not stored, lawful basis = consent."
- **DPIA.** Considered; not required. Reasoning: minimal processing, no special category data, no profiling, no large-scale automated decision-making, ephemeral handling. A two-line internal note documenting this consideration is enough.
- **Data residency.** Currently Cloud Run runs in [region TBC during launch]; if EU/UK users will be processed in a US region, either run the import handler in an EU region or rely on the EU→US Data Privacy Framework / UK Extension. **Verify before launch** — see checklist.

### Google API Services User Data Policy

Separate from GDPR; governs every OAuth integration regardless of jurisdiction.

- **Scope justification on the OAuth consent screen** must accurately describe how we use `calendar.freebusy`. Mismatch can lose us the integration on audit.
- **In-product privacy policy link** is required on the Google consent screen config and within the app where the OAuth is initiated (the interstitial covers this).
- **No selling, transferring, or using the data for ads.** Trivially satisfied — we don't store it.
- **`calendar.freebusy` is classified as non-sensitive**, so no app verification review is required. This may change; check current Google policy at launch time.

### Other regions

- **CCPA/CPRA (California):** the design satisfies "right to know" (privacy policy lists the processing) and "right to delete" (account deletion clears the consent ack). No "sale" of data — exempt from opt-out requirements.
- **Children:** Google blocks under-13 accounts at the OAuth layer, which is our practical defence. No additional gating in v1; revisit if FreeWhen ever targets school/youth groups.

### Disclaimers worth surfacing (where)

- *Interstitial:* "Tapping a suggestion makes it part of your availability — revoking Google access later won't undo it."
- *Privacy policy:* full Article 13/14 content, plus that we facilitate but don't control the user's relationship with Google.
- *Export buttons:* no banner — the deep-link sends only the calendar title and date back to Google, which is data the user is explicitly choosing to add to their own calendar. Adding a disclaimer here would be performative.

## Infrastructure

- **Google Cloud Console:** add `calendar.freebusy` as an allowed scope on the OAuth consent screen. If the app is in "Testing", this just works; if it's "In production", `calendar.freebusy` is a **non-sensitive** scope (per Google's classification) so no verification review is needed. Double-check current status before starting Phase 2.
- **DNS / Cloud Run:** no changes. The callback URL sits on the existing `api.` subdomain.
- **Secrets:** no new secrets — `GOOGLE_CLIENT_ID`/`SECRET` are reused. Add `GOOGLE_CALENDAR_REDIRECT_URL` as a regular env var.

## Build Order

1. Phase 1 (export) end-to-end — tiny, independently shippable, unblocks the finalise UX straight away.
2. Google Cloud Console scope config + local dev OAuth test.
3. Phase 2 backend (FreeBusy fetch + ComputeFreeDays + import endpoint) with unit tests.
4. Phase 2 frontend (connect flow, preview page, apply) with tests.
5. Ship Phase 2 behind a per-user rollout (just toggle the entry-point visibility on a flag if we want to canary).
6. Phase 2.1 — relax the `googleLinked` gate once the core flow is stable.
7. Phase 3 — availability source tracking (optional).

## Launch Checklist

Compliance / config items that must be verified before Phase 2 ships. None are design decisions — they're paperwork.

- [ ] **Privacy policy** updated with: Google Calendar free/busy processing, lawful basis (consent), retention (none beyond the request), recipients (none beyond Google), withdrawal route (Google permissions page + account delete), data residency.
- [ ] **Terms of service** updated to note we facilitate but don't control the user's relationship with Google.
- [ ] **Google Cloud Console OAuth consent screen** — scope justification text accurately describes free/busy use; in-product privacy policy URL set; app status confirmed (`calendar.freebusy` is non-sensitive at time of writing — verify still true).
- [ ] **Cloud Run region check** — confirm where the `calendar/callback` handler runs, and whether EU/UK user data hits a US region. If yes, document reliance on the EU→US Data Privacy Framework / UK Extension; otherwise move the handler to an EU region.
- [ ] **ROPA entry** added (when one's needed): "Google Calendar free/busy data, processed transiently for availability suggestions, not stored, lawful basis = consent."
- [ ] **DPIA note** filed internally: considered, not required (rationale documented).
- [ ] **`docs/infra-notes.md`** updated with revocation instructions (Google permissions page + the "won't undo confirmed days" caveat).
- [ ] **Interstitial copy** reviewed by whoever owns legal sign-off — it doubles as the Article 13 summary.

## Open Questions

_None outstanding — all design and engineering questions resolved into Design Decisions. Launch-time compliance items tracked in the checklist above._

## Log

- 2026-04-21 — Created plan.
- 2026-04-21 — Reworked import UX from a separate review-and-apply preview page to inline suggestions on the calendar grid itself. Four cell states (empty / suggested-free / suggested-busy / confirmed), ephemeral import chrome (pulsing progress chip + one-line key strip), single-use `HttpOnly` suggestions cookie, individual taps flow through the existing availability endpoint, "Accept all" kept as a shortcut. Visual language mocked in `plans/v2/t3/mocks/calendar-integrations-cell-states.html`.
- 2026-04-22 — Resolved four design open questions into Design Decisions: primary-calendar-only in v1 (no picker), all-day events count as busy and rely on the override state, past days filtered server-side in `ComputeFreeDays`, `.ics` SUMMARY uses `{calendar.title}` to sidestep i18n. Remaining open questions are engineering follow-ups.
- 2026-04-22 — Resolved engineering open questions: confirmed no rate-limit middleware exists (`internal/limits/` is input validation, not throttling) so a new `internal/ratelimit/` token-bucket package is needed; confirmed no client-side cookie reading exists on the frontend (per-calendar tokens use localStorage), so suggestion hydration goes with always-fetch-and-tolerate-404 rather than a marker cookie. Open Questions section now empty.
- 2026-04-22 — Added `plans/v2/t3/mocks/calendar-integrations-flow.html` — end-to-end flow mock (entry point in settings sheet, Google consent screen, mid-import with Accept-all chip, expired-import toast, finalised banner with export buttons). Companion to the existing cell-states mock; shares the same design tokens.
- 2026-04-22 — Added privacy interstitial as a Design Decision (shown once per user before first OAuth, gated by new `users.calendar_import_consent_ack_at` column). Expanded Privacy & Permissions into a full Privacy/Legal/Permissions section covering GDPR, Google API Services User Data Policy, CCPA, and the technical commitments. Added Launch Checklist for compliance/config items (privacy policy, ToS, Cloud Run region check, ROPA, DPIA note, OAuth consent screen config). New `POST /auth/google/calendar/consent-ack` endpoint and corresponding migration. Flow mock updated: layout from 5-col to 3x2 grid, interstitial inserted as Step 2 with the trust copy + caveat about confirmed days persisting after revocation, subsequent steps renumbered.
- 2026-05-10 — Bugfix: `FinalisedBanner` row layout broke on desktop. The banner had a `@media (min-width: 640px)` rule switching to `flex-direction: row` so contents/export-buttons/unfinalise sat side-by-side, but the calendar page caps its container at 540px (`app/calendars/[id]/page.module.css`). Inside that ~500px column the row layout crushed the contents flex item to a few px wide, wrapping the participant name and note text one word/character per line and visually overlapping the export buttons. Fixed by removing the row-layout media query so the banner always stacks vertically (header → notes → export buttons → unfinalise). Container queries would be the more general fix but vertical stacking is the right shape for this card anyway.
- 2026-05-02 — Verified implementation against plan: all backend packages (`ics`, `availability`, `google`, `ratelimit`), handlers (`export.go`, `calendar_integration.go`), migration `018`, and 7 routes in `server.go` present; frontend `ExportButtons` and `ImportFromGoogle` components present with `CalendarGrid` extended for suggestion states; `make test` green (Go all packages, 219/219 frontend). Marking status done.
- 2026-04-23 — Implemented both phases end-to-end. Backend: `internal/ics/` ICS generator; `internal/handler/export.go` with `ExportICS` + `ExportGoogle`; `internal/availability/freeday.go` pure `ComputeFreeDays`; `internal/google/freebusy.go` minimal FreeBusy client; `internal/ratelimit/` token bucket (10/hr per user); `internal/handler/calendar_integration.go` with consent-ack, connect, callback, suggestions handlers; `ImportAvailability` handler for the Accept-all shortcut; migration `018_add_calendar_import_consent_ack.sql`; `MergeAvailabilityDates` store method (ON CONFLICT DO NOTHING) for non-destructive merge; new service method + CalendarService interface entry. Routes registered in `internal/server/server.go`. Frontend: `ExportButtons` component wired into `FinalisedBanner`; `useImportFromGoogle` hook covering entry button, privacy interstitial (single-shot, gated by `calendarImportConsentAckAt`), one-shot suggestion hydration with 404 tolerance, progress chip + key strip chrome, Accept-all shortcut; `CalendarGrid` extended with optional `suggestedFreeDates` / `suggestedBusyDates` props (dashed accent border + hatched styles); tap handler wrapped to clear suggestion state. Import window hardcoded to today → today+90d (plan referenced non-existent `calendar.start_date`/`end_date` columns). Deviations noted: no explicit integration tests for the OAuth callback flow; availability source-tracking (Phase 3) deferred per plan. `make lint` and `make test` green (219 frontend tests, all Go packages).
