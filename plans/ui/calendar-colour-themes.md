# Calendar colour themes

Status: cut

## Goal

Let the calendar owner pick a visual vibe for their calendar — a small palette
swap that personalises the page without touching layout. This is the Tier 3
"Calendar colour themes" item in `docs/v2-priorities.md:43` ("Let creator pick a
vibe. Minor but personal.").

The texture we're going for: when a group opens `…/calendars/tennis-friday`,
the accent colour on selected cells, buttons, and badges is *the tennis
calendar's* colour — not the same warm-terracotta every calendar shares today.
Different calendars in the same owner's `/calendars` list should feel visually
distinct at a glance.

## Scope

**In**
- A curated set of ~6 named themes (slugs, not hex). Each theme defines a
  light-mode and dark-mode palette.
- `theme VARCHAR(32)` column on `calendars`, nullable. `NULL` means "default"
  and renders as the current accent — no backfill required.
- New calendars get a theme picked at create time (same pattern as the emoji:
  server-side pick from the curated set, stored once, stable for the life of
  the calendar).
- `PATCH /calendars/{id}` accepts `theme` — owner-only, validated against the
  curated allow-list.
- Owner-only theme picker on the calendar page: a small swatch row in the
  existing owner menu / settings surface, next to rename / duplicate.
- Theme applied by toggling `data-theme="<slug>"` on the calendar page
  container. Scoped CSS variables override `--accent`, `--accent-hover`,
  `--accent-subtle`, `--accent-text` (plus dark-mode variants) on that subtree
  only. No global app chrome changes.

**Out**
- Free-form colour picker / hex input. Invites bad contrast and off-brand
  results; the curated set keeps the product cohesive. If demand is real we
  revisit.
- Theming anything beyond the accent (backgrounds, borders, typography). The
  page layout stays consistent across calendars; only the accent moves.
- Per-user theme preference. Themes belong to the *calendar*, not the viewer —
  so the creator's vibe lands with everyone who opens the link.
- Theming the `/calendars` list or the home page. Themes are a
  calendar-page-scoped concern.
- A custom picker at create time. Keep the create flow zero-decision; owner
  changes the theme after landing on the calendar if they want to.

## Design decisions

- **Slug, not hex.** Storing `"ocean"` rather than `"#3b7a8e"` means we can
  tune the palette (contrast tweaks, dark-mode rebalance, new accents per
  theme) without migrating data. The store knows nothing about colours; it's
  just a tag.
- **Curated set defined in one place.** `frontend/src/lib/themes.ts` exports
  the canonical list of `{ slug, label, swatch }`. The CSS file references the
  same slugs via `[data-theme="<slug>"]` blocks. The backend has its own
  `internal/service/themes.go` slug allow-list (mirrored constant) so the
  validator doesn't need to import frontend assets. Drift risk is low — a new
  theme is "add a slug in two places and a CSS block"; a test locks the Go
  allow-list against the hard-coded list so it can't silently diverge.
- **Nullable column, render falls back.** Same rationale as the emoji column:
  old calendars predate the feature and should render the default accent, not
  pick one on read. New calendars never write `NULL`.
- **`data-theme` on a page-scoped container, not `<html>`.** Global theming
  would bleed the selected calendar's accent into `/calendars`, the header,
  modals that portal to the body, etc. Scoping to
  `CalendarPageClient`'s root keeps the accent change local. Any portal
  component that needs the accent (toast, day sheet) either lives inside the
  scoped subtree or reads the slug via context — the day sheet already lives
  under the calendar page, so this is mostly free.
- **Dark-mode variants per theme.** The existing dark-mode palette shifts
  `--accent` to a warmer, higher-L value for contrast against the dark
  background. Each theme needs its own dark variant rather than reusing the
  light one; otherwise "ocean" goes muddy on dark. Declared via
  `@media (prefers-color-scheme: dark) [data-theme="ocean"] { … }` blocks.
- **Server-side random pick on create.** Avoids a "pick your theme" step in
  the already-minimal create flow. Owner retunes it from the page if the
  default doesn't land. Same primitive as `pickRandomEmoji()` — reuse the
  pattern, don't reinvent it. `pickRandomTheme()` lives in
  `internal/service/themes.go`.
- **PATCH, not POST to a dedicated route.** The calendar update handler
  already handles `name`, `visibility`, `emoji`, `passphrase`. Adding `theme`
  is a natural extension — one code path for "change this calendar's
  presentation." Validation rejects slugs not in the allow-list with 400.
- **Picker UI is swatches, not a dropdown.** A horizontal scrollable row of
  coloured circles with the slug as aria-label. Current theme gets a check /
  ring. This reads as "pick a vibe" at a glance and fits the existing
  day-sheet / participant-list visual language. Lives inside the owner menu
  panel, below rename.
- **Accessibility.** Every palette is picked for AA contrast against the page
  background in both light and dark mode. `--accent-text` is declared per
  theme to guarantee readable button label colour (white on ocean, dark on
  lemon, etc). Swatches have `aria-label` and a visible check on the selected
  theme — colour alone never conveys state.
- **No live broadcast.** Theme changes don't go down the SSE channel. The
  next time a viewer reloads they'll see it. Live-syncing a cosmetic change is
  not worth the complexity; if someone's mid-session when the owner swaps
  themes, seeing the same accent for the rest of the session is fine.

## Curated set (v1)

Working names — palette values land during implementation after a contrast
check. Each entry needs light + dark `--accent` / `--accent-hover` /
`--accent-subtle` / `--accent-text`.

- `peach` — current warm terracotta (today's default, now named).
- `ocean` — desaturated teal-blue.
- `forest` — deep sage green.
- `plum` — muted magenta / aubergine.
- `lemon` — soft mustard / ochre (needs dark text on accent).
- `graphite` — near-monochrome grey, for the "calm" vibe.

Six is enough to feel distinct on a `/calendars` list of half a dozen
calendars without turning the picker into a decision. We can grow the set
later; removing a slug requires care because existing rows reference it, so
err on the side of "only add" and keep the list tasteful from the start.

## Plan

1. **Migration** — `XXX_add_calendar_theme.sql` adds `theme VARCHAR(32)` to
   `calendars`. Nullable, no default.
2. **Model** — add `Theme string` (`omitempty`) to `model.Calendar`.
3. **Store** — `CreateCalendar` takes a new `theme string` parameter and
   writes it via `sql.NullString`. `GetCalendar` / `GetCalendarOnly` read it
   back. `UpdateCalendar` accepts an optional theme update (same pattern as
   emoji).
4. **Service** — `internal/service/themes.go` exports `ValidThemes` (slug
   allow-list) and `pickRandomTheme()`. `Service.CreateCalendar` calls the
   picker. `Service.UpdateCalendar` validates an incoming theme against
   `ValidThemes` and returns a 400-mapped error on mismatch. Add a test that
   locks the allow-list against the expected set.
5. **Handler** — extend the `PATCH /calendars/{id}` DTO to accept an optional
   `theme` field. Round-trip test: owner changes theme, non-owner gets 403,
   unknown slug gets 400.
6. **Frontend types** — optional `theme?: string` on the `Calendar` type.
7. **Frontend themes module** — `src/lib/themes.ts` with the canonical list
   (`{ slug, label, swatch }`) and a `DEFAULT_THEME` constant used when
   `calendar.theme` is missing. Unit test that the list is non-empty and slugs
   are unique.
8. **Theme CSS** — add `[data-theme="<slug>"]` blocks to `globals.css` (or a
   dedicated `themes.css` imported from the layout), one per slug, covering
   light and dark. Default (`[data-theme="peach"]`) mirrors today's accent
   values so nothing shifts for unthemed calendars.
9. **Apply on the calendar page** — `CalendarPageClient` sets
   `data-theme={calendar.theme ?? DEFAULT_THEME}` on its root wrapper.
10. **Picker UI** — add a `ThemePicker` component (swatches + current marker)
    mounted inside the owner-only menu / settings surface on the calendar
    page. On change, calls the existing calendar-update API helper with
    `{ theme }`, updates local state optimistically, rolls back on error with
    a toast.
11. **Tests** — store round-trip (theme set / `NULL`); service
    `pickRandomTheme` membership; handler PATCH happy path + invalid slug +
    non-owner; frontend renders correct `data-theme`; `ThemePicker` fires the
    update and surfaces errors.
12. **Docs + mark shipped** — tick the row in `docs/v2-priorities.md:43`.
    Short note in `docs/api.md` that PATCH accepts `theme`.

## Open questions

- **Is a rotating default better than random?** Server-side random gives
  variety across a single owner's calendars (great). It can also repeat
  (mildly annoying). A hash-of-calendar-id pick would be deterministic but
  brittle if we ever reorder the theme list. Random + the owner's ability to
  re-pick feels right — flagging in case we want to try "avoid the owner's
  most recent theme" later.
- **Should the calendar's theme bleed into its row on `/calendars`?** Would
  make the list more personal (same vibe carries across). Out of scope for
  this plan but natural follow-up once the primitive exists — probably a tiny
  swatch dot next to each calendar name, not a full row tint.
- **Contrast against `--accent-subtle` backgrounds for selected cells.** The
  current grid uses `--accent` for a filled selected cell and `--accent-subtle`
  for hover / other participants' availability. Per-theme tuning needs both
  colours to stay distinguishable at a glance; lock in during implementation
  with a side-by-side screenshot check.

## Log

- 2026-04-21 — Created plan.
- 2026-04-21 21:48 — Shipped. Migration 014 adds `theme VARCHAR(32)`; service picks a random slug on create from a six-slug curated set (`peach`, `ocean`, `forest`, `plum`, `lemon`, `graphite`); `PATCH /calendars/{id}` accepts `theme` with allow-list validation; duplicate preserves source theme; frontend renders `data-theme="<slug>"` on the calendar page wrapper, scoped accent overrides live in `globals.css`, and an owner-only `ThemePicker` (swatch row) sits below the rename controls.
- 2026-04-22 — Cut. See `plans/ui/remove-calendar-themes.md` and the entry in `docs/cut-features.md`. Feature removed via forward-only migration 016; column and UI dropped from the stack.
