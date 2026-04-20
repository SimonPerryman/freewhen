# My Calendars

Status: done

## Goal

Give signed-in users a single page that lists every calendar they're part of — whether they created it or joined it — so they stop needing to bookmark or dig through their browser history to find a calendar they've previously used. This is the first v2 feature that makes an authenticated account feel actively valuable rather than just a gate for creation.

It also quietly solves a friction point that has existed since launch: if you joined a calendar on your phone and want to view it from your laptop, today there's no way to find it unless someone re-shares the link. Signed-in users already reconnect to their participant via `user_id` (see `plans/participant-user-linking.md`); this page is the natural UI for that data.

## Scope

**In**
- New page at `/calendars` (authenticated) listing every calendar the signed-in user is a participant of.
- Each row shows: emoji + name, participant count, finalised state (with date if finalised), my role (owner / joiner), and a "last activity" timestamp used for sorting.
- Sort: most-recently-active first.
- Empty state: "You haven't created or joined any calendars yet."
- Header link to the page (shown only when signed in).
- Backend endpoint returning the list in one query, capped server-side at 100 rows with a `has_more` flag.
- A new `calendars.updated_at` column, bumped on every mutation — used as the sort key.
- A new partial index on `participants.user_id` — required for the list query to scale.

**Out**
- Real pagination UX (cursor / "Load more" / infinite scroll). We cap the response at 100 and surface a small "Showing your 100 most recent calendars" footer. See Design Decisions for why this is enough for v1 and how to retrofit cursor pagination cleanly if anyone hits the cap.
- Archive / hide / delete from the list. Calendars are cheap; clutter isn't a real problem yet. Separate tier-3 item ("expiry / auto-archive").
- Filtering or tabs (Owned / Joined / Finalised). A flat list sorted by activity is enough for v1.
- Anonymous "my calendars" via localStorage. Anonymous users are deliberately device-scoped — if we cared, we'd tell them to sign in.
- Search within the list. Defer until someone has enough calendars to care.

## Design Decisions

**One query, one role-agnostic list.** Because `CreateCalendar` auto-joins the owner as a participant linked to their user ID (`plans/participant-user-linking.md:32`), "calendars I'm in" equals "calendars where a participant row has my `user_id`." No UNION with `calendars.owner_id`, no special-casing ownership at query time. We just derive the "owner" badge in the response by comparing `calendars.owner_id` to the requesting user's ID. This keeps the query a single JOIN and means joiners and owners appear in the same list the same way.

**Sort by `calendars.updated_at`, not `created_at`.** A list of "my calendars" is only useful if it surfaces the thing you most recently touched at the top. `created_at` doesn't move, so a calendar you made three months ago that's active today would sink below one you made yesterday and never touched. We're adding an `updated_at TIMESTAMPTZ NOT NULL DEFAULT now()` column on `calendars`, bumped by every service method that mutates calendar-visible state:

- `AddParticipant` (new person joined)
- `DeleteParticipant`
- `SetAvailability` (any participant edits)
- `UpdateCalendarName`, `UpdateCalendarVisibility`
- `SetFinalisedDate`, `ClearFinalisedDate`
- `ClaimParticipant`

This is a small, well-contained piece of state we'll want for other things eventually (sorting in an admin view, cache invalidation, "last active" display on the calendar page), so it's worth having even if the list view is the only caller today. If `live-updates` ships later, its publish points are the same set of mutations and can piggyback on the same bump.

Implementation note: bump in the service layer, same place publishing would happen — single source of truth. If a bump fails, log and continue; it's not load-bearing.

**Derived "my role" and "my last availability" are cheap add-ons, not required for v1.** We can compute `my_role` trivially from `owner_id == me`. We *could* also return "last time I personally marked availability" per calendar to hint "you haven't filled this one in yet" — but that's a second subquery per row and UI we don't need today. Hold it for a later iteration.

**Count participants inline.** Each row shows `N people`. Use `(SELECT COUNT(*) FROM participants WHERE calendar_id = c.id)` in the list query. At current scale (tens of participants, <100 calendars per user) this is fine. If it becomes a hot path, denormalise a `participant_count` onto `calendars` — not today.

