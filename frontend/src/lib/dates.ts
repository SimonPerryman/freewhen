// Every display date goes through these formatters, so a date has one shape
// across the app, and that shape is the viewer's: "Thu 3 Dec" for en-GB,
// "Thu, Dec 3" for en-US, "Do., 3. Dez." for de-DE.
//
// `locale` is a BCP 47 tag. Components pass `useDateLocale()` (lib/dateLocale)
// so server-rendered text matches what hydrates in the browser; `undefined`
// means the runtime's default locale, which in the browser is the browser's.

// Calendar days ("YYYY-MM-DD") are formatted in UTC so the host time zone
// can't shift them by a day.
const SHORT_DAY: Intl.DateTimeFormatOptions = {
  weekday: "short",
  day: "numeric",
  month: "short",
  timeZone: "UTC",
};
const WEEKDAY_DAY: Intl.DateTimeFormatOptions = {
  weekday: "long",
  day: "numeric",
  month: "short",
  timeZone: "UTC",
};
const LONG_DAY: Intl.DateTimeFormatOptions = {
  weekday: "long",
  day: "numeric",
  month: "long",
  timeZone: "UTC",
};
// Local-time variant for instants (timestamps) and local Date objects.
const LOCAL_DATE: Intl.DateTimeFormatOptions = { day: "numeric", month: "short", year: "numeric" };

const cache = new Map<string, Intl.DateTimeFormat>();

function formatter(locale: string | undefined, options: Intl.DateTimeFormatOptions) {
  const key = `${locale ?? ""}|${JSON.stringify(options)}`;
  let f = cache.get(key);
  if (!f) {
    f = new Intl.DateTimeFormat(locale, options);
    cache.set(key, f);
  }
  return f;
}

function parseDay(iso: string): Date | null {
  const [y, m, d] = iso.split("-").map(Number);
  if (!y || !m || !d) return null;
  return new Date(Date.UTC(y, m - 1, d));
}

function withDay(iso: string, locale: string | undefined, options: Intl.DateTimeFormatOptions) {
  const d = parseDay(iso);
  return d ? formatter(locale, options).format(d) : iso;
}

/** en-GB "Thu 3 Dec", en-US "Thu, Dec 3" */
export function formatShortDay(iso: string, locale?: string): string {
  return withDay(iso, locale, SHORT_DAY);
}

/** en-GB "Saturday 12 Dec", en-US "Saturday, Dec 12" */
export function formatWeekdayDay(iso: string, locale?: string): string {
  return withDay(iso, locale, WEEKDAY_DAY);
}

/** en-GB "Saturday 12 December", en-US "Saturday, December 12" */
export function formatLongDay(iso: string, locale?: string): string {
  return withDay(iso, locale, LONG_DAY);
}

/** en-GB "3 Nov 2026", en-US "Nov 3, 2026": a local Date or an instant, in the viewer's time zone. */
export function formatLocalDate(date: Date, locale?: string): string {
  return formatter(locale, LOCAL_DATE).format(date);
}

/**
 * A range of local dates with the shared parts said once, in the locale's
 * own order: en-GB "23 Nov – 27 Dec 2026", en-US "Nov 23 – Dec 27, 2026".
 */
export function formatLocalRange(first: Date, last: Date, locale?: string): string {
  return formatter(locale, LOCAL_DATE).formatRange(first, last);
}

/**
 * The preferred locale from an Accept-Language header that Intl can format
 * dates in, or undefined. "en-GB,en;q=0.9" → "en-GB".
 */
export function localeFromAcceptLanguage(header: string | null | undefined): string | undefined {
  if (!header) return undefined;
  const ranked = header
    .split(",")
    .map((part, i) => {
      const [tag, ...params] = part.trim().split(";");
      const q = params.map((p) => p.trim()).find((p) => p.startsWith("q="));
      return { tag: tag.trim(), q: q ? Number(q.slice(2)) : 1, i };
    })
    .filter((r) => r.tag && r.tag !== "*" && r.q > 0)
    .sort((a, b) => b.q - a.q || a.i - b.i);
  for (const { tag } of ranked) {
    try {
      if (Intl.DateTimeFormat.supportedLocalesOf([tag]).length > 0) return tag;
    } catch {
      // Malformed tag; try the next one.
    }
  }
  return undefined;
}
