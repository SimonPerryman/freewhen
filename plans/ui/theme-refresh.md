# Theme refresh — Paper default + light/dark toggle

Status: done

## Goal

Replace the current **light** palette with the warmer, more editorial "Paper"
palette from `plans/v2/t3/mocks/calendar-integrations-cell-states.html`, and ship a
user-facing light/dark toggle instead of only following `prefers-color-scheme`.

Two themes: **Paper (new light)** and **current dark (unchanged)**. The dark
palette is already dialled in — only light is being redesigned. No per-user
theme library beyond that.

Comparison mocks:

- `plans/ui/mocks/theme-before-after.html` — current light vs Paper light, with dark for reference and a diff strip of what's changing.
- `plans/ui/mocks/themes-compare.html` — Paper light next to current dark.

## Context

- We have no users yet, so the disruption cost of a global palette change is
  zero.
- The mock's tokens are already authored for both light and dark, and its token
  names map cleanly onto our existing `--background` / `--foreground` /
  `--border` / `--text-*` semantic tokens.
- `data-theme` is **already in use** for per-calendar accent themes
  (`peach` / `ocean` / `forest` / `plum` / `lemon` / `graphite`) — see
  `frontend/src/app/globals.css` + `frontend/src/lib/themes.ts` +
  `backend/internal/service/themes.go`. The new light/dark switch must use a
  different attribute (`data-color-scheme`) so the two systems compose.
- Current dark mode is driven by `@media (prefers-color-scheme: dark)`. We
  want it to be controllable, so dark rules move to
  `[data-color-scheme="dark"]`. `prefers-color-scheme` only sets the *initial*
  value when the user hasn't chosen.

## Plan

1. **Base tokens (`globals.css`)**
   - Rewrite `:root` using the mock's Paper palette. Map:
     - `--bg` → `--background` / `--page-bg`
     - `--bg-raised` → `--card-bg`
     - `--ink` → `--foreground` / `--text-primary`
     - `--muted` → `--text-secondary`
     - `--faint` → new token (used sparingly — placeholder text, disabled)
     - `--border` / `--border-soft` → `--border` + new `--border-soft`
   - Keep existing spacing/radius/shadow tokens untouched.
   - Leave `--accent` / `--accent-hover` / `--accent-text` / `--accent-subtle`
     driven by per-calendar `[data-theme="…"]` as today. Update the
     `peach` (default) variant to the mock's teal so unthemed calendars match
     the new chrome; keep the other five calendar accents as-is.

2. **Dark variant**
   - Keep the current dark palette values verbatim.
   - Move the current `@media (prefers-color-scheme: dark)` block to
     `[data-color-scheme="dark"]` so the toggle can force it.
   - Same for the per-calendar dark overrides (`[data-theme="peach"]` inside
     the dark media query → `[data-color-scheme="dark"] [data-theme="peach"]`,
     etc.).

3. **Initial-scheme script**
   - Inline blocking script in `src/app/layout.tsx` `<head>` that reads
     `localStorage.colorScheme` (values: `light` | `dark` | unset), falls
     back to `matchMedia('(prefers-color-scheme: dark)')`, and sets
     `document.documentElement.dataset.colorScheme` before paint. Prevents
     FOUC / flicker.

4. **Toggle UI**
   - Signed-in: new "Appearance" row inside the `UserMenu` panel (between the
     identity block and "My calendars"), as a segmented Light / Dark control
     with sun/moon icons — same `menuItem` slot pattern as the rows below it.
   - Anon: icon-only sun/moon button in `Header` actions next to the "Sign in"
     link. Toggles directly (no menu). ≥44px tap target.
   - Both entry points share one handler that writes
     `localStorage.colorScheme` and updates `data-color-scheme` on `<html>`.
   - Not in the "My calendars" sheet — it isn't reachable from login / join /
     public calendar pages, where anon users most need the toggle.

5. **Typography (deferred decision)**
   - Mock uses Instrument Serif for display + Geist for body. Current app uses
     `--font-body` (Geist or similar). Treat this as a follow-up — ship the
     palette swap first, decide on serif headings separately so we can
     evaluate it against the real app chrome, not a mock.

6. **Sweep**
   - Grep component CSS modules for hardcoded hex values that bypass tokens
     (known offenders: `CalendarGrid`, `DayDetailSheet`, `PassphraseGate`
     from the earlier grep). Replace with tokens where they should follow
     the theme; leave per-participant colours (`--p1`/`--p2`/`--p3` equivalents)
     as-is.
   - Verify the per-calendar accent themes still look right on the new
     background in both schemes.

7. **Manifest / theme-color meta**
   - Update `manifest.ts` `theme_color` / `background_color` and any
     `<meta name="theme-color">` to match Paper's bg. Consider a second
     `theme-color` with `media="(prefers-color-scheme: dark)"` so the
     browser chrome matches Ink when active.

8. **Test pass**
   - 375px mobile check: home, calendar, day sheet, join form, passphrase
     gate, login.
   - Toggle from light → dark → light, reload with each setting, confirm no
     FOUC.
   - Confirm per-calendar themes still swap accent correctly under both
     schemes.

## Out of scope

- User-pickable palette library beyond light/dark.
- Changes to per-calendar accent theme UI.
- Typography overhaul (see step 5).

## Amendment — 2026-05-10: split `--card-bg` into inline vs floating surfaces