**Visibility is irrelevant in the list response.** The list returns my own calendars — I'm already a participant, so the submit-to-view privacy rules don't apply. We can include `visibility` for UI (maybe show a lock icon on submit-to-view calendars) but we don't filter.

**Server cap instead of real pagination (v1).** A user with 1000 calendars would currently get a ~200KB payload and 1000 DOM nodes — tolerable on the wire, noticeably janky on mobile. Real pagination (cursor, "Load more", infinite scroll) is the correct long-term answer but overshoots the problem today when the vast majority of users will have <20 calendars. So v1:

- Server: hard `LIMIT 101` on the query. Return the first 100; if we got 101, drop the last and set `has_more: true`.
- Response: `{ "calendars": [...], "has_more": bool }`.
- Frontend: render all returned rows; when `has_more`, a small muted footer reads "Showing your 100 most recent calendars." No "Load more" button, no cursor handling.

This protects the DOM and payload without committing to a pagination UX before we know whether heavy users would prefer search, filter-by-status, or "load more." The API shape (`has_more`) is forward-compatible: when we do add cursor pagination, we'll accept `?before=<updated_at>_<id>` and return a `next_cursor` alongside the existing fields — no breaking change. A design sketch of the cursor query is in Open Questions so future-me doesn't re-derive it.

**Required index for the list query: `participants(user_id)` partial.** The existing `idx_participants_calendar_user` from migration 007 is `(calendar_id, user_id)` — Postgres can't use it efficiently for `WHERE p.user_id = $1`, which is exactly the filter the list's JOIN runs. Without a dedicated index, every `GET /users/me/calendars` does a sequential-ish scan over participants. Fix in the same migration:

```sql
CREATE INDEX idx_participants_user_id ON participants(user_id) WHERE user_id IS NOT NULL;
```

Partial (`WHERE user_id IS NOT NULL`) because most participants are anonymous and irrelevant to this lookup — keeps the index small and the writes cheap.

**Route path `/calendars` (flat), not `/my-calendars` or `/calendars/mine`.** Cleanest URL. Next.js handles it fine alongside `/calendars/[id]`. Alternative `/calendars/mine` is defensible — keeps namespace organised if we later add `/calendars/shared` or `/calendars/archived` — but those are speculative. Flagged in Open Questions; one-line change if we want it different.

**Authed-only page, hard-gated.** If you're not signed in, `/calendars` redirects to `/login?return_url=/calendars`. No "join a calendar to get started" empty state for anon users; that's what the home page is for.

## Database

Migration `009_add_calendar_updated_at.sql`:

```sql
ALTER TABLE calendars ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- Backfill existing rows so they don't all sort to the bottom as epoch.
UPDATE calendars SET updated_at = COALESCE(finalised_at, created_at);

CREATE INDEX idx_calendars_updated_at ON calendars(updated_at DESC);

CREATE INDEX idx_participants_user_id ON participants(user_id) WHERE user_id IS NOT NULL;
```

