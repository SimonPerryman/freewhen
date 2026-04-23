# Participant Initials in Cells

Status: done

## Goal

Render each participant's initial inside their cell slot so the calendar grid communicates *who* is free at a glance, not just *how many*. Right now slot colour is the only identity signal — users have to consult `ParticipantList` or hover for the `title` to map colour → person. An initial makes the grid self-describing, especially useful on shared calendars with 3+ participants and on mobile where the participant list sits below the fold.

## Context

- Shipped work this builds on:
  - `plans/v1/visual-overlap.md` — slot grid in `CalendarGrid.tsx`: `layoutSlots(n)` stripes for n ≤ 4, near-square for n ≥ 5, row-major, sorted by palette index.
  - Cell `min-height` scales with `layout.rows`, capped at 120px. Slot `min-height: 4px`; faint slots at `opacity: 0.2`.
  - `aria-label` already lists free/not-free names per cell — accessibility contract is unchanged.
- Participants come from the backend with `name` strings. No initials field is stored; we compute on the frontend.
- Palette colours vary in luminance wildly (`#E9C46A` yellow vs `#264653` near-black) so a single hardcoded text colour won't read against all of them.

## Plan

### 1. Initials helper

New pure function `participantInitial(name: string): string` in `src/lib/overlap.ts` (co-located with other participant-view helpers):

- Trim, take the first grapheme via `Intl.Segmenter` (available in Node 24 and every target browser), uppercase.
- Emoji-safe end-to-end: `Intl.Segmenter` handles ZWJ sequences (👨‍👩‍👧), flags, and skin-tone modifiers correctly — `Array.from(str)[0]` only handles surrogate pairs and would mangle compound emoji.
- Empty / whitespace-only name → empty string (caller skips rendering).
- Single initial only. Two-letter initials ("AR" for "Alex Reed") don't fit in the smallest slot sizes and add visual noise; single letter is the right tradeoff for V1.
- Collisions are accepted: two "Alex"es both render `A`; colour continues to disambiguate. See Decisions.

```ts
export function participantInitial(name: string): string {
  const trimmed = name.trim();
  if (!trimmed) return "";
  const [first] = new Intl.Segmenter().segment(trimmed);
  return (first?.segment ?? "").toUpperCase();
}
```

Unit tests in `overlap.test.ts`: ASCII single name, multi-word name, leading emoji, compound emoji (family ZWJ), empty string, whitespace-only string, leading whitespace.

### 2. Contrast helper

New pure function `textOnColour(hex: string): "#fff" | "#000"` in `src/lib/overlap.ts`:

- Parse `#RRGGBB`, compute relative luminance per WCAG (`0.2126*R + 0.7152*G + 0.0722*B` on sRGB-linearised channels), threshold at 0.5.
- Unit test against the 10 palette colours — lock in the expected pairing (dark palette entries → white, light ones → black) so future palette changes surface here.

### 3. Render initials in slots

In `CalendarGrid.tsx`, inside the slot `<span>`:

- **Free slots**: render the initial as a child, centred, with `color: textOnColour(p.colour)` and `aria-hidden="true"`.
- **Faint (not-free) slots**: render the initial the same way; it inherits the slot's 0.2 opacity, which preserves the "ghosted roster" shape from the visual-overlap plan. Only suppress during manual QA if it muddies adjacent free slots.
- **Empty trailing slots**: no change, no initial.
- **Past-date cells**: already skip slot rendering — nothing to do.

Sizing — single CSS rule, no JS, no layout-tier branching:

- `.slotInitial` gets a fixed small `font-size` (start at `9px`, tune to `8px` during manual QA) and the slot gets `overflow: hidden`. At n ≤ 4 the slot is wide, so the glyph reads large; at n=8 (3×3 grid, ~17px slots on a 375px viewport) it still fits; past n ≈ 12 the glyph starts getting clipped and effectively disappears, which is the right behaviour — colour + participant list do the work in that regime.
- No `clamp()` / `--slot-size` scheme: the grid uses `1fr` tracks, there is no slot-size CSS variable to key off, and container queries add complexity without payoff at this scale.

