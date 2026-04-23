# Summary card on desktop

Status: done

## Goal

On the calendar page, the Summary + Share block currently sits plainly under
the grid. On desktop, where the Share button shrinks to a 260px chip, the
block loses visual weight and does not "pop". Wrap Summary + Share in a
bordered card so the block reads as one deliberate unit on all viewports.
Keep ParticipantList expanded below (it already adapts: compact toggle on
mobile, full list on desktop).

## Plan

- Add `.summaryCard` in `app/calendars/[id]/page.module.css` — card-bg,
  border, rounded, shadow, padding.
- In `CalendarPageClient.tsx` wrap `<SummaryView>` + `<ShareButton>` in the
  new card when both render (2+ participants, not finalised). Use the
  `hero` variant on Share so it fills the card width instead of the
  desktop 260px cap.
- Drop the `margin-top: 24px` on `SummaryView .section` — card padding
  handles spacing now.

## Log

- 2026-04-20 — Created plan.
- 2026-04-20 — Added `.summaryCard` wrapper in page CSS; restructured
  `CalendarPageClient` rendering into three explicit branches (1 pt →
  InvitePrompt, 2+ pts → card + participants, finalised → share +
  participants). Share inside the card uses `hero` variant so it fills
  the width. Removed `SummaryView .section` top margin; zeroed the last
  child's bottom margin. Existing tests unchanged (same pass/fail set
  as main).
