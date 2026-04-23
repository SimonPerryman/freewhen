# Remove calendar colour themes

Status: done

## Goal

Rip out the calendar colour themes feature shipped in `f2a80a3` (2026-04-21).
The feature works, but on review it doesn't earn its keep: it's a cosmetic
knob that solves a problem nobody asked about, adds ~900 lines across the
stack, and the very next day (`c5664fa`) it had to be hidden behind a
settings sheet because its UI weight was already disproportionate to its
value. See `docs/cut-features.md` for the decision log; this plan is the
mechanical removal.

Not a rollback of the commit — migration 014 is already in trunk with
migration 015 on top of it, so removal is a *new* forward-only change.

## Scope

**In**
- Drop the `theme` column from `calendars` via a new migration.
- Remove backend theme plumbing: service allow-list + picker, store
  read/write, handler PATCH field, model field, tests.
- Remove frontend theme plumbing: `ThemePicker` component, `themes.ts`
  module, `data-theme` wrapper, theme CSS blocks in `globals.css`, the
  `theme` prop on `CalendarSettingsSheet`, the type field, the API
  client bits.
- Update `docs/api.md` to remove the `theme` field from the PATCH body.
- Update `docs/v2-priorities.md` — keep the row but annotate it as
  "previously shipped and cut" (see Resolved decisions).
- Mark `plans/ui/calendar-colour-themes.md` status as `cut` and cross-link
  `docs/cut-features.md`.
- Add the feature to `docs/cut-features.md` with rationale and
  restoration notes.

**Out**
- Revert commit `f2a80a3` directly. It's entangled with later commits
  (015 migration, settings sheet) and a clean revert isn't possible.
- Any broader audit of other features that might also be scope-cut —
  that's a separate exercise.
- Preserving the theme palette in any form. If this gets restored later,
  the palette values live in git history and in the original plan.

## Design decisions

- **Forward-only migration, not a drop of 014.** Migrations run on boot
  against production already running with 014 applied. A new migration
  `016_drop_calendar_theme.sql` does `ALTER TABLE calendars DROP COLUMN
  theme;`. The old `014_add_calendar_theme.sql` stays on disk so the
  history is honest — a fresh environment will add and then drop the
  column, which costs one no-op round-trip but preserves "migrations are
  immutable once merged."
- **Delete the plan? No — mark it `cut`.** `plans/ui/calendar-colour-themes.md`
  stays on disk as a record of what was built and why it was removed.
  Set `Status: cut` at the top and append a log entry pointing at this
  removal plan and `docs/cut-features.md`. Future-you searching the
  plans directory will find it.
- **Cut-features doc lives in `docs/`, not `plans/`.** It's not a unit
  of work; it's long-lived project context that survives after every
  individual removal plan is done. Same shelf as `v2-priorities.md` and
  `future-ideas.md`.
- **Revert `docs/v2-priorities.md` rather than adding a "cut" marker to
  the table.** The table is a forward-looking priority list; a ship-then-
  cut roundtrip is a cut-features doc concern. Restore the original row
  text so the priority list reads as if themes never shipped, with a
  parenthetical `(previously shipped and cut — see docs/cut-features.md)`
  on the row. This keeps the priority list scannable without erasing
  history.
- **No data preservation.** Existing `theme` values on live calendars
  are thrown away with the column drop. The palette was server-picked
  at random anyway; nobody has an emotional attachment to their
  calendar being "plum." If we ever restore, we re-pick.
- **Keep the feature off the settings sheet cleanly.** The sheet was
  structured so that theme selection sat below rename / visibility.
  Removing it just collapses that section — the sheet doesn't need a
  redesign, just an import and a block removed.

## Plan

1. **Backend — remove theme column.**
   - Add `backend/internal/store/migrations/016_drop_calendar_theme.sql`:
     `ALTER TABLE calendars DROP COLUMN theme;`
   - Remove `Theme string` from `model.Calendar`.
   - Remove theme parameter / read-back from `store.CreateCalendar`,
     `store.GetCalendar`, `store.GetCalendarOnly`, `store.UpdateCalendar`,
     and the duplicate path.
   - Delete `backend/internal/service/themes.go`.
   - Remove theme pick from `Service.CreateCalendar` and theme
     validation from `Service.UpdateCalendar`.
   - Remove the `theme` field from the `PATCH /calendars/{id}` handler
     DTO and its validation.
   - Strip theme assertions from `handler_test.go`, `service_test.go`,
     `calendar_test.go`, `availability_test.go`, `participant_test.go`.
   - Run `make test-backend` and `make test-integration`.

