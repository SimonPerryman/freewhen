# UI fixes: one date format everywhere, in the viewer's locale

Status: done

## Goal

Dates used two shapes at once. Most components pinned `en-GB` ("Thu 3 Dec"). My calendars' finalised date and the "Updated …" fallback (anything over 4 weeks old) followed the browser locale (`toLocaleDateString(undefined, …)`), so an en-US browser saw "Thu, Dec 3" next to "Thu 3 Dec". The grid's nav range label and day-cell aria-labels were hand-built month-first ("Nov 23 – Dec 27, 2026").

Owner decision (2026-11-24, "Browser locale, yeah 100%"): the browser locale decides the shape, and every date uses it. An en-GB viewer sees "Thu 3 Dec" and "23 Nov – 27 Dec 2026" everywhere. An en-US viewer sees "Thu, Dec 3" and "Nov 23 – Dec 27, 2026" everywhere.

## Plan

- New `src/lib/dates.ts` holds every display formatter. Each one takes an optional BCP 47 `locale`, where `undefined` means the runtime default, which in the browser is the browser's locale. Calendar days (`YYYY-MM-DD`) are formatted in UTC from their parts, so the host time zone can't shift them. Examples below are en-GB / en-US:
  - `formatShortDay`: "Thu 3 Dec" / "Thu, Dec 3". Used in Summary, NotesPill, the OG image and My calendars.
  - `formatWeekdayDay`: "Saturday 12 Dec" / "Saturday, Dec 12". Used in BestDay.
  - `formatLongDay`: "Saturday 12 December" / "Saturday, December 12". Used in the day sheet, the Confirmed banner and the share text.
  - `formatLocalDate`: "3 Nov 2026" / "Nov 3, 2026". Used for the `formatRelative` fallback and the day-cell aria-labels.
  - `formatLocalRange`: "23 Nov – 27 Dec 2026" / "Nov 23 – Dec 27, 2026". Used for the grid nav label. It uses `Intl.DateTimeFormat#formatRange`, so each locale's own range rules apply.
- **Hydration.** The calendar page is server-rendered, and the server can't see `navigator.language`. `app/calendars/[id]/page.tsx` reads `Accept-Language` (`localeFromAcceptLanguage`) and passes the result to a `DateLocaleProvider` (`lib/dateLocale.tsx`). Components read it with `useDateLocale()`, so the server HTML and the hydrated client format with the same locale. Browsers derive Accept-Language from the same preference list as `navigator.language`. Outside the provider (My calendars, which is client-only), the hook returns `undefined`, so the browser locale applies.
- **OG image.** One image serves every viewer of a shared link, and the crawler that fetches it has no meaningful locale, so it stays on fixed `en-GB`.
- **Share text.** It uses the sharer's locale, because the sharer is the one writing the message.
- Delete the seven per-component `Intl.DateTimeFormat("en-GB")` copies and `relativeTime.formatFinalisedDate`.
- **Tests:**
  - `dates.test.ts` pins exact en-GB and en-US strings, including the thin spaces Intl puts around a range's dash. It also covers Accept-Language parsing and the runtime-default fallback.
  - `dateLocale.test.tsx` covers the provider.
  - `serverCalendar.test.ts` checks that the page passes the request's locale.
  - `vitest.setup.ts` defaults `useDateLocale()` to en-GB for component tests, so assertions like "Thu 3 Dec" don't depend on the machine. Linux CI defaults to en-US.

## Log

- 2026-11-24 13:00 — Created plan
- 2026-11-24 13:10 — Added `lib/dates.ts` and moved all formatters to it, pinned to en-GB. The grid range label and cell aria-labels are day-first.
- 2026-11-24 13:15 — Shots in Playwright's default en-US locale: My calendars shows "Sat 12 Dec" (was "Sat, Dec 12"), and the nav shows "23 Nov – 27 Dec 2026". `make lint` and `make test` pass.
- 2026-11-24 14:00 — The owner clarified the decision: follow the browser locale, consistently, instead of pinning en-GB. Formatters now take a locale. Added `DateLocaleProvider`, which the calendar page fills from Accept-Language so SSR matches hydration. The range label now uses `formatRange`. Tests pin both en-GB and en-US.
- 2026-11-24 14:20 — Captured shots in en-GB and en-US. Every date on the page follows the browser's locale, and there are no hydration errors in the console. `make lint` and `make test` pass (522 tests).
- Not changed: the grid's own month tag on the 1st of a month (`MONTH_SHORT`, "DEC") and the Monday-first M–S weekday header are still fixed English. Making those locale-aware (including first day of week) is a bigger change, so it's left out of this PR.
