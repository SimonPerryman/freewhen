import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { CalendarWithDetails, Participant } from "./types";

const finaliseCalendar = vi.fn();
const unfinaliseCalendar = vi.fn();
const getCalendar = vi.fn();
const rotateCalendarPassphrase = vi.fn();
const clearCalendarPassphrase = vi.fn();
const removeParticipant = vi.fn();

vi.mock("./api", async () => {
  const actual = await vi.importActual<typeof import("./api")>("./api");
  return {
    ...actual,
    finaliseCalendar: (...a: unknown[]) => finaliseCalendar(...a),
    unfinaliseCalendar: (...a: unknown[]) => unfinaliseCalendar(...a),
    getCalendar: (...a: unknown[]) => getCalendar(...a),
    rotateCalendarPassphrase: (...a: unknown[]) => rotateCalendarPassphrase(...a),
    clearCalendarPassphrase: (...a: unknown[]) => clearCalendarPassphrase(...a),
    removeParticipant: (...a: unknown[]) => removeParticipant(...a),
  };
});
vi.mock("./analytics", () => ({ default: { capture: vi.fn() } }));

import { ApiError } from "./api";
import { useCalendarOwnerActions } from "./useCalendarOwnerActions";

const alice: Participant = { id: "p1", calendarId: "c1", name: "Alice", colour: "#f00" };
const snapshot: CalendarWithDetails = {
  calendar: { id: "c1", name: "Trip", ownerId: "o1", createdAt: "", updatedAt: "" },
  participants: [alice],
  availability: [{ participantId: "p1", calendarId: "c1", date: "2099-01-01" }],
};

function setup() {
  const setCalendar = vi.fn();
  const markLocalMutation = vi.fn();
  const hook = renderHook(() =>
    useCalendarOwnerActions({
      calendarId: "c1",
      calendar: snapshot,
      setCalendar,
      markLocalMutation,
    }),
  );
  const state = () => {
    let s: CalendarWithDetails = snapshot;
    for (const [arg] of setCalendar.mock.calls) s = typeof arg === "function" ? arg(s) : arg;
    return s;
  };
  return { ...hook, setCalendar, markLocalMutation, state };
}

beforeEach(() => vi.clearAllMocks());

describe("useCalendarOwnerActions", () => {
  it("finalise / unfinalise patch the finalised fields", async () => {
    finaliseCalendar.mockResolvedValue({ finalisedDate: "2099-01-01", finalisedAt: "t" });
    const { result, state, markLocalMutation } = setup();
    await act(() => result.current.handleFinalise("2099-01-01"));
    expect(finaliseCalendar).toHaveBeenCalledWith("c1", "2099-01-01");
    expect(state().calendar.finalisedDate).toBe("2099-01-01");
    expect(markLocalMutation).toHaveBeenCalled();

    unfinaliseCalendar.mockResolvedValue({ finalisedDate: undefined, finalisedAt: undefined });
    await act(() => result.current.handleUnfinalise());
    expect(state().calendar.finalisedDate).toBeUndefined();
  });

  it("finalise refetches on a 409", async () => {
    finaliseCalendar.mockRejectedValue(new ApiError(409, "stale"));
    const fresh = { ...snapshot, calendar: { ...snapshot.calendar, name: "Fresh" } };
    getCalendar.mockResolvedValue(fresh);
    const { result, setCalendar } = setup();
    await act(() => result.current.handleFinalise("2099-01-01"));
    expect(setCalendar).toHaveBeenLastCalledWith(fresh);
  });

  it("enable sets the passphrase straight away", async () => {
    rotateCalendarPassphrase.mockResolvedValue("new-pass");
    const { result, state } = setup();
    await act(() => result.current.handleEnablePassphrase());
    expect(state().calendar).toMatchObject({ hasPassphrase: true, passphrase: "new-pass" });
  });

  it("regenerate and disable go through a confirm dialog", async () => {
    rotateCalendarPassphrase.mockResolvedValue("rotated");
    clearCalendarPassphrase.mockResolvedValue(undefined);
    const { result, state } = setup();

    await act(() => result.current.handleRegeneratePassphrase());
    expect(result.current.regeneratePassphraseOpen).toBe(true);
    expect(rotateCalendarPassphrase).not.toHaveBeenCalled();
    act(() => result.current.cancelRegeneratePassphrase());
    expect(result.current.regeneratePassphraseOpen).toBe(false);

    await act(() => result.current.handleRegeneratePassphrase());
    await act(() => result.current.confirmRegeneratePassphrase());
    expect(result.current.regeneratePassphraseOpen).toBe(false);
    expect(state().calendar.passphrase).toBe("rotated");

    await act(() => result.current.handleDisablePassphrase());
    expect(result.current.removePassphraseOpen).toBe(true);
    await act(() => result.current.confirmDisablePassphrase());
    expect(result.current.removePassphraseOpen).toBe(false);
    expect(state().calendar).toMatchObject({ hasPassphrase: false, passphrase: undefined });
  });

  it("remove participant confirms, removes optimistically, and rolls back on failure", async () => {
    removeParticipant.mockRejectedValue(new Error("nope"));
    const { result, setCalendar } = setup();

    act(() => result.current.handleRemoveParticipant(alice));
    expect(result.current.participantToRemove).toEqual(alice);

    await act(() => result.current.confirmRemoveParticipant());
    expect(result.current.participantToRemove).toBeNull();
    const optimistic = setCalendar.mock.calls[0][0](snapshot) as CalendarWithDetails;
    expect(optimistic.participants).toEqual([]);
    expect(optimistic.availability).toEqual([]);
    expect(removeParticipant).toHaveBeenCalledWith("c1", "p1");
    expect(setCalendar).toHaveBeenLastCalledWith(snapshot);
  });

  it("cancelling remove clears the target", () => {
    const { result } = setup();
    act(() => result.current.handleRemoveParticipant(alice));
    act(() => result.current.cancelRemoveParticipant());
    expect(result.current.participantToRemove).toBeNull();
  });
});
