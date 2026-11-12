import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { StrictMode, useRef } from "react";
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

  it("rehydrate syncs the selection until the first local edit after a seed", () => {
    const { result } = setup();
    // Seeded from a snapshot taken before a save flushed on the previous
    // page's unload landed.
    const stale = cal();
    stale.availability = stale.availability.filter((a) => a.date !== F2);
    act(() => result.current.seed(stale, "me"));
    expect([...result.current.selectedDates]).toEqual([F1]);

    act(() => result.current.rehydrate(cal(), "me"));
    expect([...result.current.selectedDates].sort()).toEqual([F1, F2]);

    // After a local edit, local state wins again.
    act(() => result.current.handleToggle("2099-01-01"));
    act(() => result.current.rehydrate(stale, "me"));
    expect([...result.current.selectedDates].sort()).toEqual(["2099-01-01", F1, F2].sort());
  });

  it("toggle saves the full date set without notes / preferred", async () => {
    const { result, markLocalMutation } = setup();
    act(() => result.current.handleToggle("2099-01-01"));
    await waitFor(() => expect(setAvailability).toHaveBeenCalledTimes(1));
    // keepalive so an in-flight save survives a reload / close.
    expect(setAvailability).toHaveBeenCalledWith(
      "c1",
      { participantId: "me", dates: ["2099-01-01"] },
      { keepalive: true },
    );
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

  it("vote toggle sends the new vote set, not the previous one", async () => {
    const { result } = setup();
    act(() => result.current.seed(cal(), "me"));
    act(() => result.current.handleToggleVote(F2, true));
    await waitFor(() => expect(setAvailability).toHaveBeenCalledTimes(1));
    expect([...setAvailability.mock.calls[0][1].preferred].sort()).toEqual([F1, F2]);
    expect([...result.current.votesByDate].sort()).toEqual([F1, F2]);
  });

  it("vote toggle in the same batch as a refresh saves the new vote set", async () => {
    const { result } = setup();
    act(() => {
      result.current.seed(cal(), "me");
      result.current.handleToggleVote(F2, true);
    });
    await waitFor(() => expect(setAvailability).toHaveBeenCalledTimes(1));
    expect([...setAvailability.mock.calls[0][1].preferred].sort()).toEqual([F1, F2]);
    expect([...result.current.votesByDate].sort()).toEqual([F1, F2]);
  });

  it("rapid double vote toggles end in the right final state", async () => {
    let release: () => void = () => {};
    setAvailability.mockImplementationOnce(
      () =>
        new Promise<void>((r) => {
          release = r;
        }),
    );
    const { result } = setup();
    act(() => result.current.seed(cal(), "me"));
    act(() => {
      result.current.handleToggleVote(F2, true);
      result.current.handleToggleVote(F1, false);
    });
    expect(setAvailability).toHaveBeenCalledTimes(1);
    expect(setAvailability.mock.calls[0][1].preferred).toEqual([F1, F2].sort());
    await act(async () => release());
    await waitFor(() => expect(setAvailability).toHaveBeenCalledTimes(2));
    expect(setAvailability.mock.calls[1][1].preferred).toEqual([F2]);
    expect([...result.current.votesByDate]).toEqual([F2]);
  });

  it("vote toggles save the right set under StrictMode", async () => {
    const markLocalMutation = vi.fn();
    const { result } = renderHook(
      () => {
        const participantIdRef = useRef<string | null>("me");
        return useAvailabilitySave({
          calendarId: "c1",
          participantIdRef,
          finalisedDate: null,
          markLocalMutation,
        });
      },
      { wrapper: StrictMode },
    );
    act(() => result.current.seed(cal(), "me"));
    act(() => {
      result.current.handleToggleVote(F2, true);
      result.current.handleToggleVote(F1, false);
      result.current.handleToggleVote(F1, true);
    });
    await waitFor(() => expect(setAvailability).toHaveBeenCalled());
    const last = setAvailability.mock.calls[setAvailability.mock.calls.length - 1][1];
    expect([...last.preferred].sort()).toEqual([F1, F2]);
    expect([...result.current.votesByDate].sort()).toEqual([F1, F2]);
  });

  it("note edit in the same batch as a refresh keeps the seeded notes", () => {
    const { result } = setup();
    act(() => {
      result.current.seed(cal(), "me");
      result.current.handleSaveNote(F2, "morning");
    });
    expect(result.current.notesByDate.get(F1)).toBe("after 6");
    expect(result.current.notesByDate.get(F2)).toBe("morning");
  });

  it("deselecting a voted date after a vote toggle drops the vote from later saves", async () => {
    const { result } = setup();
    act(() => result.current.seed(cal(), "me"));
    act(() => {
      result.current.handleToggleVote(F2, true);
      result.current.handleToggle(F2);
      result.current.handleToggle(F2);
    });
    await waitFor(() => expect(setAvailability).toHaveBeenCalled());
    await waitFor(() =>
      expect(
        setAvailability.mock.calls[setAvailability.mock.calls.length - 1][1].dates.sort(),
      ).toEqual([F1, F2]),
    );
    expect(result.current.votesByDate.has(F2)).toBe(false);
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

describe("useAvailabilitySave flush on page hide", () => {
  function hold() {
    let release: () => void = () => {};
    setAvailability.mockImplementationOnce(
      () =>
        new Promise<void>((r) => {
          release = r;
        }),
    );
    return () => release();
  }

  function setVisibility(state: DocumentVisibilityState) {
    Object.defineProperty(document, "visibilityState", { configurable: true, value: state });
  }

  it("sends edits queued behind an in-flight save with keepalive on pagehide", async () => {
    const release = hold();
    const { result } = setup();
    act(() => result.current.handleToggle("2099-01-01"));
    act(() => result.current.handleToggle("2099-01-02"));
    act(() => result.current.handleSaveNote("2099-01-02", "late"));
    expect(setAvailability).toHaveBeenCalledTimes(1);

    act(() => {
      window.dispatchEvent(new Event("pagehide"));
    });
    expect(setAvailability).toHaveBeenCalledTimes(2);
    const [cid, body, opts] = setAvailability.mock.calls[1];
    expect(cid).toBe("c1");
    expect(body.dates.sort()).toEqual(["2099-01-01", "2099-01-02"]);
    expect(body.notes).toEqual({ "2099-01-02": "late" });
    expect(opts).toEqual({ keepalive: true });

    // The queue was consumed, so the normal loop doesn't resend it.
    await act(async () => release());
    await new Promise((r) => setTimeout(r, 10));
    expect(setAvailability).toHaveBeenCalledTimes(2);
  });

  it("does not double-send when the in-flight save already carries the latest state", async () => {
    const release = hold();
    const { result } = setup();
    act(() => result.current.handleToggle("2099-01-01"));
    expect(setAvailability).toHaveBeenCalledTimes(1);

    act(() => {
      window.dispatchEvent(new Event("pagehide"));
    });
    expect(setAvailability).toHaveBeenCalledTimes(1);
    await act(async () => release());
  });

  it("sends nothing when there is nothing to save", () => {
    setup();
    act(() => {
      window.dispatchEvent(new Event("pagehide"));
    });
    expect(setAvailability).not.toHaveBeenCalled();
  });

  it("flushes on visibilitychange to hidden, once across hidden + pagehide", async () => {
    const release = hold();
    const { result } = setup();
    act(() => result.current.handleToggle("2099-01-01"));
    act(() => result.current.handleToggleVote("2099-01-01", true));
    try {
      setVisibility("hidden");
      act(() => {
        document.dispatchEvent(new Event("visibilitychange"));
        window.dispatchEvent(new Event("pagehide"));
      });
    } finally {
      setVisibility("visible");
    }
    expect(setAvailability).toHaveBeenCalledTimes(2);
    const [, body, opts] = setAvailability.mock.calls[1];
    expect(body.preferred).toEqual(["2099-01-01"]);
    expect(opts).toEqual({ keepalive: true });
    await act(async () => release());
  });

  it("ignores visibilitychange back to visible", async () => {
    const release = hold();
    const { result } = setup();
    act(() => result.current.handleToggle("2099-01-01"));
    act(() => result.current.handleToggle("2099-01-02"));
    act(() => {
      document.dispatchEvent(new Event("visibilitychange"));
    });
    expect(setAvailability).toHaveBeenCalledTimes(1);
    // The queued edit still goes out, once, through the normal loop.
    await act(async () => release());
    await waitFor(() => expect(setAvailability).toHaveBeenCalledTimes(2));
    await new Promise((r) => setTimeout(r, 10));
    expect(setAvailability).toHaveBeenCalledTimes(2);
  });

  it("with nothing in flight, flushes the queue through the loop", async () => {
    // Pre-join edits stay queued; once the participant exists, a hide sends
    // them right away with keepalive.
    const markLocalMutation = vi.fn();
    const { result } = renderHook(() => {
      const participantIdRef = useRef<string | null>(null);
      const save = useAvailabilitySave({
        calendarId: "c1",
        participantIdRef,
        finalisedDate: null,
        markLocalMutation,
      });
      return { save, participantIdRef };
    });
    act(() => result.current.save.handleToggle("2099-01-01"));
    await new Promise((r) => setTimeout(r, 10));
    expect(setAvailability).not.toHaveBeenCalled();

    result.current.participantIdRef.current = "me";
    act(() => {
      window.dispatchEvent(new Event("pagehide"));
    });
    expect(setAvailability).toHaveBeenCalledTimes(1);
    expect(setAvailability).toHaveBeenCalledWith(
      "c1",
      { participantId: "me", dates: ["2099-01-01"] },
      { keepalive: true },
    );
  });

  it("re-queues and flags an error if the keepalive flush fails while the page lives", async () => {
    const release = hold();
    const { result } = setup();
    act(() => result.current.handleToggle("2099-01-01"));
    act(() => result.current.handleToggle("2099-01-02"));
    setAvailability.mockRejectedValueOnce(new Error("down"));
    await act(async () => {
      window.dispatchEvent(new Event("pagehide"));
    });
    await waitFor(() => expect(result.current.saveError).toBe(true));
    // The in-flight save finishes and the loop picks the re-queued edit up.
    await act(async () => release());
    await waitFor(() => expect(setAvailability).toHaveBeenCalledTimes(3));
    expect(setAvailability.mock.calls[2][1].dates.sort()).toEqual(["2099-01-01", "2099-01-02"]);
  });

  it("removes its listeners on unmount", () => {
    const { result, unmount } = setup();
    const release = hold();
    act(() => result.current.handleToggle("2099-01-01"));
    act(() => result.current.handleToggle("2099-01-02"));
    unmount();
    window.dispatchEvent(new Event("pagehide"));
    expect(setAvailability).toHaveBeenCalledTimes(1);
    release();
  });
});
