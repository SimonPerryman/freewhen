import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { CalendarWithDetails, Participant, User } from "./types";

const addParticipant = vi.fn();
const claimParticipant = vi.fn();
const renameParticipant = vi.fn();

vi.mock("./api", async () => {
  const actual = await vi.importActual<typeof import("./api")>("./api");
  return {
    ...actual,
    addParticipant: (...args: unknown[]) => addParticipant(...args),
    claimParticipant: (...args: unknown[]) => claimParticipant(...args),
    renameParticipant: (...args: unknown[]) => renameParticipant(...args),
  };
});
vi.mock("./analytics", () => ({ default: { capture: vi.fn() } }));

import { ApiError } from "./api";
import { MAX_PARTICIPANTS_PER_CALENDAR } from "./limits";
import { useParticipantIdentity } from "./useParticipantIdentity";

const alice: Participant = { id: "p1", calendarId: "c1", name: "Alice", colour: "#f00" };
const bob: Participant = { id: "p2", calendarId: "c1", name: "Bob", colour: "#0f0", userId: "u2" };
const signedIn: User = { id: "u1", email: "u1@example.com", name: "Una" } as User;

function cal(participants: Participant[] = [alice, bob]): CalendarWithDetails {
  return {
    calendar: { id: "c1", name: "Trip", ownerId: "o1", createdAt: "", updatedAt: "" },
    participants,
    availability: [],
  };
}

function setup(user: User | null = null, calendar: CalendarWithDetails | null = cal()) {
  const setCalendar = vi.fn();
  const markLocalMutation = vi.fn();
  const hook = renderHook(() =>
    useParticipantIdentity({ calendarId: "c1", user, calendar, setCalendar, markLocalMutation }),
  );
  return { ...hook, setCalendar, markLocalMutation };
}

function stored(): unknown {
  const raw = localStorage.getItem("freewhen:c1");
  return raw ? JSON.parse(raw) : null;
}

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.clear();
});

