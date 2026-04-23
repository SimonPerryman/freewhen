# Theme refresh — Paper default + light/dark toggle

Status: pending

## Goal

Replace the current **light** palette with the warmer, more editorial "Paper"
palette from `plans/v2/t3/mocks/calendar-integrations-cell-states.html`, and ship a
user-facing light/dark toggle instead of only following `prefers-color-scheme`.

Two themes: **Paper (new light)** and **current dark (unchanged)**. The dark
palette is already dialled in — only light is being redesigned. No per-user
theme library beyond that.

Comparison mock: `plans/ui/mocks/themes-compare.html`.

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
   - Add a light/dark toggle. Location TBD during build — likely in
     `UserMenu` (signed-in) and a small icon button for anon users, either in
     the header or inside the "My calendars" sheet. Mobile-first: tap target
     ≥44px, icon-only is fine.
   - Writes `localStorage.colorScheme` and updates `data-color-scheme` on
     `<html>`.

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

## Log

- 2026-04-21 23:25 — Created plan
- 2026-04-21 23:40 — Decided to keep current dark palette; only light is being redesigned. Added `plans/ui/mocks/themes-compare.html` as side-by-side reference.
