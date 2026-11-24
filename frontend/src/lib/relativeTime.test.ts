import { describe, expect, it } from "vitest";
import { formatRelative } from "./relativeTime";

describe("formatRelative", () => {
  const now = new Date("2026-04-20T12:00:00Z");

  it("returns 'just now' for recent timestamps", () => {
    expect(formatRelative("2026-04-20T11:59:50Z", now)).toBe("just now");
  });

  it("formats minutes", () => {
    expect(formatRelative("2026-04-20T11:55:00Z", now)).toBe("5m ago");
  });

  it("formats hours", () => {
    expect(formatRelative("2026-04-20T09:30:00Z", now)).toBe("2h ago");
  });

  it("formats days", () => {
    expect(formatRelative("2026-04-18T12:00:00Z", now)).toBe("2d ago");
  });

  it("formats weeks", () => {
    expect(formatRelative("2026-04-06T12:00:00Z", now)).toBe("2w ago");
  });

  it("falls back to a day-first date for older timestamps", () => {
    expect(formatRelative("2025-12-01T12:00:00Z", now)).toBe("1 Dec 2025");
  });
});