describe("useParticipantIdentity", () => {
  it("resolves the stored participant and rewrites the record", async () => {
    localStorage.setItem("freewhen:c1", JSON.stringify({ participantId: "p1", name: "Old" }));
    const { result } = setup();
    let resolved: Participant | null = null;
    await act(async () => {
      resolved = await result.current.resolveFromLoad(cal());
    });
    expect(resolved).toEqual(alice);
    expect(result.current.currentParticipant).toEqual(alice);
    expect(result.current.participantIdRef.current).toBe("p1");
    expect(stored()).toEqual({ participantId: "p1", name: "Alice", colour: "#f00" });
  });

  it("prefers the participant linked to the signed-in user", async () => {
    localStorage.setItem("freewhen:c1", JSON.stringify({ participantId: "p1", name: "Alice" }));
    const { result } = setup({ ...signedIn, id: "u2" });
    await act(async () => {
      await result.current.resolveFromLoad(cal());
    });
    expect(result.current.currentParticipant?.id).toBe("p2");
    expect(claimParticipant).not.toHaveBeenCalled();
  });

  it("silently claims an unlinked stored participant for a signed-in user", async () => {
    localStorage.setItem("freewhen:c1", JSON.stringify({ participantId: "p1", name: "Alice" }));
    claimParticipant.mockResolvedValue({ ...alice, userId: "u1", name: "Alice C" });
    const { result } = setup(signedIn);
    await act(async () => {
      await result.current.resolveFromLoad(cal());
    });
    expect(claimParticipant).toHaveBeenCalledWith("c1", "p1");
    expect(result.current.currentParticipant?.name).toBe("Alice C");
  });

  it("falls back to the stored participant when the claim fails", async () => {
    localStorage.setItem("freewhen:c1", JSON.stringify({ participantId: "p1", name: "Alice" }));
    claimParticipant.mockRejectedValue(new ApiError(409, "linked"));
    const { result } = setup(signedIn);
    await act(async () => {
      await result.current.resolveFromLoad(cal());
    });
    expect(result.current.currentParticipant).toEqual(alice);
  });

  it("prefills the join form: stale stored name, then user name, then last name", async () => {
    localStorage.setItem("freewhen:c1", JSON.stringify({ participantId: "gone", name: "Stale" }));
    localStorage.setItem("freewhen:lastName", "Last");
    const a = setup(signedIn);
    await act(async () => {
      expect(await a.result.current.resolveFromLoad(cal())).toBeNull();
    });
    expect(a.result.current.storedName).toBe("Stale");

    localStorage.removeItem("freewhen:c1");
    const b = setup(signedIn);
    await act(async () => {
      await b.result.current.resolveFromLoad(cal());
    });
    expect(b.result.current.storedName).toBe("Una");

    const c = setup(null);
    await act(async () => {
      await c.result.current.resolveFromLoad(cal());
    });
    expect(c.result.current.storedName).toBe("Last");
  });

  it("join stores the participant + last name and appends to the calendar", async () => {
    addParticipant.mockResolvedValue({ ...alice, id: "p9", name: "Zed" });
    const { result, setCalendar, markLocalMutation } = setup();
    await act(() => result.current.handleJoin("Zed"));
    expect(markLocalMutation).toHaveBeenCalled();
    expect(result.current.currentParticipant?.id).toBe("p9");
    expect(stored()).toEqual({ participantId: "p9", name: "Zed", colour: "#f00" });
    expect(localStorage.getItem("freewhen:lastName")).toBe("Zed");
    const updater = setCalendar.mock.calls[0][0] as (c: CalendarWithDetails) => CalendarWithDetails;
    expect(updater(cal()).participants.map((p) => p.id)).toEqual(["p1", "p2", "p9"]);
  });

  it.each([
    ["calendar_full", `This calendar is full (${MAX_PARTICIPANTS_PER_CALENDAR} people).`],
    ["already_joined", "You've already joined this calendar"],
    [undefined, "That name is already taken"],
  ])("maps a 409 %s to a join error", async (code, message) => {
    addParticipant.mockRejectedValue(new ApiError(409, "conflict", null, code ?? null));
    const { result } = setup();
    await act(() => result.current.handleJoin("Zed"));
    expect(result.current.joinError).toBe(message);
  });

  it("syncFromRefresh picks up our fresh participant record", async () => {
    localStorage.setItem("freewhen:c1", JSON.stringify({ participantId: "p1", name: "Alice" }));
    const { result } = setup();
    expect(result.current.syncFromRefresh(cal())).toBeNull();
    await act(async () => {
      await result.current.resolveFromLoad(cal());
    });
    let id: string | null = null;
    act(() => {
      id = result.current.syncFromRefresh(cal([{ ...alice, name: "Alicia" }, bob]));
    });
    expect(id).toBe("p1");
    expect(result.current.currentParticipant?.name).toBe("Alicia");
  });

  it("renaming yourself updates storage, and rolls back + rethrows on failure", async () => {
    localStorage.setItem("freewhen:c1", JSON.stringify({ participantId: "p1", name: "Alice" }));
    const snapshot = cal();
    const { result, setCalendar } = setup(null, snapshot);
    await act(async () => {
      await result.current.resolveFromLoad(snapshot);
    });

    renameParticipant.mockRejectedValueOnce(new Error("nope"));
    await act(async () => {
      await expect(result.current.handleRenameParticipant(alice, "Ally")).rejects.toThrow("nope");
    });
    expect(setCalendar).toHaveBeenLastCalledWith(snapshot);
    expect(result.current.currentParticipant?.name).toBe("Alice");

    renameParticipant.mockResolvedValueOnce(undefined);
    await act(() => result.current.handleRenameParticipant(alice, "Ally"));
    expect(result.current.currentParticipant?.name).toBe("Ally");
    expect(stored()).toEqual({ participantId: "p1", name: "Ally", colour: "#f00" });
  });
});
