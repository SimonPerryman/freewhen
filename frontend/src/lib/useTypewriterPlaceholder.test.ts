import { describe, it, expect, vi, afterEach, beforeEach } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { useTypewriterPlaceholder } from "./useTypewriterPlaceholder";

type Listener = (event: { matches: boolean }) => void;

function mockMatchMedia(reduced: boolean) {
  const listeners = new Set<Listener>();
  const mql = {
    matches: reduced,
    media: "(prefers-reduced-motion: reduce)",
    onchange: null,
    addEventListener: (_: string, cb: Listener) => listeners.add(cb),
    removeEventListener: (_: string, cb: Listener) => listeners.delete(cb),
    addListener: (cb: Listener) => listeners.add(cb),
    removeListener: (cb: Listener) => listeners.delete(cb),
    dispatchEvent: () => true,
  };
  window.matchMedia = vi.fn().mockReturnValue(mql) as unknown as typeof window.matchMedia;
  return mql;
}

describe("useTypewriterPlaceholder", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("starts with the first phrase visible", () => {
    mockMatchMedia(false);
    const phrases = ["Alpha", "Beta"];
    const { result } = renderHook(() => useTypewriterPlaceholder(phrases));
    expect(result.current).toBe("Alpha");
  });

  it("eventually changes the placeholder over time", () => {
    mockMatchMedia(false);
    const phrases = ["Alpha", "Beta"];
    const { result } = renderHook(() =>
      useTypewriterPlaceholder(phrases, {
        typeMs: 1,
        holdMs: 1,
        deleteMs: 1,
      }),
    );

    const seen = new Set<string>();
    for (let i = 0; i < 200; i++) {
      act(() => {
        vi.advanceTimersByTime(5);
      });
      seen.add(result.current);
    }
    // Should have observed at least one intermediate state and the next phrase typing in.
    expect(seen.size).toBeGreaterThan(2);
  });

  it("pauses when paused=true", () => {
    mockMatchMedia(false);
    const phrases = ["Alpha", "Beta"];
    const { result } = renderHook(() =>
      useTypewriterPlaceholder(phrases, {
        typeMs: 1,
        holdMs: 1,
        deleteMs: 1,
        paused: true,
      }),
    );

    act(() => {
      vi.advanceTimersByTime(5_000);
    });
    expect(result.current).toBe("Alpha");
  });

  it("does not animate with prefers-reduced-motion", () => {
    mockMatchMedia(true);
    const phrases = ["Alpha", "Beta"];
    const { result } = renderHook(() =>
      useTypewriterPlaceholder(phrases, {
        typeMs: 1,
        holdMs: 1,
        deleteMs: 1,
      }),
    );

    act(() => {
      vi.advanceTimersByTime(5_000);
    });
    expect(result.current).toBe("Alpha");
  });
});
