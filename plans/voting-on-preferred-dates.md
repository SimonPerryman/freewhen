# Voting on Preferred Dates

Status: done

## Goal

Let a participant signal which of their available dates they'd *prefer* — "I can do any of these, but my top picks are these two." Today the grid is binary: you're either free or not. That works for "find any date that works," but not for "find the date most people actually want." When three dates have everyone available, the organiser flips a coin. Preferences break the tie with real signal.

From `docs/v2-priorities.md` (Tier 3):
> Voting on preferred dates — "I'd prefer this date." Adds signal beyond availability.

## Context

- Availability rows live in `availability(participant_id, calendar_id, date, note)` — composite key, bulk-replaced per write (`backend/internal/store/migrations/001_create_tables.sql`, `011_add_availability_note.sql`). The notes feature already added one nullable column and a matching PUT extension, so we have a template for doing the same.
- `PUT /calendars/{id}/availability` takes `{participant_id, dates[], notes?}` with "absent `notes` = don't touch" semantics (`plans/comments-on-dates.md` §3). Voting should follow the same contract.
- `GET /calendars/{id}` returns `availability[]` with `participant_id / calendar_id / date / note?`. Adding a boolean is a pure additive change.
- SSE carries only a refresh-pointer (`{"calendar_id":"..."}`), so nothing on the wire changes (`backend/internal/handler/events.go:143`).
- Frontend state: `CalendarPageClient` tracks `selectedDates: Set<string>` and `notesByDate: Map<string,string>`, debounced via `queueSave` / `pendingNotesRef` (`frontend/src/app/calendars/[id]/CalendarPageClient.tsx`). Preferences fit the same pattern.
- `DayDetailSheet` already exists (from notes) and opens on tap of your own free cell. It's the natural home for the vote toggle — no new affordance needed.
- `BestDay` currently picks by free-count with a "clearly ahead of second place" rule (`frontend/src/components/BestDay/BestDay.tsx:37`). Preferences should feed in as a tiebreaker.
- Finalised calendars are read-only (availability + notes). Voting must follow the same rule.
- Privacy: existing visibility modes (open / submit_to_view) apply to availability. Votes inherit — if you can see who's free, you can see who prefers.

## Design Decisions

### 1. Scope — **votes attach to (participant, date) pairs the participant has marked available**

Same shape as notes. Rationale:
- One extra nullable/boolean column on `availability`, no new table, no orphaned rows.
- Semantically coherent: a vote is "of the dates I'm free, this is my pick." Voting for a date you can't attend is nonsense.
- Bulk-replace semantics handle cleanup for free: unselect a date → row deleted → vote gone.
- Rejected: a separate `vote(participant_id, calendar_id, date)` table. More plumbing, same expressive power, extra sync rule ("delete votes for dates no longer in availability").

### 2. Unlimited votes, binary per date (not ranked, no vote budget)

Each participant can mark any subset of their available dates as preferred. No limit, no ranking.

- **Why no limit:** a "top 3" cap sounds like it improves signal but in practice forces people to agonise, skip voting, or game the cap. The goal is "reveal which dates actually matter most," not to force a ranking.
- **Why binary, not numeric (stars / 1-5):** numeric ranking is a different product. Binary is the minimum viable preference signal and maps to a single checkbox in the sheet. If demand for ranking appears, upgrade to integers later without breaking the boolean read path (a migration + API version bump).
- **Why no "required vote" mode:** participation is already voluntary for availability; adding a required vote mode means a participant who genuinely has no preference is forced to fake one. That corrupts the signal.

### 3. Backend — **extend `PUT /availability` with `preferred: string[]`, absent = don't touch**

Request body grows to:
```json
{
  "participant_id": "...",
  "dates": ["2026-04-20", "2026-04-21", "2026-04-22"],
  "notes": {"2026-04-20": "only after 6pm"},
  "preferred": ["2026-04-21", "2026-04-22"]
}
```