2. **Frontend — remove theme UI.**
   - Delete `frontend/src/components/ThemePicker/` (component, CSS
     module, test).
   - Delete `frontend/src/lib/themes.ts` and `themes.test.ts`.
   - Remove `theme?: string` from the `Calendar` type in
     `frontend/src/lib/types.ts`.
   - Remove the `ThemePicker` import, `theme` prop, and `onThemeChange`
     wiring from `CalendarSettingsSheet.tsx`.
   - Remove the `data-theme={…}` attribute (and any fallback constant
     usage) from `CalendarPageClient.tsx`.
   - Delete the `[data-theme="…"]` blocks from
     `frontend/src/app/globals.css` (light + dark for all six slugs).
   - Remove theme from any calendar-update API helper.
   - Run `make test-frontend` and `make lint`.

3. **Docs.**
   - `docs/api.md` — remove the `theme` field from `PATCH /calendars/{id}`.
   - `docs/v2-priorities.md` — restore the row to its pre-ship state
     with a `(previously shipped and cut — see docs/cut-features.md)`
     suffix.
   - `plans/ui/calendar-colour-themes.md` — set `Status: cut`, add a log
     entry pointing at this plan.

4. **Create `docs/cut-features.md`** (see separate plan section below
   for its shape — first entry is this feature).

5. **Verify.**
   - `make lint && make test && make test-integration` all green.
   - Manually: open a calendar page, confirm no broken imports, no
     stray `data-theme` in the DOM, settings sheet renders without the
     theme row.

## `docs/cut-features.md` shape

One doc, append-only, newest-at-top. Each entry is a short block:

```markdown
## <Feature name>

- **Shipped:** YYYY-MM-DD — commit `<sha>` — plan `plans/<file>.md`
- **Cut:** YYYY-MM-DD — commit `<sha>` — plan `plans/<file>.md`
- **Why cut:** <1-3 sentences. The real reason, not a sanitised one.>
- **What it did:** <2-3 sentences of scope so a future reader doesn't
  need to read the original plan to know what we're talking about.>
- **Downstream decisions changed:** <bullet list of choices that only
  made sense while this feature existed and have now been rolled back
  or should be revisited.>
- **To restore:** <one paragraph. Which commits to cherry-pick, which
  migration numbers would need renaming, which assumptions from the
  original plan need re-evaluating against current code.>
```

Top-of-file preamble explains what this doc is for: a record of
features that shipped (or were built and nearly shipped) and then
removed, so the team can re-find them when someone asks "didn't we try
X?" and so restoring one is a known-cost exercise rather than
archaeology.

First entry: Calendar colour themes.

## Resolved decisions

- **Keep the themes row in `v2-priorities.md`** with a "previously
  shipped and cut — see docs/cut-features.md" annotation. The anti-re-
  propose signal is worth the line.
- **Don't tag the pre-removal SHA.** The original plan proposed a
  `themes-shipped-2026-04-21` tag as a cherry-pick target. On reflection
  this is namespace pollution: the ship commit SHA (`f2a80a3`) is already
  recorded in `docs/cut-features.md` under "To restore," and `git
  cherry-pick <sha>` works fine without a tag. Tags are a shared
  namespace; creating one per cut feature gets noisy, and if the feature
  is never restored the tag is dead weight. Keep the SHA in the
  cut-features entry — that's sufficient.

## Log

- 2026-04-22 — Created plan.
- 2026-04-22 — Resolved open questions: keep the v2-priorities row
  with a cut annotation; decided against tagging the pre-removal SHA
  (the SHA in `docs/cut-features.md` is enough).
- 2026-04-22 — Implemented. Migration 016 drops `calendars.theme`;
  removed backend theme plumbing (service allow-list, store helpers,
  handler PATCH field, model field, tests); removed frontend
  `ThemePicker`, `themes.ts`, `data-theme` wrapper, `globals.css`
  theme blocks, and settings-sheet theme section. Updated `docs/api.md`,
  `docs/v2-priorities.md`, and marked `plans/ui/calendar-colour-themes.md`
  as cut. Created `docs/cut-features.md` with the first entry.