The original Paper brief specified `--bg-raised: #FAF6ED` for all raised
surfaces. In the live calendar grid this reads too bright: tiles look like
SaaS cards-on-background rather than a single planner page. The metaphor
the theme is selling is *paper* — one sheet, where cells are defined by ink
rules and hairlines, not by being a brighter object stuck on top. The mock
was authored on isolated dense cells, so the issue didn't surface until the
full 7×N grid of mostly-empty cells was rendered.

Root cause: one `--card-bg` token is being asked to serve two jobs that want
opposite treatments.

- **Inline surfaces** (date tiles, Weekends/Weekdays/Clear pills, anything
  that's part of the page): should tone *into* the page with a hairline
  border, no shadow. The dates **are** the paper.
- **Floating surfaces** (UserMenu panel, day-detail sheet, modals, toasts):
  should lift off the page, earn a soft shadow, stay near the current
  `#faf6ed`.

### Changes

1. Add `--tile-bg` token:
   - Light: `#ece5d5` (slightly *darker* than `--background` `#f3eee4`).
   - Dark: pick a value slightly lighter than `--dk-bg` but darker than
     `--dk-paper` — likely around `#1a1713` — so dark tiles also tone into
     the page rather than lifting.
2. Keep `--card-bg` as-is for floating surfaces.
3. Swap `--card-bg` → `--tile-bg` in the calendar cell, the
   Weekends/Weekdays/Clear pills, and any other "inline" raised surface that
   currently reads as a card. UserMenu panel, sheets, hero "Best day" card,
   toasts stay on `--card-bg`.
4. Tile borders use `--border-soft` (already exists). No shadow on tiles.
5. Update `plans/v2/t3/mocks/calendar-integrations-cell-states.html` and
   `plans/ui/mocks/theme-before-after.html` to reflect the split — the
   original mock's `--bg-raised: #FAF6ED` for cells was the part of the brief
   that didn't survive contact with the real grid.

### Out of scope for this amendment

- Re-evaluating shadow tokens.
- Per-calendar accent themes (still on `data-theme`, unaffected).

## Log

- 2026-04-21 23:25 — Created plan
- 2026-04-21 23:40 — Decided to keep current dark palette; only light is being redesigned. Added `plans/ui/mocks/themes-compare.html` as side-by-side reference.
- 2026-05-10 — Added `plans/ui/mocks/theme-before-after.html` showing current light vs Paper light, with the dark panel for reference and a diff strip of what's changing.
- 2026-05-10 — Locked in toggle placement: Appearance row inside `UserMenu` for signed-in users, icon-only sun/moon button in `Header` for anon users.
- 2026-05-10 19:36 — Implemented:
  - Rewrote `globals.css` with the Paper light palette + dark scheme, dark moved from `@media (prefers-color-scheme: dark)` to `[data-color-scheme="dark"]`.
  - Added `--border-soft` and `--text-faint` tokens; dropped legacy brand vars (`--cream`, `--ink`, `--ink-2`, `--terra`, `--paper`, `--terra-soft`).
  - Updated `HeroLiveGrid` and `BestDay` CSS modules to consume semantic tokens (`--accent`, `--card-bg`, `--text-secondary`) instead of the removed brand vars; their dark-mode media queries became `[data-color-scheme="dark"]` selectors.
  - New `src/lib/colorScheme.ts` (read/persist/apply + `COLOR_SCHEME_INIT_SCRIPT`).
  - New `ColorSchemeToggle` component exposing `ColorSchemeSegmented` (UserMenu) and `ColorSchemeIconButton` (Header). 44px tap target on icon button.
  - `UserMenu` shows the segmented Light/Dark control between identity and "My calendars".
  - `Header` shows the icon button next to the "Sign in" link for anon users.
  - Inline blocking `<script>` in `layout.tsx` `<head>` reads `localStorage.colorScheme` and falls back to `prefers-color-scheme` — sets `data-color-scheme` before paint to avoid FOUC.
  - `manifest.ts` and viewport `themeColor` updated to the Paper bg (`#f3eee4`) for light; dark stays `#141210`.
  - `data-theme` per-calendar accent system referenced in the plan does not yet exist in the codebase, so the composition concern was moot — `data-color-scheme` is the only attribute currently used.
  - Typography swap (Instrument Serif headings) deferred per plan.
  - `pnpm exec oxlint` clean; `pnpm exec vitest run` 259/259 passing.
- 2026-05-10 — Amendment added: split `--card-bg` into `--card-bg` (floating) and `--tile-bg` (inline). Original `#FAF6ED` raised tone made calendar cells read as SaaS cards rather than paper-on-paper; mock was authored on isolated dense cells so the issue didn't surface until the full grid was live. See Amendment section above.
- 2026-05-10 — Implemented amendment:
  - Added `--paper-tile: #ece5d5` and `--dk-tile: #1a1713` raw tokens; new semantic `--tile-bg` mapped per scheme.
  - `CalendarGrid` `.day`, `.toolbarButton` (Weekends/Weekdays/Clear), and `.navButton` (prev/next chevrons) switched to `--tile-bg` + `--border-soft` and dropped `box-shadow` — they now tone into the page rather than lifting off it.
  - `.suggestedFree` color-mix base and the JS heat-map `backgroundColor` in `CalendarGrid.tsx` updated to mix against `--tile-bg` so heat blending stays consistent.
  - Floating surfaces (UserMenu, sheets, toasts, hero card, etc.) untouched — still on `--card-bg`. Mock files not touched in this pass.
  - `pnpm exec oxlint` clean; `pnpm exec vitest run` 259/259 passing.
