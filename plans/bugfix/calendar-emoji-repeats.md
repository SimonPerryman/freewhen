# Calendar emoji repeats

Status: done

## Goal

New calendars get an emoji auto-picked from a curated set of 12
(`backend/internal/service/emoji.go`). The old pick was uniform at random and
ignored the owner's other calendars, so repeats were common: 24% of owners
with 3 calendars and 62% with 5 had at least one repeat. In the README run,
two calendars both got 🍒. The auto-pick should avoid the owner's existing
emojis, and once all 12 are in use it should pick the least-used one,
weighted by recency.

Owner feedback: `.ai-cache/ui-fixes/feedback.md` §4. Notes:
`.ai-cache/ui-fixes/NOTES.md` §4.

## Rule

For each emoji in the curated set:

    weight(emoji) = Σ over the owner's live calendars using it of 0.5^(idle / 90 days)
    idle          = now − calendar.updated_at   (clamped at 0)

Pick the emoji with the **lowest weight**, and break ties uniformly at random.

- **Unused first.** An unused emoji weighs exactly 0. Any use, however old,
  adds a positive weight, and weights are compared exactly (no epsilon), so
  every unused emoji beats every used one.
- **Least-used, recency-weighted.** A calendar active today counts 1, one idle
  for 90 days counts 0.5, and one idle for a year counts about 0.06. Three
  year-old calendars on 🍋 (≈0.18) therefore lose to one active calendar on 🦊
  (1.0).
- **What counts as activity: `calendars.updated_at`.** Checked: it is bumped
  by rename, passphrase changes, finalise/unfinalise, soft-delete, and (via
  `Service.touch` → `TouchCalendar`) every availability write (dates, notes,
  preferred votes, calendar import) and every participant join, claim,
  rename, removal or leave. Invite edits don't bump it, which is fine
  because they're owner setup work, not group activity. It's
  also the My calendars sort key, so "recent" here matches "recent" in the
  UI. `created_at` would treat a long-running active calendar as old, and a
  MAX over availability rows would cost a join for no extra signal.
- **Scope:** only live calendars the user owns (`owner_id = $1`,
  `deleted_at IS NULL`, `emoji IS NOT NULL`). Calendars the user has joined
  don't count. Emojis outside the curated set (for example from a future
  picker) are ignored.
- The query is bounded by `limits.MaxCalendarsPerUser` (200) rows, so it's
  cheap.

## Plan

1. `model.EmojiUse{Emoji, LastActive}`.
2. `store.ListEmojiUsageForOwner(ctx, ownerID)`: `SELECT emoji, updated_at`
   with the scope above.
3. A pure `pickEmoji(usage, now, r *rand.Rand)` in `service/emoji.go`, with
   `nil` r using the global source. It replaces `pickRandomEmoji`.
4. `Service.CreateCalendar` loads the usage after the cap check and passes
   `pickEmoji(usage, time.Now(), nil)` to the store. A lookup error fails the
   create, the same way the `CountCalendarsForOwner` error just above it does.
5. Tests:
   - Unit: table-driven `pickEmoji` cases (unused first, least-used, an old
     heavy emoji losing to a recent light one, the 90-day half-life, foreign
     emojis ignored, the future-timestamp clamp). Seeded tie-breaks are
     deterministic and spread across candidates. Twelve sequential picks
     never repeat. Service tests cover the wiring and the error path.
   - Integration: the store query's scoping, and creating 12 calendars via
     the API gives 12 distinct emojis.

## Out of scope / defaults kept

- `DuplicateCalendar` still copies the source emoji on purpose (it reads as a
  series).
- No backfill of existing repeats, and no migration.
- A user-chosen emoji may repeat. Choosing one is a separate PR (calendar
  settings picker).
- Docs: `docs/api.md` and `docs/onboarding.html` don't describe emoji
  selection, so no doc changes.

## Log

- 2026-11-24 12:10 — Created plan. Confirmed `updated_at` is bumped on all calendar activity via `TouchCalendar`, so it's used as the recency signal.
- 2026-11-24 12:40 — Implemented the store query, `pickEmoji`, and the service wiring. Dropped an initial 1e-9 tie epsilon: a 10-year-idle calendar weighs ~6e-13, so the epsilon let it tie with unused emojis.
- 2026-11-24 12:55 — Unit, service, store and server integration tests pass. Status → done.
