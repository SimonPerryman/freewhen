# Past Days Visibility

Status: done

## Goal

Past days in the calendar grid (e.g. Mon–Fri of the current week when today is Saturday) currently render as cells that look nearly identical to "future days where nobody is free yet": same border, same background, same date-number weight, just a marginally dimmer number. Users can't tell they're disabled — the five leading cells read as tappable-but-empty.

Fix this so the past is visually quiet without losing the Mon–Sun column alignment users rely on.

## Options considered

### Option A — Empty gap (chosen)

Keep the Mon–Sun header and 7-column grid. Render past days in their correct grid positions but with no border, no date number, no background — just transparent placeholders holding the column. Today remains in its correct weekday column.

Row 1 when today is Sat 18 Apr:

```
M    T    W    T    F    [18]  [19]
```

- Pros: column alignment preserved, Mon–Sun muscle memory preserved, the leading gap itself becomes the signal of "the past" — no explicit disabled styling needed. Removes visual noise from the most prominent row. Cheapest diff.
- Cons: row 1 looks asymmetric the first time a user sees it (though that asymmetry *is* the signal we want).

### Option B — Loud disabled state

Keep cells visible but style them much more aggressively as disabled: ~30% opacity, remove border, strikethrough the date number.

- Pros: nothing structurally changes; cells stay where they are.
- Cons: still puts five chunks of visual weight in the top row for information the user can't act on; adds dark-theme-specific styling work; competes for attention with today's cell.

**Decision: Option A.** The goal is to stop the eye landing on dead cells — hiding them achieves that better than shouting "disabled" at them.

## Plan

- In `frontend/src/components/CalendarGrid/CalendarGrid.tsx`, when `isPast` is true, render a placeholder element (e.g. an empty `<div>` or `<button>` with `aria-hidden` and no visible chrome) that holds the grid column but has no border, background, date number, or slot grid.
- In `frontend/src/components/CalendarGrid/CalendarGrid.module.css`, replace the existing `.past` styles with rules that strip border, background, and inner content styling. Make sure `min-height` still matches the rest of row 1 so row height is consistent.
- Verify the range label in the nav (e.g. "Apr 13 – May 17, 2026") still reflects the *visible* window conceptually — decide whether to keep showing the Monday boundary or shift the label to start on today. Leaning: keep the Monday boundary so the label matches the grid structure, but revisit if it reads confusingly.
- Check the entrance animation stagger (`--entrance-delay`) — past placeholders should probably not animate in, to avoid drawing attention to the gap.
- Update `CalendarGrid.test.tsx` — any test that asserts on past-cell content or clickability should move to asserting the placeholder has no date number and is not a button (or is `aria-hidden`).
- Manual QA at 375px mobile width: row 1 should look like a short row starting at today's weekday column, with clean left-edge negative space.

## Log

- 2026-04-18 — Created plan; chose Option A (empty gap) over Option B (loud disabled state)
- 2026-04-18 — Implemented: past cells now render as aria-hidden `<div>` placeholders with transparent background, no border, no shadow. Removed `.past` CSS; added `.placeholder`. Reset entrance-delay counter to start from the first visible cell so today animates in at 0ms instead of after a leading pause. Updated CalendarGrid tests to count grid positions via `data-testid="day-cell"` and to assert placeholder semantics (div tag, aria-hidden, no children). All 81 frontend tests pass; lint clean.
