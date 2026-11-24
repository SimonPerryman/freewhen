import { describe, it, expect } from "vitest";
import { buildShareMessage } from "./share";

describe("buildShareMessage", () => {
  it("builds a pre-finalise message without an emoji", () => {
    expect(buildShareMessage({ name: "Weekend trip" })).toBe(
      "Help me find a date for Weekend trip — mark when you're free:",
    );
  });

  it("prepends the emoji with a space when provided", () => {
    expect(buildShareMessage({ name: "Weekend trip", emoji: "🏕️" })).toBe(
      "🏕️ Help me find a date for Weekend trip — mark when you're free:",
    );
  });

  it("builds a finalised-state message with the formatted date", () => {
    expect(
      buildShareMessage({ name: "Weekend trip", finalisedDate: "2026-05-01", locale: "en-GB" }),
    ).toBe("Weekend trip is on Friday 1 May — add it to your calendar:");
  });

  it("combines emoji and finalised date", () => {
    expect(
      buildShareMessage({
        name: "Weekend trip",
        emoji: "🏕️",
        finalisedDate: "2026-05-01",
        locale: "en-GB",
      }),
    ).toBe("🏕️ Weekend trip is on Friday 1 May — add it to your calendar:");
  });

  it("falls back to 'this calendar' when the name is empty or whitespace", () => {
    expect(buildShareMessage({ name: "" })).toBe(
      "Help me find a date for this calendar — mark when you're free:",
    );
    expect(buildShareMessage({ name: "   " })).toBe(
      "Help me find a date for this calendar — mark when you're free:",
    );
  });

  it("ignores null/undefined emoji and finalisedDate", () => {
    expect(buildShareMessage({ name: "Trip", emoji: null, finalisedDate: null })).toBe(
      "Help me find a date for Trip — mark when you're free:",
    );
  });
});
