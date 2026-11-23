# Surface preference in the Summary

Status: done

## Goal

The Summary block currently lists "Days everyone is free" without any signal that one of those days has been preferred by a participant. The preference signal exists (★ vote in `DayDetailSheet`, small dot on the calendar tile) but dies before reaching the Summary, which is the surface most users actually read to make a decision.

Add a per-row pill that shows who preferred each day, sized to the count:

- **n = 1** — static pill, name in-line: `● Sam prefers`
- **n > 1** — interactive pill, count in-line, name list on tap: `●●● 4 prefer` → popover with `[swatch] Sam`, `[swatch] Tess`, etc.
- **n = 0** — no pill rendered

Design spike: a temporary HTML mock, not kept in the repo (variants 1 + 2, conditional popover).

## Plan

1. **Wire data through to `SummaryView`.**
   - `CalendarPageClient` already builds `voteCountsByDate: Map<string, number>` and `preferredByDate: Map<string, Set<string>>` (participant IDs). Pass both into `SummaryView` as new optional props.
   - Resolve participant ID → `{ name, colour }` lookup inside `SummaryView` (via the existing `participants` prop).

2. **Build the pill component.**
   - New folder-per-component: `frontend/src/components/PreferencePill/PreferencePill.tsx` + `.module.css` + `.test.tsx`.
   - Props: `voters: { id, name, colour }[]`. Renders nothing if `voters.length === 0`.
   - n=1: render as a static `<span>` (no role, no cursor).
   - n>1: render as a `<button>` with `aria-haspopup="dialog"` / `aria-expanded`, popover anchored top-right by default and flipped to top-left on narrow viewports.
   - Stacked colour swatches (max ~4 visible, then `+N`).
   - Tokens: `--accent-subtle` background, `--accent` text, swatches use participant colour.

3. **Popover behaviour.**
   - Click/tap to toggle; click outside to dismiss; `Esc` to dismiss; focus returns to the pill.
   - On mobile, render the popover anchored to the pill but constrained inside the card padding (no horizontal overflow at 320px).
   - Reduced-motion path: skip the scale/fade transition.

4. **Integrate into `SummaryView`.**
   - Render the pill at the end of each `everyoneFree` and `mostFree` row, after the existing finalise button slot.
   - Order of items in row: date → (count label, mostFree only) → spacer → finalise → preference pill.
   - Confirm visual hierarchy holds when both finalise + pill are present (likely fine — they're at opposite ends of the row).

5. **Tests.**
   - `PreferencePill.test.tsx`:
     - n=0 renders nothing
     - n=1 renders static pill with the participant name (no button, no popover)
     - n>1 renders button pill; clicking opens popover; clicking outside / pressing Esc closes it
     - swatch colour matches participant colour
   - `SummaryView.test.tsx`: extend existing tests to cover a calendar with votes — assert pill visible on the right row(s), absent on rows without votes.

6. **Analytics.**
   - Existing events: `vote_added`, `vote_removed`, `best_day_shown`. Add `preference_popover_opened` (calendar_id + voter_count) when n>1 popover opens. Mirror the convention used by other surfaces (`analytics.capture` from `@/lib/analytics`).

7. **Decommission decisions.**
   - Keep the calendar-tile vote-dot for now — it serves the at-a-glance grid scan, not the Summary.
   - `BestDay` already references votes as a tiebreaker; no change needed.

## Accessibility

- **Popover is not a dialog.** Drop `aria-haspopup="dialog"`. The button uses `aria-haspopup="true"` + `aria-expanded`, and the popover content is a `<ul role="group" aria-label="People who prefer this day">` with one `<li>` per voter. No focus trap; focus still returns to the pill on close (Esc / outside click).
- **Swatches are decorative.** All colour swatches (stacked dots on the pill, per-row swatches in the popover) get `aria-hidden="true"`. Identity is carried by name text; colour is redundant reinforcement.
- **Count pill accessible name.** The n>1 button uses `aria-label={`${n} people prefer this day. Show who.`}`. Assert this string in `PreferencePill.test.tsx`.
- **Long names.** Truncate the n=1 inline name with `text-overflow: ellipsis` and a `title` attribute carrying the full name. If a name exceeds ~16 chars at 320px, fall through to the n>1 popover path even when n=1 (popover row can wrap). Cover with a test at a long-name fixture.
- Implementation hygiene (not decisions): `min-height: 32px` on the button variant for touch target, visible focus ring distinct from hover, verify `--accent-subtle` / `--accent` pairing hits 4.5:1 in both themes before merge.

## Out of scope

- Reordering the Summary list by votes — date order stays.
- Showing voter notes inside the popover. Notes belong in `DayDetailSheet`; the popover is identity-only.
- Desktop hover-preview on the calendar tile — separate concern.

## Open questions

- Max visible swatches before `+N` — proposed 4, revisit if it looks tight on iPhone SE width.

## Decisions

- **No "See full day →" link inside the popover.** Popover stays identity-only. The date in the row is already the affordance for opening `DayDetailSheet`; a second entry point would anchor two CTAs to the same destination and dilute the popover's one job (answer "who?"). Re-evaluate only if dogfood shows users opening the popover and then immediately tapping the date row.

## Log

- 2026-05-09 — Created plan. Design spike (temporary HTML mock, not kept) covers three variants (named-at-small, count+swatches, no-popover fallback). Decision: ship the conditional rule (static pill at n=1, popover at n>1).
- 2026-05-09 — Added Accessibility section. Resolved: popover is a non-modal `role="group"` (not a dialog), swatches are `aria-hidden`, count button has explicit `aria-label`, long names fall through to the popover path.
- 2026-05-09 — Decided: no "See full day →" link in the popover. Compared side-by-side in the spike; identity-only wins on clarity, and the date row already opens the day sheet.
- 2026-05-09 18:03 — Implemented. New `PreferencePill` component (folder-per-component with module CSS + tests). `SummaryView` accepts `preferredByDate` + `calendarId` and renders the pill in a `.rowEnd` container alongside the finalise button. n=1 short names render as a static inline pill; n>1 (and long-name n=1 fallthrough) renders a button-pill with a non-modal `role="group"` popover, Esc/outside-click dismiss, swatches `aria-hidden`. `analytics.capture("preference_popover_opened", { calendar_id, voter_count, date })` fires on open. Wired in `CalendarPageClient`. Tests + lint green.
