import { describe, expect, it } from "vitest";
import {
  FIRST_DAY_BY_REGION,
  firstDayOfWeek,
  formatLocalDate,
  formatLocalDayRange,
  formatLocalRange,
  formatMonthShort,
  startOfWeek,
  weekdayLabels,
  formatLongDay,
  formatShortDay,
  formatWeekdayDay,
  localeFromAcceptLanguage,
} from "./dates";

// Plain spaces around a range's en dash, whatever the ICU build: Node uses
// thin spaces, Chromium plain ones, and SSR text must match hydration.
const TO = " – ";

// Locales are passed explicitly so these pass on any machine. en-GB and en-US
// cover both orders: the point is that the viewer's locale decides the shape.
describe("dates", () => {
  it("formats calendar days in the given locale", () => {
    expect(formatShortDay("2026-12-03", "en-GB")).toBe("Thu 3 Dec");
    expect(formatWeekdayDay("2026-12-12", "en-GB")).toBe("Saturday 12 Dec");
    expect(formatLongDay("2026-12-12", "en-GB")).toBe("Saturday 12 December");

    expect(formatShortDay("2026-12-03", "en-US")).toBe("Thu, Dec 3");
    expect(formatWeekdayDay("2026-12-12", "en-US")).toBe("Saturday, Dec 12");
    expect(formatLongDay("2026-12-12", "en-US")).toBe("Saturday, December 12");
  });

  it("doesn't shift calendar days across time zones", () => {
    // Formatted in UTC from the ISO parts, so midnight boundaries can't move it.
    expect(formatShortDay("2026-01-01", "en-GB")).toBe("Thu 1 Jan");
    expect(formatShortDay("2026-12-31", "en-GB")).toBe("Thu 31 Dec");
  });

  it("returns malformed input unchanged", () => {
    expect(formatShortDay("not-a-date", "en-GB")).toBe("not-a-date");
  });

  it("formats a local date with the year", () => {
    expect(formatLocalDate(new Date(2026, 10, 3), "en-GB")).toBe("3 Nov 2026");
    expect(formatLocalDate(new Date(2026, 10, 3), "en-US")).toBe("Nov 3, 2026");
  });

  it("follows the runtime locale when none is given", () => {
    const expected = new Intl.DateTimeFormat(undefined, {
      weekday: "short",
      day: "numeric",
      month: "short",
      timeZone: "UTC",
    }).format(new Date(Date.UTC(2026, 11, 3)));
    expect(formatShortDay("2026-12-03")).toBe(expected);
  });

  describe("formatLocalRange", () => {
    it("same month", () => {
      expect(formatLocalRange(new Date(2026, 10, 2), new Date(2026, 10, 29), "en-GB")).toBe(
        `2${TO}29 Nov 2026`,
      );
      expect(formatLocalRange(new Date(2026, 10, 2), new Date(2026, 10, 29), "en-US")).toBe(
        `Nov 2${TO}29, 2026`,
      );
    });

    it("across months", () => {
      expect(formatLocalRange(new Date(2026, 10, 23), new Date(2026, 11, 27), "en-GB")).toBe(
        `23 Nov${TO}27 Dec 2026`,
      );
      expect(formatLocalRange(new Date(2026, 10, 23), new Date(2026, 11, 27), "en-US")).toBe(
        `Nov 23${TO}Dec 27, 2026`,
      );
    });

    it("across years", () => {
      expect(formatLocalRange(new Date(2026, 11, 28), new Date(2027, 0, 31), "en-GB")).toBe(
        `28 Dec 2026${TO}31 Jan 2027`,
      );
      expect(formatLocalRange(new Date(2026, 11, 28), new Date(2027, 0, 31), "en-US")).toBe(
        `Dec 28, 2026${TO}Jan 31, 2027`,
      );
    });
  });

  it("formats a short range without the year", () => {
    expect(formatLocalDayRange(new Date(2026, 10, 23), new Date(2026, 10, 29), "en-GB")).toBe(
      `23${TO}29 Nov`,
    );
    expect(formatLocalDayRange(new Date(2026, 10, 23), new Date(2026, 10, 29), "en-US")).toBe(
      `Nov 23${TO}29`,
    );
    expect(formatLocalDayRange(new Date(2026, 10, 30), new Date(2026, 11, 6), "en-GB")).toBe(
      `30 Nov${TO}6 Dec`,
    );
    expect(formatLocalDayRange(new Date(2026, 10, 30), new Date(2026, 11, 6), "en-US")).toBe(
      `Nov 30${TO}Dec 6`,
    );
  });

  it("formats the month tag in the given locale", () => {
    expect(formatMonthShort(new Date(2026, 11, 1), "en-GB")).toBe("Dec");
    expect(formatMonthShort(new Date(2026, 11, 1), "en-US")).toBe("Dec");
    expect(formatMonthShort(new Date(2026, 8, 1), "en-GB")).toBe("Sept");
    expect(formatMonthShort(new Date(2026, 8, 1), "en-US")).toBe("Sep");
    expect(formatMonthShort(new Date(2026, 11, 1), "de-DE")).toBe("Dez");
  });

  describe("week shape", () => {
    it("starts the week on the locale's first day", () => {
      expect(firstDayOfWeek("en-GB")).toBe(1);
      expect(firstDayOfWeek("en-US")).toBe(0);
      expect(firstDayOfWeek("en")).toBe(0); // maximizes to en-US
      expect(firstDayOfWeek("de-DE")).toBe(1);
      expect(firstDayOfWeek("ar-EG")).toBe(6);
      expect(firstDayOfWeek("not_a_tag")).toBe(1);
    });

    it("falls back to CLDR's region table without getWeekInfo", () => {
      const proto = Intl.Locale.prototype as unknown as Record<string, unknown>;
      const restore = Object.getOwnPropertyDescriptor(proto, "getWeekInfo");
      Object.defineProperty(proto, "getWeekInfo", { value: undefined, configurable: true });
      try {
        expect(firstDayOfWeek("en-GB")).toBe(1);
        expect(firstDayOfWeek("en-US")).toBe(0);
        expect(firstDayOfWeek("en")).toBe(0);
        expect(firstDayOfWeek("ar-EG")).toBe(6);
        expect(firstDayOfWeek("dv-MV")).toBe(5);
      } finally {
        if (restore) Object.defineProperty(proto, "getWeekInfo", restore);
        else delete proto.getWeekInfo;
      }
    });

    it("the fallback table matches the runtime's CLDR data", () => {
      type WithWeekInfo = Intl.Locale & { getWeekInfo(): { firstDay: number } };
      for (const [region, day] of FIRST_DAY_BY_REGION) {
        const info = (new Intl.Locale(`und-${region}`) as WithWeekInfo).getWeekInfo();
        expect([region, info.firstDay % 7]).toEqual([region, day]);
      }
    });

    it("finds the start of the week", () => {
      // Wed 25 Nov 2026
      const wed = new Date(2026, 10, 25, 15, 30);
      expect(startOfWeek(wed, 1)).toEqual(new Date(2026, 10, 23));
      expect(startOfWeek(wed, 0)).toEqual(new Date(2026, 10, 22));
      expect(startOfWeek(wed, 6)).toEqual(new Date(2026, 10, 21));
      // A Sunday is the end of a Monday week and the start of a Sunday week.
      expect(startOfWeek(new Date(2026, 10, 29), 1)).toEqual(new Date(2026, 10, 23));
      expect(startOfWeek(new Date(2026, 10, 29), 0)).toEqual(new Date(2026, 10, 29));
    });

    it("labels weekdays in display order", () => {
      const gb = weekdayLabels("en-GB", firstDayOfWeek("en-GB"));
      expect(gb.map((d) => d.narrow).join(" ")).toBe("M T W T F S S");
      expect(gb.map((d) => d.short).join(" ")).toBe("Mon Tue Wed Thu Fri Sat Sun");

      const us = weekdayLabels("en-US", firstDayOfWeek("en-US"));
      expect(us.map((d) => d.narrow).join(" ")).toBe("S M T W T F S");
      expect(us.map((d) => d.short).join(" ")).toBe("Sun Mon Tue Wed Thu Fri Sat");
    });
  });

  describe("localeFromAcceptLanguage", () => {
    it("takes the most preferred locale", () => {
      expect(localeFromAcceptLanguage("en-GB,en;q=0.9")).toBe("en-GB");
      expect(localeFromAcceptLanguage("fr;q=0.5, de-DE")).toBe("de-DE");
    });

    it("skips wildcards and tags Intl rejects", () => {
      expect(localeFromAcceptLanguage("*, not_a_tag, en-US;q=0.8")).toBe("en-US");
    });

    it("is undefined without a usable header", () => {
      expect(localeFromAcceptLanguage(null)).toBeUndefined();
      expect(localeFromAcceptLanguage("")).toBeUndefined();
      expect(localeFromAcceptLanguage("*")).toBeUndefined();
    });
  });
});