Semantics — mirror the notes rule exactly:
- `preferred` **absent or `null` → don't touch any existing vote rows.** Protects against a stale cached client silently clearing everyone's votes on their next availability edit.
- `preferred: []` **explicitly clears all votes** for this participant on this calendar (from among the dates in `dates`).
- Any date in `preferred` that isn't in `dates` → 400 (can't vote for a date you're not available on).
- Duplicates in `preferred` are deduped server-side, not rejected — clients may legitimately send a set-as-array.
- A date in `dates` but not in `preferred` (when `preferred` is present) → that date's `preferred` becomes `false`. This is the explicit "I unvoted this date" path.
- Shape chosen as `string[]` (not `map[string]bool`) because the value is binary and a set is the natural wire form. Matches how `dates` is already serialised.

### 4. Schema — **add `preferred BOOLEAN NOT NULL DEFAULT FALSE` to `availability`**

```sql
ALTER TABLE availability ADD COLUMN preferred BOOLEAN NOT NULL DEFAULT FALSE;
```

- `NOT NULL DEFAULT FALSE` — a row either exists (available) with a boolean vote, or it doesn't exist at all. Null would just encode "unvoted" which is identical to `false`; avoid the tri-state trap.
- No check constraint needed — type-enforced.
- Backfill is implicit: existing rows become `FALSE`, which is correct (nobody has voted yet).
- Index decision: **no index.** Query path is always "fetch all availability for calendar X" (already indexed by `calendar_id`); we never filter by `preferred` at the DB layer.

### 5. Read shape — **boolean rides on the existing `availability[]` entry**

`GET /calendars/{id}` response gains an optional `preferred` field:
```json
{"participant_id": "...", "calendar_id": "...", "date": "2026-04-20", "note": "...", "preferred": true}
```

Go model: `Preferred bool \`json:"preferred,omitempty"\`` — `omitempty` suppresses the default-false case so unchanged rows don't bloat the payload. TS type: `preferred?: boolean`. Clients treat missing as `false`.

### 6. SSE — **no payload change**

Existing refresh-pointer design handles it. Publish `EventAvailabilityChanged` on vote edits; clients refetch.

**Write cadence:** vote toggle debounced through the same `queueSave` path as notes/dates. A rapid-toggle in the sheet collapses to one PUT. Same 500 ms debounce.

### 7. UI — **checkbox in `DayDetailSheet`, star indicator on the cell**

The day sheet already opens on tap-of-own-free-cell (from the notes feature). It's the right home.

**Sheet changes:**
- Add a labeled checkbox at the top of the "your details" section: **"Prefer this date"** with a short helper line: *"Mark your top picks — helps the group pick a date everyone actually wants."*
- Toggling calls a new `onToggleVote(date, preferred)` prop → `CalendarPageClient.handleToggleVote` → updates `votesByDate` and calls `queueSave` with `preferred: [...currently-preferred dates]` (full set, not a diff — simpler reasoning, small payload).
- Sibling read list: in the "Who's free" section, append a ★ glyph next to participants who preferred this date. `aria-label="Sarah prefers this date"`.
- Read-only when calendar is finalised (checkbox hidden, ★ glyphs still shown).

**Cell indicator:** a small outline star in the bottom-right of the cell when any participant has voted on that date.
- Colour: `var(--text-secondary)` (neutral); count badge if > 1 vote — `★3`. Rejected colouring by participant because (a) the top-right corner is already a coloured dot for notes, and (b) votes are an *aggregate* signal, not a per-person badge.
- Size mirrors the note dot (≈ 10px glyph) so cells stay uncluttered at 375px.
- `.voteBadge` sits in the bottom-right; `.noteDot` stays in the top-right. No collision.
- Not rendered on past cells (same rule as the note dot). Rendered on finalised cells (consistency with notes).
- Zero-vote cells: no indicator at all, so the grid stays quiet when the feature isn't being used.

**Explicitly rejected alternatives:**
- **Second tap toggles vote** (tap once = free, tap twice = preferred). Overloads the tap gesture already doing dual duty (toggle-free / open-sheet), and gives no affordance for un-voting without un-selecting.
- **Dedicated "Vote" toolbar mode.** Fourth mode alongside Range / Weekends / Weekdays — too many modes and doesn't map to "reveal who's picking what" reading.
- **Heart / thumbs-up glyphs.** Star reads as "pick" or "rank" universally; hearts imply liking which is a different semantic.

### 8. `BestDay` — **preferences as tiebreaker, not primary metric**

The current rule is "show the top free-count day only if it's clearly ahead of second place." Extend it:

