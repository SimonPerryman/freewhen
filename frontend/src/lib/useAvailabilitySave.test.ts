import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useRef } from "react";
import type { CalendarWithDetails } from "./types";

const setAvailability = vi.fn();
const showToast = vi.fn();

vi.mock("./api", () => ({
  setAvailability: (...args: unknown[]) => setAvailability(...args),
}));
vi.mock("./analytics", () => ({ default: { capture: vi.fn() } }));
vi.mock("./toast", () => ({ useToast: () => ({ showToast, dismiss: vi.fn() }) }));

import { localToday } from "./localDate";
import { useAvailabilitySave } from "./useAvailabilitySave";

function daysFromToday(n: number): string {
  const d = new Date();
  d.setDate(d.getDate() + n);
  return localToday(d);
}

const F1 = daysFromToday(3);
const F2 = daysFromToday(6);
const PAST = daysFromToday(-3);

function cal(): CalendarWithDetails {
  return {
    calendar: { id: "c1", name: "Trip", ownerId: "o1", createdAt: "", updatedAt: "" },
    participants: [],
    availability: [
      { participantId: "me", calendarId: "c1", date: F1, note: "after 6", preferred: true },
      { participantId: "me", calendarId: "c1", date: F2 },
      { participantId: "me", calendarId: "c1", date: PAST, note: "old" },
      { participantId: "other", calendarId: "c1", date: F1, note: "theirs" },
    ],
  };
}

function setup(opts: { finalisedDate?: string | null; participantId?: string | null } = {}) {
  const markLocalMutation = vi.fn();
  const hook = renderHook(() => {
    const participantIdRef = useRef<string | null>(
      opts.participantId === undefined ? "me" : opts.participantId,
    );
    return useAvailabilitySave({
      calendarId: "c1",
      participantIdRef,
      finalisedDate: opts.finalisedDate ?? null,
      markLocalMutation,
    });
  });
  return { ...hook, markLocalMutation };
}

beforeEach(() => {
  vi.clearAllMocks();
  setAvailability.mockResolvedValue(undefined);
});

