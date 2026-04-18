# Calendar Page Layout Reorder

Status: done

## Goal

The calendar page layout had the information hierarchy inverted — summary results and the "Best Day" card appeared above the calendar grid, and the share button was at the top before users had any context. Reorder so the primary interactive element (calendar) comes first, with results and actions flowing naturally after.

## Plan

- Move CalendarGrid above BestDay and SummaryView
- Move ShareButton from the header to below the SummaryView (the decision point after seeing results)
- Clean up header styles that were only needed for the side-by-side title + share layout

## Log

- 2026-04-18 — Created plan
- 2026-04-18 — Reordered layout: Calendar → BestDay → Summary → Share → Participants
- 2026-04-18 — Moved ShareButton out of header, simplified header CSS
