# UI fixes: one day-first date format everywhere

Status: done

## Goal

On My calendars the finalised date followed the browser locale (`toLocaleDateString(undefined, …)`), so an en-US browser showed "Thu, Dec 3" while every other screen said "Thu 3 Dec". The "Updated …" fallback for anything older than 4 weeks had the same bug ("Dec 3, 2026"), and the grid's nav range label and day-cell aria-labels were hand-built month-first ("Nov 23 – Dec 27, 2026"). Owner decision (2026-11-24): one day-first format everywhere, independent of the browser locale.

## Plan

- New `src/lib/dates.ts` holds every display formatter, pinned to `en-GB`. Calendar days (`YYYY-MM-DD`) are formatted in UTC from their parts, so the host time zone can't shift them:
  - `formatShortDay` "Thu 3 Dec": Summary, NotesPill, OG image, My calendars
  - `formatWeekdayDay` "Saturday 12 Dec": BestDay
  - `formatLongDay` "Saturday 12 December": day sheet, Confirmed banner, share text
  - `formatLocalDate` "3 Nov 2026": `formatRelative` fallback, day-cell aria-labels
  - `formatLocalRange` "23 Nov – 27 Dec 2026", "2 – 29 Nov 2026", "28 Dec 2026 – 31 Jan 2027": grid nav label
- Delete the seven per-component `Intl.DateTimeFormat("en-GB")` copies and `relativeTime.formatFinalisedDate`.
- Tests:
  - `dates.test.ts` pins exact strings.
  - `relativeTime.test.ts` pins the fallback ("1 Dec 2025").
  - CalendarGrid tests build cell names with `formatLocalDate`, not a hard-coded month-first pattern.

## Log

- 2026-11-24 13:00 — Created plan
- 2026-11-24 13:10 — Added `lib/dates.ts` and moved all formatters to it. The grid range label and cell aria-labels are day-first now. Tests are pinned, and `src/lib` passes under `LANG=en_US` / `TZ=America/Los_Angeles` too.
- 2026-11-24 13:15 — Shots in Playwright's default en-US locale: My calendars shows "Sat 12 Dec" (was "Sat, Dec 12"), and the nav shows "23 Nov – 27 Dec 2026". `make lint` and `make test` pass.
- Note: en-GB ICU abbreviates September as "Sept". The cell's own month tag (`MONTH_SHORT`) still says "Sep". Left as is.
