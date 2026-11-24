import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { CalendarWithDetails } from "./types";

const updateCalendar = vi.fn();
const showToast = vi.fn();

vi.mock("./api", () => ({
  updateCalendar: (...args: unknown[]) => updateCalendar(...args),
}));
vi.mock("./analytics", () => ({ default: { capture: vi.fn() } }));
vi.mock("./toast", () => ({ useToast: () => ({ showToast, dismiss: vi.fn() }) }));

import { useCalendarEmoji } from "./useCalendarEmoji";

const snapshot: CalendarWithDetails = {
  calendar: { id: "c1", name: "Trip", emoji: "🦊", ownerId: "o1", createdAt: "", updatedAt: "" },
  participants: [],
  availability: [],
};

function setup() {
  const setCalendar = vi.fn();
  const markLocalMutation = vi.fn();
  const { result } = renderHook(() =>
    useCalendarEmoji({ calendarId: "c1", calendar: snapshot, setCalendar, markLocalMutation }),
  );
  // Replays the updaters to get the emoji the page would now show.
  const shownEmoji = () => {
    let state: CalendarWithDetails = snapshot;
    for (const [arg] of setCalendar.mock.calls) {
      state = typeof arg === "function" ? arg(state) : arg;
    }
    return state.calendar.emoji;
  };
  return { setEmoji: result.current, setCalendar, markLocalMutation, shownEmoji };
}

beforeEach(() => vi.clearAllMocks());

describe("useCalendarEmoji", () => {
  it("sets the emoji optimistically and offers Undo", async () => {
    updateCalendar.mockImplementation(async (_id: string, patch: { emoji: string }) => ({
      ...snapshot.calendar,
      emoji: patch.emoji,
    }));
    const { setEmoji, markLocalMutation, shownEmoji } = setup();

    await act(() => setEmoji("📚"));

    expect(updateCalendar).toHaveBeenCalledWith("c1", { emoji: "📚" });
    expect(markLocalMutation).toHaveBeenCalled();
    expect(shownEmoji()).toBe("📚");
    const toast = showToast.mock.calls[0][0];
    expect(toast.action.label).toBe("Undo");

    await act(async () => toast.action.onClick());
    expect(updateCalendar).toHaveBeenLastCalledWith("c1", { emoji: "🦊" });
    expect(shownEmoji()).toBe("🦊");
  });

  it("rolls back and says so when the save fails", async () => {
    updateCalendar.mockRejectedValue(new Error("nope"));
    const { setEmoji, shownEmoji } = setup();

    await act(() => setEmoji("📚"));

    expect(shownEmoji()).toBe("🦊");
    expect(showToast).toHaveBeenCalledTimes(1);
    expect(showToast.mock.calls[0][0].action).toBeUndefined();
  });

  it("does nothing when the emoji is unchanged", async () => {
    const { setEmoji, setCalendar } = setup();
    await act(() => setEmoji("🦊"));
    expect(updateCalendar).not.toHaveBeenCalled();
    expect(setCalendar).not.toHaveBeenCalled();
  });
});
