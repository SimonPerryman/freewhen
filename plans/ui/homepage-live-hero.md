# Homepage live hero (typewriter placeholder + animated grid)

Status: done

## Goal

Make the homepage feel alive without being distracting:

1. Cycle the event-name input's placeholder through a short list of phrases with a typewriter effect (currently a static "Weekend trip").
2. Animate the `HeroLiveGrid` so cells subtly shift heat intensity over time, instead of being a frozen pattern.

Both effects run only when the user isn't interacting, respect `prefers-reduced-motion`, and pause when the tab is hidden.

## Plan

### 1. Typewriter placeholder on `src/app/page.tsx`

- Extract phrase list as a module-level const: `["Weekend trip", "Sarah's birthday", "Team offsite", "Ski week", "Climbing weekend"]` (final list TBD — ~5 phrases).
- New `useTypewriterPlaceholder(phrases, { typeMs, holdMs, deleteMs })` hook in `src/lib/useTypewriterPlaceholder.ts` (or inline in `page.tsx` if it stays small):
  - Returns a `string` to feed into `<input placeholder={...} />`.
  - Internal state machine: `typing → holding → deleting → next`.
  - Defaults: `typeMs=60`, `holdMs=1800`, `deleteMs=35`.
  - Pauses (returns the current full phrase, no further mutation) when:
    - `name` (the controlled input value) is non-empty, OR
    - the input is focused, OR
    - `document.visibilityState !== "visible"`, OR
    - `window.matchMedia("(prefers-reduced-motion: reduce)").matches`.
  - Listens to `visibilitychange` and the reduced-motion media query so it resumes/stops correctly.
- Wire into `page.tsx`: track `isFocused` via `onFocus`/`onBlur`, pass `name`, feed the returned string into `placeholder`.
- Accessibility: placeholder is decorative; no `aria-live` needed. Keep `autoComplete="off"` as today.

### 2. Animated `HeroLiveGrid`

- Convert `HeroLiveGrid` to a client component (`"use client"`) and make `PATTERN` initial state instead of a const.
- On an interval (default `1800ms`), pick a random cell index and step its intensity along a small cycle (`0 → 1 → 2 → 3 → 2 → 1 → 0`, or just random next-different intensity — pick whatever looks calmer in practice).
- CSS: ensure `.cell` has a `transition: background-color 600ms ease` (check `HeroLiveGrid.module.css`) so the change fades rather than pops.
- Pause the interval when:
  - `document.visibilityState !== "visible"`, OR
  - `prefers-reduced-motion: reduce` is set (in which case render the static pattern, no interval at all).
- Keep `aria-hidden="true"` — purely decorative.
- No prop API changes; the homepage import stays the same.

### 3. Tests (`src/app/page.test.tsx` + new `HeroLiveGrid.test.tsx`)

- Typewriter:
  - Renders with the first phrase's first char(s) after timers advance (use `vi.useFakeTimers()`).
  - Pauses when `name` is non-empty (type into the input → placeholder stops mutating).
  - Pauses when `prefers-reduced-motion` matches (mock `matchMedia`).
- Grid:
  - Renders 21 cells initially matching the seed pattern.
  - After advancing timers, at least one cell's class has changed.
  - With `prefers-reduced-motion`, no mutation occurs across timer advances.

### 4. Manual QA

- 375px viewport: no layout shift, no jank, animations feel calm not twitchy.
- Focus the input → placeholder freezes mid-phrase (or snaps to a stable state — decide which feels better during build).
- macOS "Reduce motion" system setting → both effects disabled, grid shows seed pattern, placeholder stays on first phrase.
- Switch tabs for 30s → no runaway timers, no console noise.

## Open questions

- Final phrase list — keep playful but not cute. Avoid anything that implies a specific use case we don't actually support well.
- Does the placeholder snap to the full current phrase on focus, or freeze mid-type? Snapping is less jarring; freezing is cheaper. Decide during implementation.
- Cadence numbers (`1800ms` grid, `60ms` type) are first guesses — tune by feel.

## Log

- 2026-05-09 — Created plan.
- 2026-05-09 12:15 — Implemented typewriter placeholder hook (`useTypewriterPlaceholder`), animated `HeroLiveGrid` (random cell intensity tick every 1800ms), reduced-motion + tab-hidden + focus/typed pauses, and tests. Placeholder pauses while input is focused or non-empty.
