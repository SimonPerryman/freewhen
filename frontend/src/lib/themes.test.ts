import { describe, expect, it } from "vitest";
import { DEFAULT_THEME, THEMES, isValidTheme, resolveTheme } from "./themes";

describe("themes", () => {
  it("is non-empty", () => {
    expect(THEMES.length).toBeGreaterThan(0);
  });

  it("has unique slugs", () => {
    const slugs = THEMES.map((t) => t.slug);
    expect(new Set(slugs).size).toBe(slugs.length);
  });

  it("includes the default theme", () => {
    expect(isValidTheme(DEFAULT_THEME)).toBe(true);
  });

  it("resolves unknown slugs to default", () => {
    expect(resolveTheme("neon-goblin")).toBe(DEFAULT_THEME);
    expect(resolveTheme(undefined)).toBe(DEFAULT_THEME);
    expect(resolveTheme(null)).toBe(DEFAULT_THEME);
  });

  it("passes through known slugs", () => {
    expect(resolveTheme("ocean")).toBe("ocean");
  });
});
