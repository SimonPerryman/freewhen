import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { CalendarWithDetails } from "./types";

const getCalendar = vi.fn();
const unlockCalendar = vi.fn();
const showToast = vi.fn();
const replace = vi.fn();
const clearCalendarAccessToken = vi.fn();

vi.mock("./api", async () => {
  const actual = await vi.importActual<typeof import("./api")>("./api");
  return {
    ...actual,
    getCalendar: (...args: unknown[]) => getCalendar(...args),
    unlockCalendar: (...args: unknown[]) => unlockCalendar(...args),
  };
});
vi.mock("./calendarAccess", () => ({
  clearCalendarAccessToken: (...args: unknown[]) => clearCalendarAccessToken(...args),
}));
vi.mock("./analytics", () => ({ default: { capture: vi.fn() } }));
vi.mock("./toast", () => ({ useToast: () => ({ showToast, dismiss: vi.fn() }) }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace, push: vi.fn() }) }));

import { ApiError, CalendarUnavailableError } from "./api";
import { LOCAL_MUTATION_SUPPRESS_MS, useCalendarData } from "./useCalendarData";

function cal(name = "Trip"): CalendarWithDetails {
  return {
    calendar: { id: "c1", name, ownerId: "o1", createdAt: "", updatedAt: "" },
    participants: [],
    availability: [],
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  window.history.replaceState({}, "", "/calendars/c1");
});

describe("useCalendarData", () => {
  it("loads the calendar and calls onLoaded before clearing loading", async () => {
    getCalendar.mockResolvedValue(cal());
    let loadingDuringOnLoaded: boolean | null = null;
    const { result } = renderHook(() =>
      useCalendarData("c1", {
        user: null,
        onLoaded: () => {
          loadingDuringOnLoaded = result.current.loading;
        },
      }),
    );
    expect(result.current.loading).toBe(true);
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.calendar?.calendar.name).toBe("Trip");
    expect(loadingDuringOnLoaded).toBe(true);
    expect(getCalendar).toHaveBeenCalledTimes(1);
  });

  it("maps 404 to not-found and other failures to load-failed; retryLoad refetches", async () => {
    getCalendar.mockRejectedValueOnce(new ApiError(404, "nope"));
    const { result } = renderHook(() => useCalendarData("c1", { user: null }));
    await waitFor(() => expect(result.current.error).toBe("not-found"));

    getCalendar.mockRejectedValueOnce(new Error("network"));
    act(() => result.current.retryLoad());
    await waitFor(() => expect(result.current.error).toBe("load-failed"));

    getCalendar.mockResolvedValueOnce(cal());
    act(() => result.current.retryLoad());
    await waitFor(() => expect(result.current.calendar).not.toBeNull());
    expect(result.current.error).toBeNull();
    expect(getCalendar).toHaveBeenCalledTimes(3);
  });

  it("shows the passphrase gate and reloads once unlocked", async () => {
    getCalendar.mockResolvedValueOnce({ ...cal(""), requiresPassphrase: true });
    const onLoaded = vi.fn();
    const { result } = renderHook(() => useCalendarData("c1", { user: null, onLoaded }));
    await waitFor(() => expect(result.current.requiresPassphrase).toBe(true));
    expect(result.current.calendar).toBeNull();
    expect(onLoaded).not.toHaveBeenCalled();

    getCalendar.mockResolvedValueOnce(cal());
    act(() => result.current.handleUnlocked());
    await waitFor(() => expect(result.current.calendar).not.toBeNull());
    expect(result.current.requiresPassphrase).toBe(false);
    expect(onLoaded).toHaveBeenCalledTimes(1);
  });

  it("redeems ?unlock= before fetching and strips it from the URL", async () => {
    window.history.replaceState({}, "", "/calendars/c1?unlock=secret");
    unlockCalendar.mockResolvedValue(undefined);
    getCalendar.mockResolvedValue(cal());
    const { result } = renderHook(() => useCalendarData("c1", { user: null }));
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(unlockCalendar).toHaveBeenCalledWith("c1", "secret");
    expect(unlockCalendar.mock.invocationCallOrder[0]).toBeLessThan(
      getCalendar.mock.invocationCallOrder[0],
    );
    expect(window.location.search).toBe("");
    expect(result.current.magicLinkError).toBeNull();
  });

  it("surfaces a failed magic link", async () => {
    window.history.replaceState({}, "", "/calendars/c1?unlock=bad");
    unlockCalendar.mockRejectedValue(new Error("bad"));
    getCalendar.mockResolvedValue({ ...cal(""), requiresPassphrase: true });
    const { result } = renderHook(() => useCalendarData("c1", { user: null }));
    await waitFor(() => expect(result.current.requiresPassphrase).toBe(true));
    expect(result.current.magicLinkError).toBe("That link is no longer valid.");
  });

  it("redirects when the calendar is unavailable", async () => {
    getCalendar.mockRejectedValue(new CalendarUnavailableError("gone"));
    renderHook(() => useCalendarData("c1", { user: null }));
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/calendars"));
    expect(showToast).toHaveBeenCalledWith({ message: "This calendar is no longer available." });
  });

  it("refresh updates the calendar and calls onRefreshed", async () => {
    getCalendar.mockResolvedValueOnce(cal());
    const onRefreshed = vi.fn();
    const { result } = renderHook(() => useCalendarData("c1", { user: null, onRefreshed }));
    await waitFor(() => expect(result.current.loading).toBe(false));

    getCalendar.mockResolvedValueOnce(cal("Renamed"));
    await act(() => result.current.refresh());
    expect(result.current.calendar?.calendar.name).toBe("Renamed");
    expect(onRefreshed).toHaveBeenCalledTimes(1);
  });

  it("refresh is suppressed right after a local mutation", async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    try {
      getCalendar.mockResolvedValueOnce(cal());
      const { result } = renderHook(() => useCalendarData("c1", { user: null }));
      await waitFor(() => expect(result.current.loading).toBe(false));

      act(() => result.current.markLocalMutation());
      await act(() => result.current.refresh());
      expect(getCalendar).toHaveBeenCalledTimes(1);

      vi.setSystemTime(Date.now() + LOCAL_MUTATION_SUPPRESS_MS + 1);
      getCalendar.mockResolvedValueOnce(cal("Later"));
      await act(() => result.current.refresh());
      expect(getCalendar).toHaveBeenCalledTimes(2);
    } finally {
      vi.useRealTimers();
    }
  });

  it("refresh drops the access token and re-gates when the passphrase changed", async () => {
    getCalendar.mockResolvedValueOnce(cal());
    const { result } = renderHook(() => useCalendarData("c1", { user: null }));
    await waitFor(() => expect(result.current.loading).toBe(false));

    getCalendar.mockResolvedValueOnce({ ...cal(""), requiresPassphrase: true });
    await act(() => result.current.refresh());
    expect(clearCalendarAccessToken).toHaveBeenCalledWith("c1");
    expect(result.current.calendar).toBeNull();
    expect(result.current.requiresPassphrase).toBe(true);
  });
});
