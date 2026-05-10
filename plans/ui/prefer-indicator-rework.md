# Rework the "preferred day" indicator

Status: done

## Goal

Replace the ★ star badge used to mark preferred dates in the calendar grid with a quieter, more native signal. The star feels imported from a different visual language (favourite/rating/bookmark) and competes with the note dot in a tile that's already carrying date + note + 3 free/busy slots + heat tint.

## Decision

**Use the heat tint as the preference signal.** Preference contributes additional weight to the existing accent-tinted heat the cell already shows. No new glyph, no count, no corner badge, **no inset border** — just a warmer cell.

Detail (who prefers, how many) moves entirely into the `DayDetailSheet`, which is where the user actually decides on a date. The grid's job is *scan*; the sheet's job is *detail*.

### Formula

Each preference vote is worth half a "free" vote of additional warmth, participant-relative, within the existing 14% accent-mix ceiling:

```ts
const freeRatio = sorted.length > 0 ? freeIds.size / sorted.length : 0;
const preferRatio = sorted.length > 0 ? preferCount / sorted.length : 0;
const heatRatio = Math.min(1, freeRatio + 0.5 * preferRatio);
const heatPercent = Math.round(heatRatio * 14); // unchanged ceiling
```

Why:

- **Participant-relative**: one preference in a group of 8 is a smaller nudge than one in a group of 3.
- **Half-weight (additive, flat)**: preferences modulate the availability picture, they don't override it. The `0.5` is the only tuning knob — easy to dial up or down.
- **Same 14% ceiling**: the existing `everyoneFree` style stays the unbeatable top tier so "THE day" still pops.
- **Flat rather than sliding** because predictability matters more than micro-optimisation inside a 0–14% range that's already perceptually narrow. If real-group usage reveals that "preferred but few people free" reads identical to "preferred when many are free", revisit with a multiplicative variant (`heatRatio = freeRatio * (1 + 0.5 * preferRatio)`) so preferences amplify viable days instead of rescuing dead ones.

### What this replaces

- `voteBadge`, `voteBadgeStar`, `voteBadgeCount` in
  `frontend/src/components/CalendarGrid/CalendarGrid.module.css` (lines ~420–441)
- The badge render block in `CalendarGrid.tsx` (currently around lines 530–547)

### Why this works

- Note dot and preference occupy **different visual channels** — corner glyph vs. background warmth — so they stack rather than fight when both are present on the same cell.
- The grid already uses heat tint to mean "good fit" — preference is just a stronger vote, so it's the same channel turned up.
- At production cell scale (~46px wide on mobile, 14px date number) no corner glyph is legible enough to carry a count cleanly; the star + numeral pair was already losing that fight.

## Demo

Interactive HTML POC: [`prefer-indicator-poc.html`](./prefer-indicator-poc.html) — open
in a browser. Shows:

1. A 4-state matrix (`note × preferred`) at 2× zoom so the combinations are easy to inspect
2. A real-mobile-scale (375px viewport) baseline-vs-preferred week strip for each of: current ★ star, heat-only (recommended), bottom stripe, italic serif date
3. Light/dark toggle, real Paper/Ink tokens, Fraunces + DM Sans + Instrument Serif

A bottom-stripe variant is documented in the POC as the fallback if heat-only proves too subtle in usage.

## Plan

1. Update the heat calculation in `CalendarGrid.tsx` (around line 444) to fold in preferences using the formula above. `preferCount` should come from the existing `voteCountsByDate` map.
2. Remove `voteBadge` / `voteBadgeStar` / `voteBadgeCount` from `CalendarGrid.tsx` (render block ~530–547) and the matching styles from `CalendarGrid.module.css` (~420–441).
3. Update the cell's `aria-label` to include "preferred by N" alongside the existing free/busy text, so screen reader users don't lose the signal that was previously carried by the badge.
4. Confirm the `DayDetailSheet` still surfaces "who prefers this" prominently — that becomes the only place preference is enumerated. (`DayDetailSheet.tsx` line 224 already renders a per-person `preferStar`; revisit whether that should also lose the star glyph in favour of a quieter typographic treatment, but treat as a separate follow-up rather than blocking this change.)
5. Update / remove tests in `CalendarGrid.test.tsx` that assert on the vote badge DOM (search for `vote-badge`). Add a test that asserts the heat formula combines free + preference counts correctly.
6. Visual QA on the calendar page at 375px and desktop widths, light + dark — particularly for a day with `preferred ≥ 1` and a note attached (the matrix case in the POC).

## Log

- 2026-05-10 23:30 — Created plan after iterating on `prefer-indicator-poc.html`. Picked heat-only after comparing star (current), heat-only, bottom stripe, italic serif date at real mobile scale. State matrix confirmed note dot + heat tint don't conflict.
- 2026-05-10 23:55 — Locked the heat formula: flat additive, `+0.5 * preferRatio` within the existing 14% ceiling. Dropped the optional inset border — if the tint isn't loud enough, tune the multiplier rather than adding decoration. Removed the open-questions section now that both are answered.
- 2026-05-10 23:47 — Implemented. Folded preference into the heat tint via a new `computeHeatPercent(participants, free, prefer)` helper in `CalendarGrid.tsx`; ripped out the `voteBadge` / `voteBadgeStar` / `voteBadgeCount` render block + matching CSS; threaded `preferCount` into `buildAriaLabel` so screen reader users still hear "Preferred by N". Replaced the vote-badge test block with a direct unit test of the formula (covers zero participants, free-only, free + prefer half-weight, prefer-only nudge, and the 14% ceiling) plus assertions that the legacy badge is gone and the aria-label carries the count. `make lint` + `make test` green.
