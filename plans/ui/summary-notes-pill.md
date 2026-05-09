# Surface notes in the Summary

Status: done

## Goal

The Summary block now surfaces the *prefer* signal via `PreferencePill` (shipped in #48). The *notes* signal — whose written context might actually decide the day ("after 6pm", "only if we move the dinner") — still dies before reaching the Summary. A reader sees "Days everyone is free" with a preference pill, has no idea three of those days carry caveats, and has to tap each date open to find out.

Add a per-row notes affordance that echoes the preference pill visually (so the row stays cohesive) but tuned to the fact notes have *content*, not just authorship — the popover, not the pill, is where the text lives:

- **n = 0** — nothing rendered.
- **n ≥ 1** — always a count pill (`❝ 1 note` / `❝ 3 notes`), opening a popover of `[swatch] Sam — after 6pm` rows. Tapping a row closes the popover and opens `DayDetailSheet` for that date.

The pill is intentionally count-only at every scale, even n=1 short. Inline preview was prototyped (spike Variant 2) and rejected: at 320px with a prefer pill already on the row, an inline `Sam: after 6pm` shoves the row to a second line, while the count pill keeps the row bounded. Notes earn the action affordance via the popover row → day sheet jump (the asymmetry vs. prefer, which is identity-only).

Design spike: `/tmp/freewhen-summary-notes-pill.html` — three scenarios (1 short, 1 long, 3 notes) plus a 320px row-test. Inline-preview variant (Variant 2) prototyped and rejected; count-only ships at every scale.

## Plan

1. **Wire data through to `SummaryView`.**
   - `CalendarPageClient` already builds `noteAuthorsByDate: Map<string, Set<string>>` (line 1048). Pass it into `SummaryView` as a new optional prop.
   - We also need the note *text* per (date, participant). Build a parallel `notesByDateAndAuthor: Map<string, Map<string, string>>` in `CalendarPageClient` from the same `calendar.availability` walk that produces `noteAuthorsByDate`, plus the current participant's `notesByDate` overlay. Pass both into `SummaryView`.
   - `SummaryView` resolves participant ID → `{ name, colour }` via the existing `participants` prop (same lookup pattern as the preference pill).

2. **Build the pill component.**
   - New folder-per-component: `frontend/src/components/NotesPill/NotesPill.tsx` + `.module.css` + `.test.tsx`.
   - Props: `notes: { id, name, colour, text }[]`, `calendarId?`, `date?`, `onOpenDay?: (date: string) => void`.
   - Renders nothing if `notes.length === 0`.
   - Glyph: `❝` (left double quotation mark, U+201C) as a leading character — distinct from the preference pill's swatch stack so the two pills are scannable side-by-side. Same height, same radius, same `--accent-subtle` family but a different accent token (`--note-accent` / `--note-accent-subtle`) so colour reinforces meaning. Pick tokens that hit 4.5:1 in both themes before merge.
   - Always renders a `<button>` with `aria-haspopup="true"` / `aria-expanded`, label `"${n} note(s) on this day. Show ${n === 1 ? 'it' : 'them'}."`.
   - Visible content: `❝ <n> <note|notes>`. No inline name or text — those live in the popover.

3. **Popover behaviour.**
   - Same machinery as `PreferencePill`: pointerdown-outside dismiss, `Esc` dismiss, focus return on close, non-modal `role="group" aria-label="Notes on this day"`.
   - Each row: `[swatch] <name> — <note text>`. Note text wraps; row is a `<button>` if `onOpenDay` is provided (tapping it closes the popover and calls `onOpenDay(date)`); otherwise a non-interactive `<li>`.
   - Mobile: anchor to the pill, constrain inside card padding at 320px, allow vertical scroll inside the popover when notes are long (cap height ~60vh).
   - Reduced-motion: skip scale/fade.

4. **Integrate into `SummaryView`.**
   - Render in the same `.rowEnd` slot already introduced for the preference pill, after `PreferencePill`. Order in row: date → (count label, mostFree only) → spacer → finalise → preference pill → notes pill.
   - With the count-only pill the row stays bounded at 320px even with both pills + finalise; verify in dogfood.
   - Pass `onOpenDay={openDay}` from `CalendarPageClient` (the same handler `DayDetailSheet` already uses; no new state needed).

5. **Tests.**
   - `NotesPill.test.tsx`:
     - n=0 renders nothing.
     - n=1 renders count pill labelled `1 note`; clicking opens popover with the single note row.
     - n>1 renders count pill labelled `${n} notes`; clicking opens popover; `Esc` and outside pointerdown close it.
     - Clicking a popover row calls `onOpenDay(date)` and closes the popover.
     - Long note text wraps inside the popover row (no truncation).
     - Swatches and the `❝` glyph are `aria-hidden`.
     - Count button has the explicit `aria-label` for both n=1 and n>1.
   - `SummaryView.test.tsx`: extend existing tests with a fixture that has notes — assert pill visible on the right rows, absent on rows without notes, and that both prefer + notes pills can coexist on the same row.

6. **Analytics.**
   - Add `notes_popover_opened` (calendar_id + note_count + date) when the popover opens.
   - Add `notes_popover_row_clicked` (calendar_id + date) when a row is tapped to open the day sheet — distinguishes "users browse the popover" from "users actually act on it" so we know if the row-as-button affordance pays for itself.
   - Mirror the convention used by `PreferencePill` (`analytics.capture` from `@/lib/analytics`).

7. **Decommission decisions.**
   - Keep the calendar-grid note-dot — it serves the at-a-glance scan, same role the vote-dot plays for prefer.
   - `DayDetailSheet` keeps full ownership of note *editing*; the Summary pill is read + jump only.

## Accessibility

- **Popover is not a dialog.** Same shape as `PreferencePill`: `aria-haspopup="true"` + `aria-expanded`, popover content is `role="group" aria-label="Notes on this day"`. No focus trap; focus returns to the pill on close.
- **Swatches are decorative.** All swatches `aria-hidden="true"`. Identity is carried by the name text adjacent to the note.
- **Quote glyph is decorative.** `❝` lives in a `<span aria-hidden="true">`; screen readers shouldn't read "left double quotation mark" before every note.
- **Count pill accessible name.** `aria-label={n === 1 ? "1 note on this day. Show it." : `${n} notes on this day. Show them.`}` (assert in test for both branches).
- **Row buttons inside the popover.** Each is a `<button>` with `aria-label={`Open ${formatDate(date)} to read ${name}'s note`}` so SR users hear the destination, not just the name.
- **No truncation.** Note text wraps inside the popover row — popover has `max-height` with internal scroll for very long notes so nothing is hidden.
- Implementation hygiene: `min-height: 32px` on the button, visible focus ring distinct from hover.

## Out of scope

- Editing notes from the Summary popover. Edit lives in `DayDetailSheet`; the popover row jumps there.
- Threading / replies on a note. The model is one note per (participant, date); that doesn't change.
- Reordering Summary rows by note count.
- A combined "prefer + notes" merged pill. Two pills stay legible; merging would couple two independent signals.

## Open questions

- Should `notes_popover_opened` include note authors? Probably not — author identity isn't an experiment dimension yet, and we'd rather not log participant IDs alongside calendar IDs without a reason. Decide before merge.
- When a row in the popover is tapped, do we close the popover *before* or *after* the day sheet animates in? Default: close immediately, day sheet handles its own transition. Verify on mobile that focus doesn't snap to body between the two.

## Decisions

- **Notes pill is action-bearing; preference pill is identity-only.** Asymmetric on purpose. A reader who sees a preference wants to know *who*; a reader who sees a note wants to *read it / respond*. The popover row → `DayDetailSheet` jump is the whole reason the notes pill earns its weight.
- **Distinct accent colour, not just a different glyph.** Two pills on the same row need to be distinguishable at a glance, not just on inspection. Colour does the work; glyph reinforces.
- **Count-only pill at every n, even n=1 short.** Inline preview was prototyped (spike Variant 2). Rejected: at 320px, with a prefer pill already on the row, even a short `Sam: after 6pm` shoves the row to a second line. The count pill keeps the row bounded; the popover holds the text. Symmetry with `PreferencePill`'s structural rhythm is a happy second-order benefit, not the primary reason.

## Log

- 2026-05-09 — Created plan, mirrored from `plans/ui/summary-preference-pill.md`. Data plumbing already exists (`noteAuthorsByDate` in `CalendarPageClient`); needs a parallel `notesByDateAndAuthor` map for the text payload.
- 2026-05-09 — Spike at `/tmp/freewhen-summary-notes-pill.html` covers three scenarios (1 short, 1 long, 3 notes) plus a 320px row test. Reviewed with Simon: count-only pill wins at every scale. Inline-preview variant rejected because the second line at 320px isn't worth the inline text saving one tap.
- 2026-05-09 — Implemented. Added `--note-accent` / `--note-accent-subtle` tokens (slate-blue, distinct from terra accent). New `NotesPill` component with count-only trigger, `❝` glyph, popover rows that jump to `DayDetailSheet` via `onOpenDay`. `CalendarPageClient` now builds `notesByDateAndAuthor` parallel to `noteAuthorsByDate` and passes it + `handleOpenDay` to `SummaryView`. Analytics: `notes_popover_opened` + `notes_popover_row_clicked`. Tests in `NotesPill.test.tsx` + extended `SummaryView.test.tsx`.
