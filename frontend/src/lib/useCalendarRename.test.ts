import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { CalendarWithDetails } from "./types";

const renameCalendar = vi.fn();

vi.mock("./api", () => ({
  renameCalendar: (...args: unknown[]) => renameCalendar(...args),
}));
vi.mock("./analytics", () => ({ default: { capture: vi.fn() } }));

import { useCalendarRename } from "./useCalendarRename";

const snapshot: CalendarWithDetails = {
  calendar: { id: "c1", name: "Trip", ownerId: "o1", createdAt: "", updatedAt: "" },
  participants: [],
  availability: [],
};

function setup(inputValue: string) {
  const setCalendar = vi.fn();
  const markLocalMutation = vi.fn();
  const hook = renderHook(() =>
    useCalendarRename({ calendarId: "c1", calendar: snapshot, setCalendar, markLocalMutation }),
  );
  hook.result.current.renameInputRef.current = {
    value: inputValue,
    focus: () => {},
    select: () => {},
  } as unknown as HTMLInputElement;
  return { ...hook, setCalendar, markLocalMutation };
}

function applyUpdaters(setCalendar: ReturnType<typeof vi.fn>): CalendarWithDetails {
  let state: CalendarWithDetails = snapshot;
  for (const [arg] of setCalendar.mock.calls) {
    state = typeof arg === "function" ? arg(state) : arg;
  }
  return state;
}

beforeEach(() => vi.clearAllMocks());

describe("useCalendarRename", () => {
  it("start seeds the draft with the current name; cancel clears it", () => {
    const { result } = setup("");
    act(() => result.current.handleRenameStart());
    expect(result.current.renameDraft).toBe("Trip");
    act(() => result.current.handleRenameCancel());
    expect(result.current.renameDraft).toBeNull();
  });

  it("commit is a no-op for blank or unchanged names", async () => {
    for (const value of ["   ", "Trip"]) {
      const { result, setCalendar } = setup(value);
      act(() => result.current.handleRenameStart());
      await act(() => result.current.handleRenameCommit());
      expect(result.current.renameDraft).toBeNull();
      expect(setCalendar).not.toHaveBeenCalled();
    }
    expect(renameCalendar).not.toHaveBeenCalled();
  });

  it("commit renames optimistically and applies the server's name", async () => {
    renameCalendar.mockResolvedValue({ ...snapshot.calendar, name: "Server Name" });
    const { result, setCalendar, markLocalMutation } = setup("  New Name ");
    act(() => result.current.handleRenameStart());
    await act(() => result.current.handleRenameCommit());
    expect(renameCalendar).toHaveBeenCalledWith("c1", "New Name");
    expect(markLocalMutation).toHaveBeenCalled();
    expect(applyUpdaters(setCalendar).calendar.name).toBe("Server Name");
  });

  it("commit rolls back on failure", async () => {
    renameCalendar.mockRejectedValue(new Error("nope"));
    const { result, setCalendar } = setup("New Name");
    await act(() => result.current.handleRenameCommit());
    expect(setCalendar).toHaveBeenLastCalledWith(snapshot);
  });
});
