# Calendar emoji

Status: done

## Goal

Give every calendar a small visual identity by rendering an emoji to the left of
its name in the page header (Notion-style). The emoji is picked at create time
from a small curated set so it's stable for the life of the calendar. A future
picker will let users overwrite it; until then this is decoration only.

## Design decisions

- **Storage**: a single `emoji VARCHAR(16)` column on `calendars`. Plain unicode
  value, no foreign key — emojis are already a stable universal identifier and
  there's nothing extra to model.
- **Nullability**: column is nullable. `NULL` means "calendar predates the
  feature"; render falls back to no emoji rather than substituting one in. New
  calendars never write `NULL`.
- **Selection at create time, not on the fly**: a hash-of-id approach would mean
  every calendar's emoji silently shifts whenever the curated set is reordered.
  Storing the value once means future tasteful re-curations don't break
  identity. It also means the picker is just "let the user overwrite the field
  we already write" — no second code path.
- **Curated set, not random unicode**: avoid 📅 / 🗓️ (would read as
  inference failure once we get more capable), avoid tonally loaded emojis
  (💀, 💊, 🚨), avoid anything ambiguous in tone. Aim for ~12 friendly,
  ambiguous icons.
- **Selection lives in the service layer, not the store**: the store does data,
  the service does the small bit of "pick one" logic. The store accepts an
  emoji argument so future callers (the picker) can pass a user-chosen value.
- **Markup**: emoji is a sibling of the `<h1>`, not inside it — keeps the
  existing `text-overflow: ellipsis` on the heading working when names are
  long.
- **No server-side validation yet**: the only writer is the server itself,
  picking from a known-good list. Validation arrives with the picker.

## Curated set (v1)

`✨ 🌿 🎈 🍋 🌊 🦊 🪴 🍒 🛼 🎏 🪩 🌻`

## Plan

1. **Migration** — `002_add_calendar_emoji.sql` adds `emoji VARCHAR(16)` to
   `calendars`. Existing rows get `NULL`.
2. **Model** — add `Emoji string` (with `omitempty` so `NULL`/empty doesn't
   pollute the JSON for old calendars) to `model.Calendar`.
3. **Store** — `CreateCalendar` takes a new `emoji string` parameter and writes
   it. `GetCalendar` reads it via `sql.NullString` and surfaces as `""` when
   `NULL`. Update the integration test fixtures.
4. **Service / handler** — add a small `internal/service/emoji.go` with the
   curated set and a `pickRandomEmoji()` helper. `Service.CreateCalendar` calls
   the helper and forwards to the store.
5. **Frontend types** — add optional `emoji?: string` to `Calendar`.
6. **Frontend rendering** — wrap heading + emoji in a flex row in
   `CalendarPageClient`. Emoji is a sibling `<span aria-hidden="true">`. CSS
   adds `.titleRow` (flex, gap, min-width: 0) and `.emoji` (28px, flex-shrink: 0,
   emoji font fallback stack).
7. **Tests** — bump test fixtures (`makeCalendarData`, handler tests) to
   include the new field; add a frontend test that the emoji renders alongside
   the name; add a service-layer test that `pickRandomEmoji` returns a value
   from the curated set.
8. **Commit + push**.

## Out of scope

- The picker UI.
- Server-side emoji validation (lands with the picker).
- A backfill for old `NULL` rows (next session — easy to do once the set is
  agreed).
- A hover affordance on the emoji (lands with the picker so it doesn't lie
  about being interactive).

## Log

- 2026-04-18 23:50 — Created plan.
- 2026-04-18 23:53 — Implemented end-to-end. Migration `002_add_calendar_emoji.sql`
  adds the column. `model.Calendar` gains `Emoji string` (omitempty). Store's
  `CreateCalendar` now takes `(name, emoji string)` and round-trips via
  `sql.NullString`. Service exposes `pickRandomEmoji()` over a 12-emoji curated
  set and forwards to the store at create time. Frontend `Calendar` type gains
  optional `emoji`; `CalendarPageClient` renders it as a flex sibling of the
  heading so truncation still works. Tests added: store round-trip + null,
  service `pickRandomEmoji` membership check + service-level forwarding,
  frontend present/absent rendering. All tests pass; oxlint clean; oxfmt run.
