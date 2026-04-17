import { describe, it, expect } from "vitest";
import {
  layoutSlots,
  slotPosition,
  classifyDay,
  sortParticipants,
  COLOUR_PALETTE,
} from "./overlap";
import type { Participant } from "./types";

function makeParticipant(id: string, colour: string, name = "P"): Participant {
  return { id, calendarId: "cal-1", name, colour };
}

describe("layoutSlots", () => {
  it("returns 0x0 for n=0", () => {
    expect(layoutSlots(0)).toEqual({ cols: 0, rows: 0 });
  });

  it("returns 1-col stripes for n=1..4", () => {
    expect(layoutSlots(1)).toEqual({ cols: 1, rows: 1 });
    expect(layoutSlots(2)).toEqual({ cols: 1, rows: 2 });
    expect(layoutSlots(3)).toEqual({ cols: 1, rows: 3 });
    expect(layoutSlots(4)).toEqual({ cols: 1, rows: 4 });
  });

  it("returns near-square for n>=5", () => {
    expect(layoutSlots(5)).toEqual({ cols: 3, rows: 2 });
    expect(layoutSlots(8)).toEqual({ cols: 3, rows: 3 });
    expect(layoutSlots(9)).toEqual({ cols: 3, rows: 3 });
    expect(layoutSlots(12)).toEqual({ cols: 4, rows: 3 });
    expect(layoutSlots(16)).toEqual({ cols: 4, rows: 4 });
    expect(layoutSlots(17)).toEqual({ cols: 5, rows: 4 });
    expect(layoutSlots(25)).toEqual({ cols: 5, rows: 5 });
    expect(layoutSlots(32)).toEqual({ cols: 6, rows: 6 });
  });
});

describe("slotPosition", () => {
  it("maps row-major: participant 0 at (0,0)", () => {
    expect(slotPosition(0, 3)).toEqual({ col: 0, row: 0 });
  });

  it("wraps to next row at cols boundary", () => {
    expect(slotPosition(3, 3)).toEqual({ col: 0, row: 1 });
  });

  it("participant at index cols is (0,1)", () => {
    expect(slotPosition(5, 5)).toEqual({ col: 0, row: 1 });
  });
});

describe("classifyDay", () => {
  const p1 = makeParticipant("a", COLOUR_PALETTE[0]);
  const p2 = makeParticipant("b", COLOUR_PALETTE[1]);
  const p3 = makeParticipant("c", COLOUR_PALETTE[2]);

  it("returns everyoneFree=false when participants < 2", () => {
    expect(classifyDay([p1], new Set(["a"]))).toEqual({ everyoneFree: false });
  });

  it("returns everyoneFree=false when not all are free", () => {
    expect(classifyDay([p1, p2, p3], new Set(["a", "b"]))).toEqual({
      everyoneFree: false,
    });
  });

  it("returns everyoneFree=true when all are free and N >= 2", () => {
    expect(classifyDay([p1, p2], new Set(["a", "b"]))).toEqual({
      everyoneFree: true,
    });
  });

  it("silent participant (no record) counts as not free", () => {
    // p3 never submitted availability — not in freeSet
    expect(classifyDay([p1, p2, p3], new Set(["a", "b"]))).toEqual({
      everyoneFree: false,
    });
  });

  it("returns everyoneFree=false with zero participants", () => {
    expect(classifyDay([], new Set())).toEqual({ everyoneFree: false });
  });
});

describe("sortParticipants", () => {
  it("sorts by palette index", () => {
    const p0 = makeParticipant("a", COLOUR_PALETTE[2], "Third");
    const p1 = makeParticipant("b", COLOUR_PALETTE[0], "First");
    const p2 = makeParticipant("c", COLOUR_PALETTE[1], "Second");
    const sorted = sortParticipants([p0, p1, p2]);
    expect(sorted.map((p) => p.name)).toEqual(["First", "Second", "Third"]);
  });

  it("unknown colours sort to the end", () => {
    const known = makeParticipant("a", COLOUR_PALETTE[0], "Known");
    const unknown = makeParticipant("b", "#FFFFFF", "Unknown");
    const sorted = sortParticipants([unknown, known]);
    expect(sorted.map((p) => p.name)).toEqual(["Known", "Unknown"]);
  });
});