Slot container: flex-centre children via `.slotGrid > *` rather than adding `display: flex` to both `.slotFree` and `.slotFaint` individually. The initial is a `<span class={styles.slotInitial} aria-hidden="true">` child. Keep `data-testid="slot-${p.id}"` on the outer span so existing tests don't break.

### 4. CSS additions

In `CalendarGrid.module.css`:

- New `.slotInitial` class: `font-size: 9px; font-weight: 600; line-height: 1; pointer-events: none; user-select: none; font-family: inherit;`. No `letter-spacing` — it applies between letters, so it's a no-op on a single glyph.
- `.slotGrid > *` gains `display: flex; align-items: center; justify-content: center; overflow: hidden;` — centres initials and clips them cleanly when the slot shrinks past the font-size.
- No change to `.slotEmpty` (still empty).

### 5. Tests

- `overlap.test.ts`:
  - `participantInitial` cases above.
  - `textOnColour` cases against every palette entry plus edge `#808080`.
- `CalendarGrid.test.tsx`:
  - Use fixtures with distinct first letters (`Alex`, `Bo`, `Cam`, …) so `getByText` is unambiguous.
  - Initial letter appears inside each rendered slot for n=1, n=3, n=9 — scope per slot via `within(screen.getByTestId('slot-${p.id}'))`.
  - Free slot has `color` inline style matching `textOnColour(p.colour)` — assert the computed `color` attribute on the `.slotInitial` element, not the visually-rendered colour (the faint slot's 0.2 opacity would confuse a pixel check).
  - Past-date cells still have no slot container (existing test continues to pass).
  - `aria-label` on the cell button is unchanged and remains the accessibility contract — the glyph is `aria-hidden="true"` so screen readers don't double-announce.

### 6. Manual verification

- `make dev-frontend`. Load a calendar with 1, 3, 8, 12 participants.
- Confirm initials are legible at 375px width for n ≤ 8.
- Confirm palette-dark colours get white text, palette-light get black.
- Confirm faint initials don't compete with free ones.
- Confirm VoiceOver / screen reader still reads the cell `aria-label` and doesn't announce each initial.

## Decisions

- **Single letter, not two-letter initials.** Fits the smallest slots; two-letter adds noise without enough payoff.
- **Initial collisions accepted.** Two "Alex"es both render `A`; colour continues to disambiguate. Any collision-resolution scheme (second character, disambiguating suffix) is scope creep for V1.
- **Emoji-safe via `Intl.Segmenter`.** Handles ZWJ sequences, flags, and skin-tone modifiers correctly. `Array.from(str)[0]` would split compound emoji.
- **Computed WCAG contrast, not a palette lookup table.** Palette may grow; the luminance function keeps working.
- **Faint slots get faint initials**, not "free slots only". Preserves the "full roster shape" invariant from the visual-overlap plan.
- **Fixed `font-size` + `overflow: hidden`, no layout-tier branching.** Single CSS rule handles every n. Glyph naturally disappears past ~n=12 as slots shrink below the font-size — acceptable degradation.
- **`aria-hidden` on the initial.** The cell `aria-label` already names everyone; double-reading is noise.
- **Frontend-only, no API changes.**

## Log

- 2026-04-21 10:00 — Created plan
- 2026-04-21 11:30 — Reviewed plan; switched grapheme split to `Intl.Segmenter`, dropped broken `clamp()` sketch in favour of fixed `font-size` + `overflow: hidden`, removed no-op `letter-spacing`, documented initial-collision decision, tightened test fixtures to avoid duplicate first letters, moved flex-centring to `.slotGrid > *`.
- 2026-04-21 14:15 — Implemented all phases. Added `participantInitial` (Intl.Segmenter-based) and `textOnColour` (WCAG luminance) to `overlap.ts`. Rendered aria-hidden initial spans inside each slot in `CalendarGrid.tsx`. Added `.slotInitial` + `.slotGrid > *` flex-centre + `overflow: hidden` to the CSS module. Extended `overlap.test.ts` with grapheme/emoji cases and locked palette→text pairings (only `#E9C46A` crosses the 0.5 luminance threshold to black — all others render white). Extended `CalendarGrid.test.tsx` with per-slot initial assertions for n=1/3/9 and an aria-hidden + computed-colour check. Full vitest suite passes (176 tests).
