import { describe, expect, it } from "vitest";
import {
  formatLocalDate,
  formatLocalRange,
  formatLongDay,
  formatShortDay,
  formatWeekdayDay,
} from "./dates";

// Exact strings on purpose: these must not depend on the machine's locale
// (an en-US runner used to get "Thu, Dec 3").
describe("dates", () => {
  it("formats calendar days day-first", () => {
    expect(formatShortDay("2026-12-03")).toBe("Thu 3 Dec");
    expect(formatWeekdayDay("2026-12-12")).toBe("Saturday 12 Dec");
    expect(formatLongDay("2026-12-12")).toBe("Saturday 12 December");
  });

  it("doesn't shift calendar days across time zones", () => {
    // Formatted in UTC from the ISO parts, so midnight boundaries can't move it.
    expect(formatShortDay("2026-01-01")).toBe("Thu 1 Jan");
    expect(formatShortDay("2026-12-31")).toBe("Thu 31 Dec");
  });

  it("returns malformed input unchanged", () => {
    expect(formatShortDay("not-a-date")).toBe("not-a-date");
  });

  it("formats a local date with the year", () => {
    expect(formatLocalDate(new Date(2026, 10, 3))).toBe("3 Nov 2026");
  });

  describe("formatLocalRange", () => {
    it("same month", () => {
      expect(formatLocalRange(new Date(2026, 10, 2), new Date(2026, 10, 29))).toBe(
        "2 – 29 Nov 2026",
      );
    });

    it("across months", () => {
      expect(formatLocalRange(new Date(2026, 10, 23), new Date(2026, 11, 27))).toBe(
        "23 Nov – 27 Dec 2026",
      );
    });

    it("across years", () => {
      expect(formatLocalRange(new Date(2026, 11, 28), new Date(2027, 0, 31))).toBe(
        "28 Dec 2026 – 31 Jan 2027",
      );
    });
  });
});
