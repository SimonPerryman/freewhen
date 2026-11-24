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
  return plainSpaces(formatter(locale, LOCAL_DATE).formatRange(first, last));
}

// ICU builds disagree on the spaces around a range's dash: Node puts thin
// spaces (U+2009), Chromium plain ones. The calendar page renders on the
// server and hydrates in the browser, so a thin space there is a hydration
// mismatch. Plain spaces everywhere keep the two identical.
function plainSpaces(s: string): string {
  return s.replace(/[  ]/g, " ");
}

const DAY_MONTH: Intl.DateTimeFormatOptions = { day: "numeric", month: "short" };
const MONTH_SHORT: Intl.DateTimeFormatOptions = { month: "short" };
const WEEKDAY_NARROW: Intl.DateTimeFormatOptions = { weekday: "narrow", timeZone: "UTC" };
const WEEKDAY_SHORT: Intl.DateTimeFormatOptions = { weekday: "short", timeZone: "UTC" };

/** A range of local dates without the year: en-GB "23 – 29 Nov", en-US "Nov 23 – 29". */
export function formatLocalDayRange(first: Date, last: Date, locale?: string): string {
  return plainSpaces(formatter(locale, DAY_MONTH).formatRange(first, last));
}

/** The month of a local Date: en-GB and en-US "Dec", de-DE "Dez". */
export function formatMonthShort(date: Date, locale?: string): string {
  return formatter(locale, MONTH_SHORT).format(date);
}

/**
 * Weekday names in display order, starting at `firstDay` (Date#getDay
 * numbering): en-GB from Monday "M T W T F S S" / "Mon … Sun", en-US from
 * Sunday "S M T W T F S" / "Sun … Sat".
 */
export function weekdayLabels(
  locale: string | undefined,
  firstDay: number,
): { narrow: string; short: string }[] {
  return Array.from({ length: 7 }, (_, i) => {
    // 4 Jan 1970 was a Sunday.
    const d = new Date(Date.UTC(1970, 0, 4 + ((firstDay + i) % 7)));
    return {
      narrow: formatter(locale, WEEKDAY_NARROW).format(d),
      short: formatter(locale, WEEKDAY_SHORT).format(d),
    };
  });
}

// Regions whose week doesn't start on Monday, from CLDR's weekData (the data
// Intl.Locale#getWeekInfo reads), in Date#getDay numbering. Only used when the
// runtime lacks getWeekInfo (Firefox), so such a browser still agrees with the
// server, which has it.
const NON_MONDAY_REGIONS: [number, string][] = [
  [5, "MV"],
  [6, "AF BH DJ DZ EG IQ IR JO KW LY OM QA SD SY"],
  [
    0,
    "AG AS BD BR BS BT BW BZ CA CO DM DO ET GT GU HK HN ID IL IN IS JM JP KE KH KR LA MH MM " +
      "MO MT MX MZ NI NP PA PE PH PK PR PT PY SA SG SV TH TT TW UM US VE VI WS YE ZA ZW",
  ],
];
export const FIRST_DAY_BY_REGION: ReadonlyMap<string, number> = new Map(
  NON_MONDAY_REGIONS.flatMap(([day, regions]) => regions.split(" ").map((r) => [r, day] as const)),
);

type WeekInfo = { firstDay: number };
type LocaleWithWeekInfo = Intl.Locale & { getWeekInfo?: () => WeekInfo; weekInfo?: WeekInfo };

/**
 * The first day of the week in `locale`, in Date#getDay numbering
 * (0 = Sunday … 6 = Saturday): en-GB 1 (Monday), en-US 0 (Sunday).
 */
export function firstDayOfWeek(locale?: string): number {
  try {
    const tag = locale ?? new Intl.DateTimeFormat().resolvedOptions().locale;
    const loc = new Intl.Locale(tag) as LocaleWithWeekInfo;
    // getWeekInfo() in current engines; Safari 17 shipped it as a getter.
    const info = typeof loc.getWeekInfo === "function" ? loc.getWeekInfo() : loc.weekInfo;
    if (info && typeof info.firstDay === "number") return info.firstDay % 7; // Intl: Mon 1 … Sun 7
    const region = loc.maximize().region;
    return (region ? FIRST_DAY_BY_REGION.get(region) : undefined) ?? 1;
  } catch {
    return 1; // Malformed tag: Monday.
  }
}

/** The local Date (at midnight) that starts `date`'s week. */
export function startOfWeek(date: Date, firstDay: number): Date {
  const d = new Date(date);
  d.setHours(0, 0, 0, 0);
  d.setDate(d.getDate() - ((d.getDay() - firstDay + 7) % 7));
  return d;
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
