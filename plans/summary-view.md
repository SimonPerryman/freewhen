# Summary View

Status: done

## Goal

Add a summary section to the calendar view (step 9 of `v1-build-order.md`) that lists dates where everyone is free and dates where most people are free, so users can quickly find the best days without scanning the grid.

## Context

- All data is already available client-side via `freeByDate` (Map<string, Set<string>>) and `calendar.participants`.
- `classifyDay` in `src/lib/overlap.ts` already determines everyone-free status.
- No API changes needed — purely frontend.
- Current page layout: header -> join form -> save error -> calendar grid -> participant list. Summary slots in between grid and participant list.
- Page card is max 540px wide.

## Plan

### 1. Summary logic (`src/lib/summary.ts`)

Pure functions, no rendering:

```ts
interface DateSummary {
  date: string;           // YYYY-MM-DD
  freeCount: number;
  totalCount: number;
}
```

`computeSummary(participants, freeByDate, today)`:
- Filters to future dates only (>= today).
- For each date, counts how many participants are free.
- Returns `{ everyoneFree: DateSummary[], mostFree: DateSummary[] }`.
- `everyoneFree`: dates where `freeCount === totalCount`. Sorted chronologically. (The `< 2 participants` guard lives in the component, not here.)
- `mostFree`: dates where `freeCount >= 2` but not everyone. Sorted by `freeCount` desc, then date asc. Capped at 5 entries to avoid a wall of text.
- No special participant-count guard here — the component handles that by returning `null` when `< 2`.

### 2. Component (`src/components/SummaryView.tsx`)

Props:
```ts
interface SummaryViewProps {
  participants: Participant[];
  freeByDate: Map<string, Set<string>>;
}
```

Component calls `computeSummary`, passing `new Date()` as `today` (kept as a parameter on `computeSummary` for testability).

Rendering:
- Returns `null` when fewer than 2 participants — don't render the section at all.
- Section heading: "Summary".
- **"Days everyone is free"** sub-heading with a list of formatted dates (e.g. "Thu 17 Apr"). If empty, show "No days yet" in secondary text.
- **"Days most people are free"** sub-heading with entries like "Thu 17 Apr — 3/4 free". If empty (because either everyone is free on all dates or no dates have 2+ free), omit this sub-section entirely rather than showing an empty state.
- Date formatting: `Weekday Day Month` (e.g. "Thu 17 Apr") — short, scannable. Use `Intl.DateTimeFormat` with `{ weekday: 'short', day: 'numeric', month: 'short' }`.

Visual style:
- Same card aesthetic as the rest of the page — no box/border of its own, just section within the card.
- Compact list, no bullet points. Each date on its own line.
- Everyone-free dates use the same accent colour as the grid highlight for visual consistency.
- Muted text for the count labels on most-free dates.
- `margin-top: 24px` to separate from the grid above.

### 3. Integration into page

In `page.tsx`, add `<SummaryView>` between `<CalendarGrid>` and `<ParticipantList>`. Pass `participants` and `freeByDate` (same props the grid already receives). Use the `@/` import alias (e.g. `import SummaryView from "@/components/SummaryView"`).

### 4. CSS (`src/components/SummaryView.module.css`)

Minimal styles:
- Section heading matches existing heading sizes.
- Sub-headings smaller, secondary text colour.
- Date list items with small vertical padding.
- Accent colour on everyone-free dates.
- Responsive — stacks naturally at narrow widths (no special handling needed at 540px max).

### 5. Tests

`src/lib/summary.test.ts`:
- 0 or 1 participant: both arrays empty.
- 2 participants, one shared date: appears in `everyoneFree`.
- 3 participants, 2 of 3 share a date: appears in `mostFree`, not `everyoneFree`.
- Past dates excluded.
- `mostFree` sorted by count desc then date asc.
- `mostFree` capped at 5 entries.
- Date where everyone is free does NOT also appear in `mostFree`.

`src/components/SummaryView.test.tsx`:
- Returns null with fewer than 2 participants.
- Renders everyone-free section with formatted dates.
- Renders most-free section with counts.
- Hides most-free sub-section when empty.

### 6. Manual verification

- `make dev-backend` + `make dev-frontend`.
- Calendar with 1 participant: no summary section visible.
- 2 participants, both mark same day: summary shows that day under "everyone free".
- 3 participants, 2 mark same day: shows under "most people free" as "2/3 free".
- All dates in the past: "No days yet" shown.
- Verify accent colour matches grid highlight.

## Decisions

- **Summary between grid and participant list**, not a sidebar. The 540px card is too narrow for a side panel; vertical stacking fits the existing layout.
- **Cap mostFree at 5 entries.** More than that becomes noise — the grid is there for full detail.
- **Omit mostFree sub-section when empty** rather than showing a placeholder. Keeps the section tight.
- **Hide entire component with <2 participants.** Summary of one person's availability is just their availability — no value added.
- **Future dates only.** Past dates aren't actionable.
- **No interactivity.** Clicking a summary date to scroll/highlight the grid cell is a nice V2 enhancement, not V1.
- **Dropped `freeNames` from `DateSummary`.** The component only renders counts ("3/4 free"), not individual names. If we want name display later (e.g. tooltips), we can add it then.

## Log

- 2026-04-17 — Created plan
- 2026-04-17 — Audit fixes: dropped unused `freeNames` from `DateSummary`, moved `< 2 participants` guard to component only, clarified `today` wiring, added `@/` import alias note
- 2026-04-17 — Implemented: summary.ts, SummaryView.tsx + CSS module, integrated into page.tsx, added tests (summary.test.ts + SummaryView.test.tsx). All 66 tests pass, 0 lint errors.
