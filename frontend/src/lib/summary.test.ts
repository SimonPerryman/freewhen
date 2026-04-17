import { describe, it, expect } from "vitest";
import { computeSummary } from "./summary";
import type { Participant } from "./types";

function makeParticipant(id: string): Participant {
  return { id, calendarId: "cal-1", name: `P-${id}`, colour: "#000" };
}

function makeDate(daysFromToday: number): string {
  const d = new Date(2025, 5, 15); // fixed "today" for tests
  d.setDate(d.getDate() + daysFromToday);
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

const today = new Date(2025, 5, 15); // 2025-06-15

describe("computeSummary", () => {
  it("returns empty arrays with 0 participants", () => {
    const result = computeSummary([], new Map(), today);
    expect(result.everyoneFree).toEqual([]);
    expect(result.mostFree).toEqual([]);
  });

  it("returns empty arrays with 1 participant", () => {
    const p1 = makeParticipant("a");
    const freeByDate = new Map([[makeDate(1), new Set(["a"])]]);
    const result = computeSummary([p1], freeByDate, today);
    // With 1 participant, freeCount (1) === totalCount (1) so it goes to everyoneFree,
    // but mostFree requires freeCount >= 2, so that's empty.
    // everyoneFree includes it since 1 === 1.
    expect(result.everyoneFree).toHaveLength(1);
    expect(result.mostFree).toEqual([]);
  });

  it("puts shared date in everyoneFree for 2 participants", () => {
    const p1 = makeParticipant("a");
    const p2 = makeParticipant("b");
    const date = makeDate(1);
    const freeByDate = new Map([[date, new Set(["a", "b"])]]);
    const result = computeSummary([p1, p2], freeByDate, today);
    expect(result.everyoneFree).toEqual([{ date, freeCount: 2, totalCount: 2 }]);
    expect(result.mostFree).toEqual([]);
  });

  it("puts partial overlap in mostFree for 3 participants", () => {
    const participants = [makeParticipant("a"), makeParticipant("b"), makeParticipant("c")];
    const date = makeDate(2);
    const freeByDate = new Map([[date, new Set(["a", "b"])]]);
    const result = computeSummary(participants, freeByDate, today);
    expect(result.everyoneFree).toEqual([]);
    expect(result.mostFree).toEqual([{ date, freeCount: 2, totalCount: 3 }]);
  });

  it("excludes past dates", () => {
    const participants = [makeParticipant("a"), makeParticipant("b")];
    const pastDate = makeDate(-1);
    const futureDate = makeDate(1);
    const freeByDate = new Map([
      [pastDate, new Set(["a", "b"])],
      [futureDate, new Set(["a", "b"])],
    ]);
    const result = computeSummary(participants, freeByDate, today);
    expect(result.everyoneFree).toHaveLength(1);
    expect(result.everyoneFree[0].date).toBe(futureDate);
  });

  it("includes today's date", () => {
    const participants = [makeParticipant("a"), makeParticipant("b")];
    const todayStr = makeDate(0);
    const freeByDate = new Map([[todayStr, new Set(["a", "b"])]]);
    const result = computeSummary(participants, freeByDate, today);
    expect(result.everyoneFree).toHaveLength(1);
  });

  it("sorts mostFree by count desc then date asc", () => {
    const participants = [
      makeParticipant("a"),
      makeParticipant("b"),
      makeParticipant("c"),
      makeParticipant("d"),
    ];
    const d1 = makeDate(1);
    const d2 = makeDate(2);
    const d3 = makeDate(3);
    const freeByDate = new Map([
      [d1, new Set(["a", "b"])],        // 2/4
      [d2, new Set(["a", "b", "c"])],   // 3/4
      [d3, new Set(["a", "b"])],        // 2/4
    ]);
    const result = computeSummary(participants, freeByDate, today);
    expect(result.mostFree.map((d) => d.date)).toEqual([d2, d1, d3]);
  });

  it("caps mostFree at 5 entries", () => {
    const participants = [
      makeParticipant("a"),
      makeParticipant("b"),
      makeParticipant("c"),
    ];
    const freeByDate = new Map<string, Set<string>>();
    for (let i = 1; i <= 8; i++) {
      freeByDate.set(makeDate(i), new Set(["a", "b"])); // 2/3 free
    }
    const result = computeSummary(participants, freeByDate, today);
    expect(result.mostFree).toHaveLength(5);
  });

  it("does not put everyone-free dates in mostFree", () => {
    const participants = [makeParticipant("a"), makeParticipant("b"), makeParticipant("c")];
    const d1 = makeDate(1);
    const d2 = makeDate(2);
    const freeByDate = new Map([
      [d1, new Set(["a", "b", "c"])], // everyone
      [d2, new Set(["a", "b"])],      // most
    ]);
    const result = computeSummary(participants, freeByDate, today);
    expect(result.everyoneFree).toHaveLength(1);
    expect(result.everyoneFree[0].date).toBe(d1);
    expect(result.mostFree).toHaveLength(1);
    expect(result.mostFree[0].date).toBe(d2);
  });

  it("sorts everyoneFree chronologically", () => {
    const participants = [makeParticipant("a"), makeParticipant("b")];
    const d1 = makeDate(3);
    const d2 = makeDate(1);
    const freeByDate = new Map([
      [d1, new Set(["a", "b"])],
      [d2, new Set(["a", "b"])],
    ]);
    const result = computeSummary(participants, freeByDate, today);
    expect(result.everyoneFree.map((d) => d.date)).toEqual([d2, d1]);
  });
});