1. If exactly one date has everyone free → show it (unchanged).
2. If multiple dates tie at the top free-count:
   - Pick the one with the most preferred-votes.
   - If votes also tie → show no best day (current "not clearly ahead" behaviour).
3. Label the winner differently when votes drove the call: `"Everyone's free — 4 prefer this"` or `"3 of 5 free — 2 prefer this"`. Subtitle text only; card layout unchanged.

**Why not weight votes into the primary score:** availability is a hard constraint (can they come?) and preference is a soft signal (would they rather?). Letting a preference override availability — "Bob prefers Thursday so let's pick it even though Alice can't come" — defeats the point of the tool. Preferences only tie-break among already-available-winning dates.

### 9. Finalised banner — **"N prefer this date" line**

If the finalised date has ≥ 1 vote, the banner adds a line beneath the existing note list: `"4 of 5 preferred this date"`. If zero votes, line omitted entirely. No per-name list here — the day sheet already surfaces individual stars.

### 10. Privacy

- Votes are visible to every viewer of the calendar. Same boundary as availability and notes.
- Submit-to-view: votes follow availability — you see neither until you've submitted your own availability.

## Plan

Ordered to ship backend + API first, then wire the frontend, same pattern as the notes feature.

1. **Migration** — add `013_add_availability_preferred.sql` with the `ALTER TABLE ... ADD COLUMN preferred BOOLEAN NOT NULL DEFAULT FALSE`. Embedded via `embed.FS`, picked up by `store.Migrate()` on boot.
2. **Model** — add `Preferred bool \`json:"preferred,omitempty"\`` to `model.Availability` (`backend/internal/model/model.go:44`).
3. **Store** — extend `SetAvailability(participantID, dates, noteOverrides, preferred)` to accept a `preferred *map[string]bool` (or equivalent "nil means don't touch, non-nil means replace"). Read current preferred in the same transaction, merge with override-or-preserve per date, re-insert with `preferred` column. Read path (`ListAvailability`) selects the new column.
4. **Service** — extend `service.SetAvailability` signature to pass preferences through. Reuse `EventAvailabilityChanged` — no new event type.
5. **Handler** — extend `PUT /availability` decoder to accept optional `preferred []string`. Validate:
   - Field absent/null → skip all vote writes (don't-touch).
   - Field present → build `map[string]bool` from the list, defaulting every `dates` entry to `false` unless listed in `preferred`.
   - Any date in `preferred` not in `dates` → 400 with `"preferred date X is not in dates"`.
   - Dedupe `preferred` server-side (convert to set before validating).
   - Finalised calendar rejects vote writes with 409 (same rule as notes/dates today).
6. **Backend tests** (integration)
   - Vote round-trips on GET.
   - Vote cleared when date removed from selection.
   - Vote cleared by explicit `preferred: []`.
   - Votes preserved when `preferred` field absent from request.
   - Vote-without-matching-date returns 400.
   - Duplicate entries in `preferred` dedupe (don't 400).
   - Finalised calendar rejects vote writes with 409.
   - Per-participant isolation: editing A's votes doesn't affect B's row's `preferred`.
7. **Handler tests** — validation paths for absent/present/empty/unknown-date/duplicate `preferred`.
8. **Frontend types** — add `preferred?: boolean` to the availability row type (`frontend/src/lib/types.ts`) and `preferred?: string[]` to `SetAvailabilityRequest`.
9. **Frontend state** — add `votesByDate: Set<string>` to `CalendarPageClient`, populated from the fetched calendar. Include it in `queueSave` payload only when the user edited a vote (pattern mirrors `pendingNotesRef` — call it `pendingPreferredRef`).
10. **`CalendarGrid` vote badge** — render `.voteBadge` absolutely positioned bottom-right for cells with ≥ 1 vote. Shows `★` alone when count is 1, `★N` when >1. Styles in `CalendarGrid.module.css`. Not on past cells; rendered on finalised cells.
11. **`DayDetailSheet`** — add "Prefer this date" checkbox above the note editor. Wire to a new `onToggleVote(date, preferred)` prop. In the "Who's free" list, append ★ next to participants who preferred this date (`aria-label="{Name} prefers this date"`). Read-only when finalised: hide the checkbox, keep the stars.
12. **`CalendarPageClient.handleToggleVote`** — update `votesByDate`, mark `pendingPreferredRef.current = true`, call `queueSave` which serialises `preferred: [...votesByDate].filter(d => selectedDates.has(d))`.
13. **`BestDay`** — extend `computeSummary` (or compute alongside) to return per-date vote counts. Update the component to tie-break by vote count when free-counts tie, and update the subtitle copy (`"Everyone's free — N prefer this"` / `"X of Y free — N prefer this"`).
14. **Finalised banner** — append `"N preferred this date"` line if the finalised date has ≥ 1 vote.
15. **Frontend tests**
    - `CalendarGrid.test.tsx`: vote badge renders with correct count; no badge at count 0; badge hidden on past cells; rendered on finalised cells.
    - `DayDetailSheet.test.tsx`: checkbox reflects current state; toggle calls `onToggleVote` with the inverted boolean; stars render next to preferring participants; checkbox hidden (stars still shown) when finalised.
    - `CalendarPageClient.test.tsx`: toggling a vote fires a debounced PUT carrying only `preferred`; an availability-only edit doesn't send `preferred`; removing a date from selection implicitly drops the vote on the next GET (server-driven, but assert the client doesn't resend stale votes).
    - `BestDay.test.tsx`: two dates tied on free-count → more-voted wins; tied-tied → no best day; subtitle includes vote count when > 0.
    - `summary.test.ts`: vote-count aggregation in `computeSummary`.
16. **Analytics**
    - `vote_added { calendar_id_hash }` / `vote_removed { calendar_id_hash }` on toggle.
    - `best_day_shown { tiebreaker: "votes" | "availability" }` when the card renders so we can measure whether votes are doing the tiebreaking work.
    - **Never log date values, participant names, or calendar IDs in cleartext** (per AGENTS.md PII rule — dates aren't strictly PII but identify a group's plan). Use hashed IDs consistent with existing analytics.
17. **Docs**
    - `docs/api.md` — document `preferred` in `GET` response and `PUT` request, with the "absent = don't touch" semantics spelled out (cross-reference the same rule in the notes section).
    - `docs/v2-priorities.md` — mark the Tier 3 row shipped.
    - `docs/user-journeys.md` — add the vote affordance under "mark availability" and the tiebreaker behaviour under "pick a date."

## Out of Scope

- **Ranked voting** (1st / 2nd / 3rd choice). Numeric extension of the boolean. Ship binary first; revisit if users ask.
- **Vote budgets** ("you have 3 picks"). Product conversation, not a data-shape conversation.
- **Required voting** / enforced opt-in. Corrupts signal when participants have no preference.
- **Voting on dates you're NOT available on** ("I can't make Thursday but I'd love it if you picked it"). Different data shape; nonsense for the "find a date" use case.
- **Anonymous votes.** Votes follow the calendar's visibility rules — no separate per-feature privacy toggle.
- **Vote notifications** ("5 people voted for Thursday"). Live updates already reflect it; noisy as a notification.
- **Owner "locks in by votes" automation.** Finalise is still a deliberate owner action. The best-day card surfaces the vote signal; the owner decides.

## Log

- 2026-04-21 HH:MM — Created plan. Pending owner confirmation on: checkbox copy ("Prefer this date" vs alternatives like "Top pick"), cell badge glyph (★ vs outline/fill variant), whether `BestDay` tiebreaker subtitle should name the vote count ("— 2 prefer this") or stay generic. Implementation otherwise ready to proceed.
- 2026-04-21 18:00 — Implemented all phases: migration 013 (`preferred BOOLEAN NOT NULL DEFAULT FALSE`), backend model/store/service/handler with "absent = don't touch" semantics matching notes, integration + handler tests, frontend types, `votesByDate` state wired through `CalendarPageClient`, `★` cell badge in `CalendarGrid`, "Prefer this date" checkbox + per-participant stars in `DayDetailSheet`, vote-aware tiebreak in `BestDay` with `best_day_shown { tiebreaker }` analytics, vote-count line on `FinalisedBanner`, `vote_added`/`vote_removed` analytics, docs updates (`api.md`, `v2-priorities.md`, `user-journeys.md`). Backend tests (unit + integration) and frontend vitest run clean (210 tests).
