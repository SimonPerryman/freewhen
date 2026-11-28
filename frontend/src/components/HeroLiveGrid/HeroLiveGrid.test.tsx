import { describe, it, expect, vi, afterEach, beforeEach } from "vitest";
import { act, render } from "@testing-library/react";
import { renderToString } from "react-dom/server";
import HeroLiveGrid from "./HeroLiveGrid";

type MediaQueryListener = (event: { matches: boolean }) => void;

function mockMatchMedia(reduced: boolean) {
  const listeners = new Set<MediaQueryListener>();
  const mql = {
    matches: reduced,
    media: "(prefers-reduced-motion: reduce)",
    onchange: null,
    addEventListener: (_: string, cb: MediaQueryListener) => listeners.add(cb),
    removeEventListener: (_: string, cb: MediaQueryListener) => listeners.delete(cb),
    addListener: (cb: MediaQueryListener) => listeners.add(cb),
    removeListener: (cb: MediaQueryListener) => listeners.delete(cb),
    dispatchEvent: () => true,
  };
  window.matchMedia = vi.fn().mockReturnValue(mql) as unknown as typeof window.matchMedia;
  return mql;
}

describe("HeroLiveGrid", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("renders 21 cells", () => {
    mockMatchMedia(false);
    const { container } = render(<HeroLiveGrid />);
    const cells = container.querySelectorAll("[class*='cell']");
    // 21 grid cells; some other class names contain `cell` (e.g. dowCell). Filter to grid cells only.
    const gridCells = container.querySelector("[class*='grid']")!.children;
    expect(gridCells.length).toBe(21);
    expect(cells.length).toBeGreaterThanOrEqual(21);
  });

  it("mutates at least one cell over time", () => {
    mockMatchMedia(false);
    const { container } = render(<HeroLiveGrid />);
    const grid = container.querySelector("[class*='grid']")!;
    const before = Array.from(grid.children).map((c) => c.className);

    act(() => {
      vi.advanceTimersByTime(20_000);
    });

    const after = Array.from(grid.children).map((c) => c.className);
    const changed = before.some((cls, i) => cls !== after[i]);
    expect(changed).toBe(true);
  });

  describe("date text", () => {
    // Wed 25 Nov 2026: the en-GB week is Mon 23–Sun 29, the en-US week Sun 22–Sat 28.
    beforeEach(() => {
      vi.setSystemTime(new Date(2026, 10, 25, 12));
    });

    const weekdays = (c: HTMLElement) =>
      Array.from(c.querySelector("[data-testid='hero-weekdays']")!.children)
        .map((el) => el.textContent)
        .join(" ");

    it("shows this week Monday-first for en-GB", () => {
      mockMatchMedia(true);
      const { container, getByTestId } = render(<HeroLiveGrid locale="en-GB" />);
      expect(getByTestId("hero-range")).toHaveTextContent("23 – 29 Nov");
      expect(weekdays(container)).toBe("M T W T F S S");
    });

    it("shows this week Sunday-first for en-US", () => {
      mockMatchMedia(true);
      const { container, getByTestId } = render(<HeroLiveGrid locale="en-US" />);
      expect(getByTestId("hero-range")).toHaveTextContent("Nov 22 – 28");
      expect(weekdays(container)).toBe("S M T W T F S");
    });

    it("leaves the date text out of the prerendered HTML", () => {
      const html = renderToString(<HeroLiveGrid locale="en-GB" />);
      expect(html).not.toContain("Nov");
      expect(html).not.toMatch(/>M</);
    });
  });

  it("does not mutate when prefers-reduced-motion is set", () => {
    mockMatchMedia(true);
    const { container } = render(<HeroLiveGrid />);
    const grid = container.querySelector("[class*='grid']")!;
    const before = Array.from(grid.children).map((c) => c.className);

    act(() => {
      vi.advanceTimersByTime(20_000);
    });

    const after = Array.from(grid.children).map((c) => c.className);
    expect(after).toEqual(before);
  });
});
