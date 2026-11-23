import { renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import {
  buildFreeByDate,
  buildNoteAuthorsByDate,
  buildNotesByDateAndAuthor,
  buildPreferredByDate,
  buildVoteCountsByDate,
  countResponded,
  respondedParticipantIds,
  useAvailabilityMaps,
} from "./availabilityMaps";

describe("responded", () => {
  it("respondedParticipantIds collects everyone free on any date", () => {
    const ids = respondedParticipantIds(
      new Map([
        ["2026-06-01", new Set(["a", "b"])],
        ["2026-06-02", new Set(["b", "c"])],
      ]),
    );
    expect([...ids].sort()).toEqual(["a", "b", "c"]);
    expect(respondedParticipantIds(new Map()).size).toBe(0);
  });

  it("countResponded ignores joined-but-silent participants and stale rows", () => {
    const participants = [{ id: "owner" }, { id: "a" }, { id: "b" }];
    expect(countResponded(participants, [])).toBe(0);
    expect(
      countResponded(participants, [
        { participantId: "a" },
        { participantId: "a" },
        { participantId: "removed" },
      ]),
    ).toBe(1);
  });
});
import type { CalendarWithDetails, Participant } from "./types";

const me: Participant = { id: "me", calendarId: "c1", name: "Me", colour: "#f00" };

const calendar: CalendarWithDetails = {
  calendar: { id: "c1", name: "Trip", ownerId: "o1", createdAt: "", updatedAt: "" },
  participants: [me, { id: "o", calendarId: "c1", name: "O", colour: "#0f0" }],
  availability: [
    // Server copy of my row — ignored in favour of local state.
    { participantId: "me", calendarId: "c1", date: "2099-01-01", note: "stale", preferred: true },
    { participantId: "o", calendarId: "c1", date: "2099-01-01", note: "hi", preferred: true },
    { participantId: "o", calendarId: "c1", date: "2099-01-02" },
  ],
};

const selected = new Set(["2099-01-02", "2099-01-03"]);
const votes = new Set(["2099-01-03", "2099-01-09"]); // 01-09 not selected → ignored
const notes = new Map([["2099-01-03", "mine"]]);

describe("availabilityMaps", () => {
  it("returns empty maps without a calendar", () => {
    expect(buildFreeByDate(null, me, selected).size).toBe(0);
    expect(buildVoteCountsByDate(null, me, votes, selected).size).toBe(0);
  });

  it("freeByDate layers local selection over everyone else's rows", () => {
    const m = buildFreeByDate(calendar, me, selected);
    expect([...(m.get("2099-01-01") ?? [])]).toEqual(["o"]);
    expect([...(m.get("2099-01-02") ?? [])].sort()).toEqual(["me", "o"]);
    expect([...(m.get("2099-01-03") ?? [])]).toEqual(["me"]);
  });

  it("without a current participant every server row counts", () => {
    const m = buildFreeByDate(calendar, null, selected);
    expect([...(m.get("2099-01-01") ?? [])].sort()).toEqual(["me", "o"]);
    expect(m.has("2099-01-03")).toBe(false);
  });

  it("votes count only on selected dates", () => {
    const counts = buildVoteCountsByDate(calendar, me, votes, selected);
    expect(Object.fromEntries(counts)).toEqual({ "2099-01-01": 1, "2099-01-03": 1 });
    const who = buildPreferredByDate(calendar, me, votes, selected);
    expect([...(who.get("2099-01-03") ?? [])]).toEqual(["me"]);
    expect(who.has("2099-01-09")).toBe(false);
  });

  it("note maps use local notes for me", () => {
    const authors = buildNoteAuthorsByDate(calendar, me, notes);
    expect([...(authors.get("2099-01-01") ?? [])]).toEqual(["o"]);
    expect([...(authors.get("2099-01-03") ?? [])]).toEqual(["me"]);
    const text = buildNotesByDateAndAuthor(calendar, me, notes);
    expect(text.get("2099-01-03")?.get("me")).toBe("mine");
    expect(text.get("2099-01-01")?.get("me")).toBeUndefined();
  });

  it("useAvailabilityMaps memoises on its inputs", () => {
    const own = {
      participant: me,
      selectedDates: selected,
      votesByDate: votes,
      notesByDate: notes,
    };
    const { result, rerender } = renderHook(({ o }) => useAvailabilityMaps(calendar, o), {
      initialProps: { o: own },
    });
    const first = result.current;
    rerender({ o: { ...own } });
    expect(result.current.freeByDate).toBe(first.freeByDate);
    rerender({ o: { ...own, notesByDate: new Map() } });
    expect(result.current.freeByDate).toBe(first.freeByDate);
    expect(result.current.noteAuthorsByDate).not.toBe(first.noteAuthorsByDate);
  });
});
