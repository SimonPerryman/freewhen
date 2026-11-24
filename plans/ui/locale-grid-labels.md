# Locale-aware grid labels: month tag, weekday header, first day of week, landing mini-grid

Status: done

## Goal

Finish the "browser locale decides the shape" decision from `plans/bugfix/ui-fixes-date-format.md`. Three pieces of date text still ignore the locale:

1. The landing page's decorative mini-grid builds its week label by hand, month first ("Nov 23 — Nov 29", uppercased by CSS), and its weekday row is a fixed Monday-first "M T W T F S S".
2. The calendar grid's month tag on the 1st of a month (`MONTH_SHORT`, "DEC") is fixed English.
3. The calendar grid's weekday header is a fixed Monday-first "M T W T F S S" ("Mon … Sun" on wide screens).

## Plan

- **New formatters in `src/lib/dates.ts`** (all take the optional `locale`, like the rest):
  - `formatMonthShort(date, locale)`: en-GB "Dec", en-US "Dec", de-DE "Dez.". CSS keeps uppercasing it.
  - `firstDayOfWeek(locale)`: 0 = Sunday … 6 = Saturday, the same numbering as `Date#getDay`. It reads `Intl.Locale#getWeekInfo()` (or the older `weekInfo` getter in Safari 17). When neither exists (Firefox at the time of writing), it falls back to a small table of CLDR's non-Monday regions, keyed on the maximized locale's region (`en` → `en-US` → US → Sunday). Anything else is Monday. The table means the server (Node has `getWeekInfo`) and a browser without it agree, so there's no hydration mismatch.
  - `weekdayLabels(locale, firstDay)`: seven `{ narrow, short }` pairs in display order, e.g. en-US "S M T W T F S" / "Sun Mon …".
  - `startOfWeek(date, firstDay)`: the local Date of the week's first day.
- **Calendar grid** (`CalendarGrid.tsx`):
  - Replace `getMonday` with `startOfWeek(…, firstDayOfWeek(locale))`, so the first row, the 35 visible days, the nav range label and the week paging all start on the locale's first day. en-GB is unchanged; en-US now starts on Sunday.
  - Build the header from `weekdayLabels`, and the month tag from `formatMonthShort`.
  - Reorder the Quick fill `WeekGlyph` strip to match the header, so the filled squares sit under the right columns.
  - "Weekends" stays Saturday + Sunday. Locale weekends (`weekInfo.weekend`, e.g. Fri–Sat) would change what the Quick fill button selects, which is behaviour, not formatting, so it's out of scope.
  - **Scope check for first-day-of-week:** the grid has no arrow-key navigation, and e2e tests pick cells by their order among enabled buttons, which is chronological however the rows start. The grid layout is generated from one start date. So the change is contained, and it ships here instead of as a follow-up.
  - SSR: the locale already comes from `DateLocaleProvider` (Accept-Language), so the server and the hydrating client compute the same first day.
- **Landing mini-grid** (`HeroLiveGrid.tsx`): The landing page (`app/page.tsx`) is a client component that Next prerenders statically at build time. There's no request, so no Accept-Language. Making the page dynamic just to read one header would lose the static prerender for a decorative, `aria-hidden` card. Instead, the date text renders only on the client. A `useSyncExternalStore` "is client" flag is `false` in the server HTML and during hydration, then `true` straight after, and the label and weekday row render in the browser's locale (`locale` undefined = the runtime default). The server HTML has an empty, height-reserved label and an empty weekday row, so nothing mismatches and no wrong-format text flashes. The range uses `formatLocalRange` ("23 – 29 Nov" / "Nov 23 – 29"), so it reads like the calendar grid's nav. The old hard-coded "Apr 28 — May 4" fallback goes away.
- **Range spaces (found while testing):** `formatLocalRange`/`formatLocalDayRange` replace thin spaces (U+2009/U+202F) with plain ones. Node and Chromium's ICU builds disagree on these, which made the server-rendered nav label a hydration mismatch.
- **Tests:**
  - `dates.test.ts` pins en-GB and en-US output for every new formatter, plus de-DE for the month tag. It also covers the `firstDayOfWeek` fallback table, by stubbing `getWeekInfo`/`weekInfo` away.
  - CalendarGrid tests cover the header, the month tag, and a Sunday-first en-US grid through `DateLocaleProvider`.
  - HeroLiveGrid tests cover the localised label and weekday row.

## Log

- 2026-11-24 15:00 — Created plan. Scoped first-day-of-week as contained (see Plan), so it ships in this PR.
- 2026-11-24 15:20 — Added the new formatters to `dates.ts`. Generated the fallback region table from Node's own `getWeekInfo` over every ISO region, and a unit test keeps the two in sync. Wired them into CalendarGrid (header, month tag, week start, Quick fill glyph) and HeroLiveGrid (client-only range and weekday row). `make lint` and `make test` pass (541 tests).
- 2026-11-24 15:40 — 375px shots in en-GB and en-US, light and dark, found a hydration error (React #418) on the calendar page. It was already there from the PR #124 nav range label: Node's ICU puts thin spaces (U+2009) around a range's dash, and Chromium's ICU puts plain spaces. The two range formatters now normalise to plain spaces. After the fix, the console shows no hydration errors on either page, in dev or the production build.
- 2026-11-24 15:50 — Checked the shots. en-GB shows "23 – 29 NOV" and "M T W T F S S" on the landing card, with a Monday-first grid. en-US shows "NOV 22 – 28" and "S M T W T F S", with a Sunday-first grid and "Nov 22 – Dec 26, 2026". The "DEC" tag shows on the 1st. There's no horizontal scroll at 375px. The full e2e suite passes. Status → done.
