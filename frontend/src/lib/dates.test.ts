import { describe, expect, it } from "vitest";
import {
  formatLocalDate,
  formatLocalRange,
  formatLongDay,
  formatShortDay,
  formatWeekdayDay,
  localeFromAcceptLanguage,
} from "./dates";

// Intl puts thin spaces around a range's en dash.
const TO = " – ";

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