describe("useAvailabilitySave", () => {
  it("seed loads own future dates, notes and votes only", () => {
    const { result } = setup();
    act(() => result.current.seed(cal(), "me"));
    expect([...result.current.selectedDates].sort()).toEqual([F1, F2]);
    expect([...result.current.notesByDate]).toEqual([[F1, "after 6"]]);
    expect([...result.current.votesByDate]).toEqual([F1]);
  });

  it("rehydrate refreshes notes and votes but leaves the selection alone", () => {
    const { result } = setup();
    act(() => result.current.handleToggle("2099-01-01"));
    act(() => result.current.rehydrate(cal(), "me"));
    expect([...result.current.selectedDates]).toEqual(["2099-01-01"]);
    expect(result.current.notesByDate.get(F1)).toBe("after 6");
    expect(result.current.votesByDate.has(F1)).toBe(true);
  });

  it("toggle saves the full date set without notes / preferred", async () => {
    const { result, markLocalMutation } = setup();
    act(() => result.current.handleToggle("2099-01-01"));
    await waitFor(() => expect(setAvailability).toHaveBeenCalledTimes(1));
    expect(setAvailability).toHaveBeenCalledWith("c1", {
      participantId: "me",
      dates: ["2099-01-01"],
    });
    expect(markLocalMutation).toHaveBeenCalled();
  });

  it("serializes saves: edits during an in-flight PUT coalesce into one follow-up", async () => {
    let release: () => void = () => {};
    setAvailability.mockImplementationOnce(
      () =>
        new Promise<void>((r) => {
          release = r;
        }),
    );
    const { result } = setup();
    act(() => result.current.handleToggle("2099-01-01"));
    act(() => result.current.handleToggle("2099-01-02"));
    act(() => result.current.handleSaveNote("2099-01-02", "late"));
    expect(setAvailability).toHaveBeenCalledTimes(1);

    await act(async () => release());
    await waitFor(() => expect(setAvailability).toHaveBeenCalledTimes(2));
    const [, body] = setAvailability.mock.calls[1];
    expect(body.dates.sort()).toEqual(["2099-01-01", "2099-01-02"]);
    expect(body.notes).toEqual({ "2099-01-02": "late" });
  });

  it("vote toggles send `preferred`, limited to selected dates", async () => {
    const { result } = setup();
    act(() => result.current.seed(cal(), "me"));
    act(() => result.current.handleToggleVote("2099-12-31", true));
    await waitFor(() => expect(setAvailability).toHaveBeenCalledTimes(1));
    const [, body] = setAvailability.mock.calls[0];
    expect(Array.isArray(body.preferred)).toBe(true);
    // 2099-12-31 is not a selected date, so it is never sent as preferred.
    expect(body.preferred).not.toContain("2099-12-31");
    for (const d of body.preferred) expect([F1, F2]).toContain(d);
    expect(result.current.votesByDate.has("2099-12-31")).toBe(true);
  });

  it("deselecting drops the note and vote locally", async () => {
    const { result } = setup();
    act(() => result.current.seed(cal(), "me"));
    act(() => result.current.handleToggle(F1));
    expect(result.current.notesByDate.has(F1)).toBe(false);
    expect(result.current.votesByDate.has(F1)).toBe(false);
    await waitFor(() => expect(setAvailability).toHaveBeenCalledTimes(1));
    expect("notes" in setAvailability.mock.calls[0][1]).toBe(false);
  });

  it("range toggle adds and removes in one save", async () => {
    const { result } = setup();
    act(() => result.current.handleRangeToggle(["2099-01-01", "2099-01-02"], true));
    await waitFor(() => expect(setAvailability).toHaveBeenCalledTimes(1));
    act(() => result.current.handleRangeToggle(["2099-01-01"], false));
    await waitFor(() => expect(setAvailability).toHaveBeenCalledTimes(2));
    expect(setAvailability.mock.calls[1][1].dates).toEqual(["2099-01-02"]);
  });

  it("sets saveError on failure and handleRetry re-sends the selection", async () => {
    setAvailability.mockRejectedValueOnce(new Error("down"));
    const { result } = setup();
    act(() => result.current.handleToggle("2099-01-01"));
    await waitFor(() => expect(result.current.saveError).toBe(true));
    act(() => result.current.handleRetry());
    await waitFor(() => expect(result.current.saveError).toBe(false));
    expect(setAvailability).toHaveBeenCalledTimes(2);
    expect(setAvailability.mock.calls[1][1].dates).toEqual(["2099-01-01"]);
  });

  it("does not save before a participant exists", async () => {
    const { result } = setup({ participantId: null });
    act(() => result.current.handleToggle("2099-01-01"));
    await new Promise((r) => setTimeout(r, 10));
    expect(setAvailability).not.toHaveBeenCalled();
  });

  it("clear removes clearable dates, toasts, and undo restores them", async () => {
    const { result } = setup({ finalisedDate: F2 });
    act(() => result.current.seed(cal(), "me"));
    expect(result.current.canClear).toBe(true);

    act(() => result.current.handleClearAvailability());
    // F2 is the finalised date, so only F1 is cleared.
    expect([...result.current.selectedDates]).toEqual([F2]);
    expect(result.current.notesByDate.size).toBe(0);
    expect(showToast).toHaveBeenCalledTimes(1);
    const toast = showToast.mock.calls[0][0];
    expect(toast.message).toBe("Cleared 1 date.");
    await waitFor(() => expect(setAvailability).toHaveBeenCalledTimes(1));

    act(() => toast.action.onClick());
    expect([...result.current.selectedDates].sort()).toEqual([F1, F2]);
    expect(result.current.notesByDate.get(F1)).toBe("after 6");
    await waitFor(() => expect(setAvailability).toHaveBeenCalledTimes(2));
  });

  it("canClear is false with nothing clearable", () => {
    const { result } = setup();
    expect(result.current.canClear).toBe(false);
    act(() => result.current.handleClearAvailability());
    expect(showToast).not.toHaveBeenCalled();
  });
});
