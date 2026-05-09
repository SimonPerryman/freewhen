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
- On an interval (default `1800ms`), pick a cell and step its intensity to a different value.
- CSS: ensure `.cell` has a `transition: background-color 600ms ease` (check `HeroLiveGrid.module.css`) so the change fades rather than pops.
- Pause the interval when:
  - `document.visibilityState !== "visible"`, OR
  - `prefers-reduced-motion: reduce` is set (in which case render the static pattern, no interval at all).
- Keep `aria-hidden="true"` — purely decorative.
- No prop API changes; the homepage import stays the same.

#### Calmness rules (learned during first pass)

The naive "pick a random cell, pick a random next intensity" version produced two visible problems on a real screen:

1. **Empty cells (intensity 0) read as bright white in dark mode.** The `fillIn` keyframe's `to` value falls back to a paper-tinted color (`color-mix(... var(--paper) ...)`) when `--target` is unset, which inverts the intent in dark mode. Fix: give intensity-0 cells an explicit dark-mode-aware background that sits close to `var(--card-bg)`, and don't rely on the keyframe fallback.
2. **Random cell picks visually clump** — the same corner can flicker three times in a row.

Mitigations to bake in from the start:

- **Weight `nextIntensity` toward heat.** Roughly: ~10% chance of 0, ~30% each of 1/2/3 (excluding the current value). Empty cells should be uncommon; the grid is meant to feel "filling in," not "blinking on/off."
- **De-clump cell selection.** Track the last N changed cell indices (N≈3) and exclude those *and their immediate neighbours* (±1, ±7 for the 7-wide grid) from the next pick. Falls back to a fully random pick if the exclusion set leaves no candidates.
- **Cadence:** keep `1800ms`. Slowing it further makes the page feel dead; the calmness comes from contrast + de-clumping, not from a slower tick.

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
- 2026-05-09 — On a real dark-mode screen, intensity-0 cells read as bright white (keyframe fallback uses `--paper`) and random picks clumped. Added "Calmness rules" section above so a re-implementation would catch both up front.
- 2026-05-09 — Run 2: implemented calmness rules. CSS now defines `--target` on the base `.cell` (light + dark) so the `fillIn` keyframe's `to` resolves correctly for empty cells in dark mode. Cell picks track the last 3 indices and exclude them plus their ±1/±7 neighbours; intensity choice is weighted (~10% empty, ~30% each of 1/2/3, excluding current).
- 2026-05-09 — Post-merge polish: dynamic week range. `HeroLiveGrid` now computes the visitor's local Mon–Sun on mount and renders it in place of the hardcoded `"Apr 28 — May 4"` label. SSR still emits the fallback string so hydration matches; the effect swaps in the live range immediately. Known limitation: tab kept open across midnight Sunday→Monday won't auto-roll the label until reload — accepted for a marketing hero.
- 2026-05-09 — Post-merge polish: initial pattern shuffle. On mount the seed `PATTERN` is Fisher-Yates shuffled before the tick interval starts, so returning visitors don't see the same canned mockup as their first frame. SSR continues to render the fixed pattern (hydration-safe); the shuffle also runs in reduced-motion mode so those users get a varied still-frame instead of one frozen layout.
