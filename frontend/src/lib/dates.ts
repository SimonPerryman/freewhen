// One day-first date format everywhere ("Thu 3 Dec", "Saturday 12 December"),
// whatever the browser locale. Locale-default formatting (`undefined`) made
// the same date read "Thu, Dec 3" on an en-US machine, so every display
// formatter lives here, pinned to en-GB.

const LOCALE = "en-GB";

// Calendar days ("YYYY-MM-DD") are formatted in UTC so the host time zone
// can't shift them by a day.
const shortDay = new Intl.DateTimeFormat(LOCALE, {
  weekday: "short",
  day: "numeric",
  month: "short",
  timeZone: "UTC",
});
const weekdayDay = new Intl.DateTimeFormat(LOCALE, {
  weekday: "long",
  day: "numeric",
  month: "short",
  timeZone: "UTC",
});
const longDay = new Intl.DateTimeFormat(LOCALE, {
  weekday: "long",
  day: "numeric",
  month: "long",
  timeZone: "UTC",
});
// Local-time variants for instants (timestamps) and local Date objects.
const dayMonthYearLocal = new Intl.DateTimeFormat(LOCALE, {
  day: "numeric",
  month: "short",
  year: "numeric",
});
const dayMonthLocal = new Intl.DateTimeFormat(LOCALE, { day: "numeric", month: "short" });

function parseDay(iso: string): Date | null {
  const [y, m, d] = iso.split("-").map(Number);
  if (!y || !m || !d) return null;
  return new Date(Date.UTC(y, m - 1, d));
}

function withDay(iso: string, fmt: Intl.DateTimeFormat): string {
  const d = parseDay(iso);
  return d ? fmt.format(d) : iso;
}

/** "Thu 3 Dec" */
export function formatShortDay(iso: string): string {
  return withDay(iso, shortDay);
}

/** "Saturday 12 Dec" */
export function formatWeekdayDay(iso: string): string {
  return withDay(iso, weekdayDay);
}

/** "Saturday 12 December" */
export function formatLongDay(iso: string): string {
  return withDay(iso, longDay);
}

/** "3 Dec 2026" for a local Date or an instant, in the viewer's time zone. */
export function formatLocalDate(date: Date): string {
  return dayMonthYearLocal.format(date);
}

/**
 * A range of local dates, day first, with the shared parts said once:
 * "23 – 29 Nov 2026", "23 Nov – 27 Dec 2026", "28 Dec 2026 – 3 Jan 2027".
 */
export function formatLocalRange(first: Date, last: Date): string {
  const fy = first.getFullYear();
  const ly = last.getFullYear();
  if (fy !== ly) {
    return `${dayMonthYearLocal.format(first)} – ${dayMonthYearLocal.format(last)}`;
  }
  if (first.getMonth() !== last.getMonth()) {
    return `${dayMonthLocal.format(first)} – ${dayMonthLocal.format(last)} ${ly}`;
  }
  return `${first.getDate()} – ${dayMonthLocal.format(last)} ${ly}`;
}