Two indexes, one migration (they're both needed specifically for the my-calendars query):

- `idx_calendars_updated_at` supports the `ORDER BY c.updated_at DESC` in the list query. Narrow; we don't need a composite on `(owner_id, updated_at)` because the list filters by `participants.user_id`, not `owner_id`.
- `idx_participants_user_id` supports the `WHERE p.user_id = $1` join filter. Partial because anonymous participants dominate the table and aren't relevant here — keeps the index small and write cost negligible. See Design Decisions for why the existing `(calendar_id, user_id)` index can't be used for this filter.

## API

### New endpoint

`GET /users/me/calendars` — `requireAuth`.

Response:

```json
{
  "calendars": [
    {
      "id": "abc12345",
      "name": "Weekend plans",
      "emoji": "🎉",
      "visibility": "open",
      "created_at": "2026-04-10T12:00:00Z",
      "updated_at": "2026-04-19T18:30:00Z",
      "finalised_date": null,
      "finalised_at": null,
      "participant_count": 5,
      "my_role": "owner"
    },
    {
      "id": "def67890",
      "name": "Team offsite",
      "emoji": "",
      "visibility": "submit_to_view",
      "created_at": "2026-03-01T09:00:00Z",
      "updated_at": "2026-04-15T11:15:00Z",
      "finalised_date": "2026-05-02",
      "finalised_at": "2026-04-15T11:15:00Z",
      "participant_count": 8,
      "my_role": "joiner"
    }
  ],
  "has_more": false
}
```

Sorted by `updated_at DESC`. Capped at 100 rows; `has_more: true` when the user has more than 100 calendars. No cursor/query-params in v1 — see Design Decisions. Empty list returns `{"calendars": [], "has_more": false}`, not 404.

Response shape is **not** `[]model.Calendar` — we wrap in an object so we can add metadata later (`next_cursor`, total count, "you have muted N calendars") without a breaking change.

`my_role` is `"owner"` or `"joiner"`. Enum-style string keeps future roles (`"admin"`) cheap to add.

### No other API changes

All existing endpoints untouched. The `updated_at` field will naturally flow into `GET /calendars/{id}` responses via the existing Calendar model — that's a minor response-shape change but additive.

## Backend Implementation

### Phase 1 — `updated_at` column

1. Migration `009_add_calendar_updated_at.sql` as above.
2. `model.Calendar` gains `UpdatedAt time.Time` with `json:"updated_at"`.
3. `store.getCalendarRow` and `store.CreateCalendar` read/return `updated_at`.
4. Each `UPDATE` on `calendars` sets `updated_at = now()` in the SQL (not via app code) — cheapest to do and can't drift.
5. Mutations that touch adjacent tables (participants, availability) bump the parent calendar's `updated_at` explicitly — see Phase 2.
6. Unit tests: create → assert `updated_at == created_at`; update name → assert `updated_at` bumps; read only → no change.

### Phase 2 — bump hooks for child mutations

In the service layer, after any successful mutation that changes calendar-visible state but doesn't directly `UPDATE calendars`, bump:

- `AddParticipant`, `DeleteParticipant`, `ClaimParticipant`
- `SetAvailability`

Add a `store.TouchCalendar(ctx, calendarID) error` method: `UPDATE calendars SET updated_at = now() WHERE id = $1`. Ignore `ErrNotFound` (race with calendar deletion is not worth thinking about here). Calls are fire-and-log — bump failure must not fail the user's write.

Unit tests cover each mutation path.

### Phase 3 — list handler

1. `store.ListCalendarsForUser(ctx, userID string, limit int) ([]CalendarListItem, error)` where `CalendarListItem` embeds `model.Calendar` + `ParticipantCount int`. Single query:

    ```sql
    SELECT c.id, c.name, c.emoji, c.owner_id, c.visibility, c.created_at, c.updated_at,
           c.finalised_date::text, c.finalised_at,
           (SELECT COUNT(*) FROM participants WHERE calendar_id = c.id) AS participant_count
    FROM calendars c
    JOIN participants p ON p.calendar_id = c.id
    WHERE p.user_id = $1
    ORDER BY c.updated_at DESC, c.id DESC
    LIMIT $2
    ```

    `ORDER BY c.updated_at DESC, c.id DESC` — the `id` tiebreak gives a deterministic order when timestamps collide (important for future cursor pagination). Handler passes `limit = 101` so the service can detect "more than 100 exist" by comparing `len(rows) > 100`.

2. `service.ListUserCalendars(ctx, userID) (items []CalendarListItem, hasMore bool, err error)` — calls the store with `limit = pageSize + 1` (constants: `const pageSize = 100`), trims to `pageSize` and sets `hasMore = true` if the extra row came back. Thin but owns the cap logic so future cursor pagination has a natural home.

3. `handler.ListUserCalendars(w, r)` — reads user from context (guaranteed by `requireAuth`), calls service, maps each row to the response DTO (adds `my_role` by comparing `owner_id == user.ID`), writes JSON with `has_more`.

4. Route: `s.mux.HandleFunc("GET /users/me/calendars", s.requireAuth(s.handler.ListUserCalendars))`.

5. Tests:
    - Store test: create two calendars as user A, one as user B, join A2 as user B; assert user A sees 2, user B sees 2 (A2 and their own), sort order matches `(updated_at DESC, id DESC)`.
    - Store test: with `limit = 3` and 5 matching calendars, exactly 3 rows come back — the service-layer cap logic is tested separately.
    - Service test: with 100 matching calendars, `hasMore = false`; with 101+ matching, `hasMore = true` and returned slice is trimmed to 100.
    - Handler test: 401 without session, 200 with session, empty list for a user with no calendars, `has_more` serialised correctly.
    - Integration test: end-to-end with real Postgres — mirror the store scenarios including the cap.

## Frontend Implementation

### Phase 1 — API client + types

1. Add `listMyCalendars(): Promise<MyCalendarsResponse>` to `src/lib/api.ts`.
2. Type `MyCalendarListItem` (camelCase via existing converter): `id, name, emoji, visibility, createdAt, updatedAt, finalisedDate, finalisedAt, participantCount, myRole`.
3. Type `MyCalendarsResponse { calendars: MyCalendarListItem[]; hasMore: boolean }`.
4. Extend existing `Calendar` type with `updatedAt` (additive, matches backend change).

### Phase 2 — `/calendars` page

New route `src/app/calendars/page.tsx`.

- Server-side behaviour-wise this is a client component; Next.js auth in this app is cookie-based and read via the client `AuthProvider`. So: `"use client"` at the top, use `AuthProvider`, and if unauthenticated redirect to `/login?return_url=/calendars`.
- On mount (when auth context is resolved and authenticated), call `listMyCalendars`.
- Mobile-first layout: single-column card list, `max-w-xl` on desktop.
- Each card links to `/calendars/{id}`.
- Each card shows:
  - Emoji (or a neutral placeholder) + name (bold, truncate on overflow).
  - `N people` sub-label.
  - Role pill: "Owner" (subtle accent) for owners; no pill for joiners (reduces visual noise — most users will see a mix and the pill is only interesting when it's "Owner").
  - Finalised badge: "📅 Sat 2 May" if `finalisedDate` set; otherwise nothing. Fine to format on the client (no timezone issues for a plain date string).
  - Relative time: "Updated 2h ago" bottom-right. Use a small relative-time helper (existing? check `src/lib/` — probably write a tiny `relativeTime.ts` if not).
- Loading: skeleton rows (3). Error: friendly message + retry button (same pattern as `CalendarPageClient`).
- Empty state: illustration-lite ("You haven't created or joined any calendars yet.") with a primary CTA to `/` ("Create a calendar"). Mobile-first, centred.
- When `hasMore` is true, render a small muted footer below the list: "Showing your 100 most recent calendars." No "Load more" button — intentional, see Design Decisions.

New component folder `src/components/MyCalendarsList/` with `MyCalendarsList.tsx` + test. Keep the `page.tsx` a thin orchestrator; logic in the component.

### Phase 3 — header link

In `src/components/Header/Header.tsx`, when `isAuthenticated`, add a "My calendars" link next to the avatar/sign-out button. On mobile, tuck it into whatever menu affordance exists (or the avatar dropdown — check current header). On desktop it's a plain link.

If the header currently has no dropdown, add one only if needed; if the existing pattern is "avatar, name, sign out," simply insert "My calendars" as a link in the same row.

### Phase 4 — Analytics

- `my_calendars_viewed` `{ count }` — once per mount.
- `my_calendars_empty` `{ }` — when the returned list is empty, for measuring how many signed-in users have never created/joined.

## Testing

### Backend
- Store: `ListCalendarsForUser` with 0 / 1 / N calendars, mixed owner + joiner, correct sort, excludes calendars where user isn't a participant.
- Store: `TouchCalendar` bumps `updated_at`; touching a non-existent calendar is a no-op (not an error the caller needs to handle beyond logging).
- Service: each mutation method bumps `updated_at` (verify via before/after read).
- Handler: 401 unauthenticated, 200 authenticated, empty list, `my_role` correctly set for owner vs joiner.
- Integration: create + join flow end-to-end, assert list reflects correctly.

### Frontend
- `MyCalendarsList` renders loading, error, empty, and populated states.
- Role pill shown on owner rows, hidden on joiner rows.
- Finalised badge renders when `finalisedDate` is present.
- `hasMore: true` renders the cap footer; `hasMore: false` does not.
- Clicking a card navigates to `/calendars/{id}` (router mock).
- Unauthenticated visit to `/calendars` redirects (AuthProvider test).

### Manual
- Sign in, create 2 calendars, join a 3rd via link → assert all 3 appear, most-recently-touched first.
- Edit availability on the oldest calendar → assert it jumps to top on refresh.
- Finalise one calendar → assert badge appears and row jumps to top.
- Sign out → assert header link disappears; hitting `/calendars` directly bounces to `/login`.
- Second device: sign in on laptop, view list, open a calendar, edit → refresh list on phone, assert order updated.

## Build Order

1. Backend Phase 1 (migration + `updated_at` column + model + store reads).
2. Backend Phase 2 (`TouchCalendar` + service bumps + tests).
3. Backend Phase 3 (store `ListCalendarsForUser` + service + handler + route + tests).
4. Backend integration tests.
5. Frontend Phase 1 (API client + types).
6. Frontend Phase 2 (`/calendars` page + `MyCalendarsList` component + unit tests).
7. Frontend Phase 3 (header link).
8. Frontend Phase 4 (analytics).
9. Manual QA matrix above.

## Open Questions

- **Route path.** `/calendars` vs `/calendars/mine`. Going with `/calendars` in this plan. Flip is a one-line change in `src/app/` if we prefer the nested form.
- **Relative-time library or homegrown?** We already avoid heavy frontend deps. A 20-line `formatRelative(date)` that returns "just now" / "5m ago" / "2h ago" / "3d ago" / "2w ago" / date-string for older covers it. No `date-fns` unless we find ourselves needing more.
- **Should "my last availability marked" be surfaced per row?** Skipped for v1. Revisit if users report "I can't tell which of my calendars I still owe a response on." Needs a second subquery per row or a small denormalisation.
- **Archiving / muting.** Not in this scope. Separate Tier 3 "expiry / auto-archive" item. If clutter becomes a real problem before that ships, a client-side "hide" (localStorage list of hidden IDs) is a cheap stopgap.
- **Cursor pagination follow-up.** When the first user reports hitting the 100-cap, ship this. Sketch for future-me:
  - Endpoint accepts `?before=<updated_at_iso>_<calendar_id>`.
  - Query: `... AND (c.updated_at, c.id) < ($before_updated_at, $before_id) ORDER BY c.updated_at DESC, c.id DESC LIMIT 101`. Postgres handles the row-value comparison natively; the existing `idx_calendars_updated_at` plus the deterministic `(updated_at, id)` tiebreak gives a stable page boundary.
  - Response gains `next_cursor: "<updated_at_iso>_<id>" | null` (null when `has_more: false`).
  - UI adds a "Load more" button at the footer that calls the endpoint with the last item's cursor and appends to the list. No infinite-scroll — explicit action is simpler and keeps analytics honest.
  - No backwards-compatibility break: `has_more` is already there; `next_cursor` is additive.
- **Showing joined-but-anonymous calendars for newly-signed-in users.** Handled implicitly by the claim flow on calendar-page visit. Not by this list — if a user never revisits an old anon calendar, it will never attach to their account. Acceptable.

## Log

- 2026-04-20 16:30 — Created plan.
- 2026-04-20 16:45 — Added `idx_participants_user_id` partial index to migration 009 (existing `(calendar_id, user_id)` index can't serve `WHERE user_id = $1`). Added server-side `LIMIT 100` cap with `has_more` flag instead of real pagination; cursor-pagination design sketched in Open Questions for the follow-up.
- 2026-04-20 17:00 — Implemented end-to-end. Backend: migration 009 (updated_at + both indexes), `Calendar.UpdatedAt` model field, all existing calendar UPDATE statements bump `updated_at` inline, new `TouchCalendar` store method, service bump hooks on `AddParticipant`/`DeleteParticipant`/`ClaimParticipant`/`SetAvailability`, new `ListCalendarsForUser` store query + `ListUserCalendars` service method (page size = 100, requests 101 to detect has_more), new `GET /users/me/calendars` handler with `my_role` derived inline. Frontend: `listMyCalendars` API client, `MyCalendarListItem`/`MyCalendarsResponse` types, `/calendars` page (client component, redirects anon → login), `MyCalendarsList` component with loading/error/empty/ready states + cap footer + owner pill + finalised badge, `relativeTime.ts` helper, header "My calendars" link when signed in, analytics (`my_calendars_viewed`, `my_calendars_empty`). Unit + integration tests added; all green.
